// Package audio 音频处理
package audio

// PassthroughAECNS 直通预处理
type PassthroughAECNS struct {
	label string
}

// NewPassthroughAECNS 创建直通预处理
func NewPassthroughAECNS(label string) *PassthroughAECNS {
	if label == "" {
		label = "passthrough_aec_ns"
	}
	return &PassthroughAECNS{label: label}
}

// Process 处理音频帧（直通）
func (p *PassthroughAECNS) Process(frame []float32) []float32 {
	out := make([]float32, len(frame))
	copy(out, frame)
	return out
}

// Name 获取名称
func (p *PassthroughAECNS) Name() string { return p.label }
