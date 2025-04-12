// Package audio 提供音频处理的核心接口和管道实现
package audio

// EndpointDetector 端点检测器，用于检测语音的开始和结束
// 通过分析连续的音频帧，判断用户是否在说话以及何时停止说话
type EndpointDetector struct {
	silenceFrames int  // 判定语音结束的连续静默帧数阈值
	inSpeech      bool // 当前是否处于语音状态
	silentCount   int  // 当前连续静默帧计数
}

// NewEndpointDetector 创建新的端点检测器
// 参数 silenceFrames: 判定语音结束的连续静默帧数阈值(默认4帧)
func NewEndpointDetector(silenceFrames int) *EndpointDetector {
	if silenceFrames <= 0 {
		silenceFrames = 4
	}
	return &EndpointDetector{silenceFrames: silenceFrames}
}

// Update 更新端点检测器状态
// 参数 isSpeech: 当前帧是否为语音
// 返回值:
//   - speechStart: 是否检测到语音开始
//   - speechEnd: 是否检测到语音结束
func (d *EndpointDetector) Update(isSpeech bool) (speechStart bool, speechEnd bool) {
	// 检测到语音活动
	if isSpeech {
		if !d.inSpeech {
			// 之前不在语音状态，现在开始语音
			d.inSpeech = true
			d.silentCount = 0
			return true, false
		}
		// 继续在语音状态，重置静默计数
		d.silentCount = 0
		return false, false
	}

	// 当前帧是静默
	if !d.inSpeech {
		// 之前也不在语音状态，保持静默
		return false, false
	}

	// 之前在语音状态，现在出现静默
	d.silentCount++
	// 达到静默阈值，判定语音结束
	if d.silentCount >= d.silenceFrames {
		d.inSpeech = false
		d.silentCount = 0
		return false, true
	}
	return false, false
}
