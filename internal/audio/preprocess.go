package audio

type PassthroughAECNS struct {
	label string
}

func NewPassthroughAECNS(label string) *PassthroughAECNS {
	if label == "" {
		label = "passthrough_aec_ns"
	}
	return &PassthroughAECNS{label: label}
}

func (p *PassthroughAECNS) Process(frame []float32) []float32 {
	out := make([]float32, len(frame))
	copy(out, frame)
	return out
}

func (p *PassthroughAECNS) Name() string { return p.label }
