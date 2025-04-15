// Package audio 提供音频处理的核心接口和管道实现
package audio

import (
	"context"
	"errors"
	"log"
)

const minPostWakeHoldFrames = 75

// Run 消费音频帧，端点检测时触发ASR回调
func (p *Pipeline) Run(
	ctx context.Context,
	source FrameSource,
	onSpeechStart func(),
	onSpeechEnd func(),
	onPartial func(string),
	onFinal func(string),
) error {
	// 验证必需组件
	if p.VAD == nil || p.ASR == nil {
		return errors.New("pipeline requires VAD and ASR")
	}

	// 设置默认静默帧阈值
	silenceFrames := p.SilenceFrames
	if silenceFrames <= 0 {
		silenceFrames = 4
	}
	// 创建端点检测器
	detector := NewEndpointDetector(silenceFrames)
	// 当前 utterances 片段
	utterance := make([][]float32, 0, 16)
	// 唤醒词缓冲帧数
	wakeBufferFrames := p.WakeBufferFrames
	if wakeBufferFrames <= 0 {
		wakeBufferFrames = 1
	}
	// 唤醒词缓冲区
	wakeBuffer := make([][]float32, 0, wakeBufferFrames)
	// 确定唤醒模式
	wakeMode := p.WakeMode
	if wakeMode == "" {
		wakeMode = WakeModeVADOnly
	}
	// 是否已就绪(可以开始语音检测)
	armed := wakeMode == WakeModeVADOnly || p.WakeWord == nil
	// 唤醒后保持语音的帧数
	postWakeHoldFrames := 0
	log.Printf("[pipeline] start wake_mode=%s armed=%v allow_barge_in=%v", wakeMode, armed, p.AllowBargeIn)

	// flushUtterance 处理并发送当前的 utterances 到ASR
	flushUtterance := func(reason string) error {
		if len(utterance) == 0 {
			return nil
		}
		log.Printf("[pipeline] flush utterance reason=%s utterance_frames=%d", reason, len(utterance))
		// 触发语音结束回调
		if onSpeechEnd != nil {
			onSpeechEnd()
		}
		// 优先使用FrameASR进行转录
		if fasr, ok := p.ASR.(FrameASR); ok {
			log.Printf("[pipeline] transcribe via FrameASR frames=%d", len(utterance))
			if err := fasr.TranscribeFrames(ctx, utterance, onPartial, onFinal); err != nil {
				log.Printf("[pipeline] frame asr error: %v", err)
				return err
			}
		} else {
			// 使用流式ASR
			log.Printf("[pipeline] transcribe via streaming ASR")
			if err := p.ASR.Stream(ctx, onPartial, onFinal); err != nil {
				log.Printf("[pipeline] asr stream error: %v", err)
				return err
			}
		}
		// 重置 utterances
		utterance = utterance[:0]
		// 如果是唤醒词模式，重置检测器状态
		if wakeMode == WakeModeWakeWordVAD && p.WakeWord != nil {
			armed = false
			postWakeHoldFrames = 0
			wakeBuffer = wakeBuffer[:0]
			detector = NewEndpointDetector(silenceFrames)
		}
		return nil
	}

	// 主循环: 持续读取音频帧
	for {
		frame, err := source.Next(ctx)
		if err != nil {
			// 流结束，处理剩余 utterances
			if errors.Is(err, ErrEndOfStream) {
				if flushErr := flushUtterance("end_of_stream"); flushErr != nil {
					return flushErr
				}
				return nil
			}
			// 上下文取消
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		// 预处理: 依次通过各个预处理器(AEC/NS等)
		processed := frame
		for _, pre := range p.Preprocessors {
			if pre == nil {
				continue
			}
			processed = pre.Process(processed)
		}

		// 唤醒词检测模式
		if !armed && p.WakeWord != nil {
			// 缓冲当前帧用于唤醒词检测
			buffered := cloneFrame(processed)
			wakeBuffer = append(wakeBuffer, buffered)
			// 保持缓冲在指定帧数
			if len(wakeBuffer) > wakeBufferFrames {
				wakeBuffer = append([][]float32(nil), wakeBuffer[len(wakeBuffer)-wakeBufferFrames:]...)
			}
			// 转换为PCM16格式进行唤醒词检测
			pcm := floatToPCM16(processed)
			matched, err := p.WakeWord.Detect(pcm)
			if err != nil {
				log.Printf("[pipeline] wake detect error: %v", err)
				return err
			}
			// 检测到唤醒词
			if matched {
				log.Printf("[pipeline] wake detected buffered_frames=%d", len(wakeBuffer))
				armed = true
				// 设置唤醒后保持帧数(至少保持75帧)
				postWakeHoldFrames = wakeBufferFrames
				if postWakeHoldFrames < minPostWakeHoldFrames {
					postWakeHoldFrames = minPostWakeHoldFrames
				}
				// 将唤醒缓冲区的帧复制到 utterances
				utterance = append(utterance[:0], cloneFrames(wakeBuffer)...)
				wakeBuffer = wakeBuffer[:0]
				// 触发语音开始回调
				if onSpeechStart != nil {
					log.Printf("[pipeline] speech start from wake utterance_frames=%d", len(utterance))
					onSpeechStart()
				}
				// 更新端点检测器为语音状态
				_, _ = detector.Update(true)
			}
			continue
		}

		// 唤醒后保持期间强制视为语音
		forceSpeech := false
		if postWakeHoldFrames > 0 {
			forceSpeech = true
			postWakeHoldFrames--
		}

		// VAD语音活动检测
		isSpeech, err := p.VAD.Detect(processed)
		if err != nil {
			log.Printf("[pipeline] vad detect error: %v", err)
			return err
		}

		// 如果检测到语音或处于保持期，添加到 utterances
		if isSpeech || forceSpeech {
			utterance = append(utterance, cloneFrame(processed))
		}

		// 更新端点检测器状态
		start, end := detector.Update(isSpeech || forceSpeech)
		// 允许打断: 检测到新语音时触发回调
		if p.AllowBargeIn && isSpeech && onSpeechStart != nil {
			log.Printf("[pipeline] barge-in speech detected")
			onSpeechStart()
		}
		// 语音开始信号
		if start && onSpeechStart != nil {
			log.Printf("[pipeline] speech start utterance_frames=%d", len(utterance))
			onSpeechStart()
		}

		// 语音结束信号，触发转录
		if end {
			log.Printf("[pipeline] speech end utterance_frames=%d", len(utterance))
			if err := flushUtterance("endpoint"); err != nil {
				return err
			}
		}
	}
}

// floatToPCM16 将float32音频帧转换为int16 PCM格式
func floatToPCM16(frame []float32) []int16 {
	out := make([]int16, len(frame))
	for i, sample := range frame {
		if sample > 1 {
			sample = 1
		}
		if sample < -1 {
			sample = -1
		}
		out[i] = int16(sample * 32767)
	}
	return out
}

// cloneFrame 深拷贝单帧音频数据
func cloneFrame(frame []float32) []float32 {
	copied := make([]float32, len(frame))
	copy(copied, frame)
	return copied
}

// cloneFrames 深拷贝多帧音频数据
func cloneFrames(frames [][]float32) [][]float32 {
	out := make([][]float32, 0, len(frames))
	for _, frame := range frames {
		out = append(out, cloneFrame(frame))
	}
	return out
}
