// Package audio 音频处理
package audio

// 直通预处理
type PassthroughAECNS struct {
	label string
}

// 创建直通预处理
func NewPassthroughAECNS(label string) *PassthroughAECNS {
	if label == "" {
		label = "passthrough_aec_ns"
	}
	return &PassthroughAECNS{label: label}
}

// 处理音频帧（直通）
func (p *PassthroughAECNS) Process(frame []float32) []float32 {
	out := make([]float32, len(frame))
	copy(out, frame)
	return out
}

// 获取名称
func (p *PassthroughAECNS) Name() string { return p.label }
