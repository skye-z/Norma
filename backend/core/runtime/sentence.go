package runtime

import "strings"

// 流式分句器：按句末标点切分，过长的片段在逗号处提前输出以缩短首句等待
type sentenceSplitter struct {
	buf      []rune
	minComma int
}

func newSentenceSplitter() *sentenceSplitter {
	return &sentenceSplitter{minComma: 24}
}

func isSentenceEnd(r rune) bool {
	switch r {
	case '。', '！', '？', '!', '?', '；', ';', '\n':
		return true
	}
	return false
}

func isClauseBreak(r rune) bool {
	return r == '，' || r == ',' || r == '、' || r == '：' || r == ':'
}

// 追加一段流式文本，返回此时已完整的句子
func (p *sentenceSplitter) Push(token string) []string {
	var out []string
	for _, r := range token {
		p.buf = append(p.buf, r)
		if isSentenceEnd(r) || (isClauseBreak(r) && len(p.buf) >= p.minComma) {
			if s := strings.TrimSpace(string(p.buf)); s != "" {
				out = append(out, s)
			}
			p.buf = p.buf[:0]
		}
	}
	return out
}

// 输出尚未结束的剩余文本
func (p *sentenceSplitter) Flush() string {
	s := strings.TrimSpace(string(p.buf))
	p.buf = p.buf[:0]
	return s
}
