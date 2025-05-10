package audio

// 端点检测器
type EndpointDetector struct {
	silenceFrames int // 判定结束的静默帧数
	inSpeech      bool
	silentCount   int
}

// 新建端点检测器
func NewEndpointDetector(silenceFrames int) *EndpointDetector {
	if silenceFrames <= 0 {
		silenceFrames = 4
	}
	return &EndpointDetector{silenceFrames: silenceFrames}
}

// 更新检测状态
func (d *EndpointDetector) Update(isSpeech bool) (speechStart bool, speechEnd bool) {
	if isSpeech {
		if !d.inSpeech {
			d.inSpeech = true
			d.silentCount = 0
			return true, false
		}
		d.silentCount = 0
		return false, false
	}

	// 静默帧
	if d.inSpeech {
		d.silentCount++
		if d.silentCount >= d.silenceFrames {
			d.inSpeech = false
			d.silentCount = 0
			return false, true
		}
	}
	return false, false
}
