// Package memory 记忆服务
package memory

// CompositeSink 组合Sink
type CompositeSink struct {
	sinks []Sink
}

// NewCompositeSink 创建组合Sink
func NewCompositeSink(sinks ...Sink) *CompositeSink {
	out := make([]Sink, 0, len(sinks))
	for _, s := range sinks {
		if s != nil {
			out = append(out, s)
		}
	}
	return &CompositeSink{sinks: out}
}

// Upsert 插入或更新
func (c *CompositeSink) Upsert(path string, chunks []string) error {
	for _, s := range c.sinks {
		if err := s.Upsert(path, chunks); err != nil {
			return err
		}
	}
	return nil
}

// Delete 删除
func (c *CompositeSink) Delete(path string) error {
	for _, s := range c.sinks {
		if err := s.Delete(path); err != nil {
			return err
		}
	}
	return nil
}
