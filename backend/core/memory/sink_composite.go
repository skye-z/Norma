package memory

// CompositeSink fan-outs writes to multiple sinks.
type CompositeSink struct {
	sinks []Sink
}

func NewCompositeSink(sinks ...Sink) *CompositeSink {
	out := make([]Sink, 0, len(sinks))
	for _, s := range sinks {
		if s != nil {
			out = append(out, s)
		}
	}
	return &CompositeSink{sinks: out}
}

func (c *CompositeSink) Upsert(path string, chunks []string) error {
	for _, s := range c.sinks {
		if err := s.Upsert(path, chunks); err != nil {
			return err
		}
	}
	return nil
}

func (c *CompositeSink) Delete(path string) error {
	for _, s := range c.sinks {
		if err := s.Delete(path); err != nil {
			return err
		}
	}
	return nil
}
