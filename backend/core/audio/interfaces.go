// Package audio 音频处理核心接口和管道实现
package audio

import (
	"context"
	"errors"
)

const (
	WakeModeVADOnly     = "vad"           // 仅使用VAD模式，无需唤醒词即可检测语音
	WakeModeWakeWordVAD = "wake_word_vad" // 唤醒词+VAD模式，需要唤醒词触发
)

// 检测音频帧中的语音活动
type VAD interface {
	Detect(frame []float32) (bool, error)
}

// 从PCM流中检测唤醒词/短语
type WakeWord interface {
	Detect(frame []int16) (bool, error)
}

// 在VAD/ASR之前进行预处理，可应用AEC(回声消除)/NS(噪声抑制)
type Preprocessor interface {
	Process(frame []float32) []float32
	Name() string
}

// 流式输出转录结果
type ASR interface {
	Stream(ctx context.Context, onPartial func(text string), onFinal func(text string)) error
}

// 可以直接转录语音片段
type FrameASR interface {
	ASR
	TranscribeFrames(ctx context.Context, frames [][]float32, onPartial func(text string), onFinal func(text string)) error
}

// 流式合成音频块，支持中断
type TTS interface {
	Stream(ctx context.Context, text string, onChunk func(chunk []byte)) error
	Stop(sessionID string) error
}

// 提供音频格式信息
type AudioFormatProvider interface {
	SampleRate() int
	Channels() int
	Encoding() string
}

// 指示是否支持韵律提示
type ProsodyHintProvider interface {
	SupportsProsodyHints() bool
}

// 控制后台音频捕获
type BackgroundCaptureController interface {
	EnableBackgroundCapture(sessionID string, enabled bool)
}

// 协调语音相关组件的执行
type Pipeline struct {
	VAD              VAD            // 语音活动检测器
	WakeWord         WakeWord       // 唤醒词检测器
	ASR              ASR            // 语音识别引擎
	TTS              TTS            // 语音合成引擎
	Preprocessors    []Preprocessor // 预处理链(AEC/NS等)
	WakeMode         string         // 唤醒模式: vad 或 wake_word_vad
	WakeBufferFrames int            // 唤醒缓冲帧数
	SilenceFrames    int            // 静默帧数阈值
	AllowBargeIn     bool           // 是否允许打断
	OnWake           func()         // 检测到唤醒词时回调，可为空；回调不能阻塞音频循环
}

// 不可用的ASR实现，返回错误信息
type UnavailableASR struct {
	Reason string // 不可用原因
}

func (u UnavailableASR) Stream(ctx context.Context, onPartial func(string), onFinal func(string)) error {
	_ = ctx
	_ = onPartial
	_ = onFinal
	if u.Reason == "" {
		return errors.New("asr unavailable")
	}
	return errors.New("asr unavailable: " + u.Reason)
}

// 不可用的唤醒词检测器实现
type UnavailableWakeWord struct {
	Reason string // 不可用原因
}

func (u *UnavailableWakeWord) Detect(frame []int16) (bool, error) {
	_ = frame
	if u == nil || u.Reason == "" {
		return false, errors.New("wake word unavailable")
	}
	return false, errors.New("wake word unavailable: " + u.Reason)
}

// 不可用的TTS实现
type UnavailableTTS struct {
	Reason string // 不可用原因
}

func (u *UnavailableTTS) Stream(ctx context.Context, text string, onChunk func(chunk []byte)) error {
	_ = ctx
	_ = text
	_ = onChunk
	if u == nil || u.Reason == "" {
		return errors.New("tts unavailable")
	}
	return errors.New("tts unavailable: " + u.Reason)
}

func (u *UnavailableTTS) Stop(sessionID string) error {
	_ = sessionID
	return nil
}
