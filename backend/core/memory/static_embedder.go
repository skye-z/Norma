// Package memory 记忆服务
package memory

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/text/unicode/norm"
)

// 静态词向量嵌入器（Model2Vec 格式）
// 流程：规范化、Unigram 分词、查词向量表、取平均并归一化。没有神经网络推理，也不依赖外部服务进程
type StaticEmbedder struct {
	tok  *unigramTokenizer
	file *os.File
	base int64 // 向量数据在文件中的起始偏移
	dim  int
	rows int
	name string

	mu    sync.Mutex
	cache map[int][]float32
}

// 加载模型目录，目录内需有 model.safetensors 与 tokenizer.json
func NewStaticEmbedder(dir string) (*StaticEmbedder, error) {
	tok, err := loadUnigramTokenizer(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(dir, "model.safetensors"))
	if err != nil {
		return nil, err
	}
	var hlen uint64
	if err := binary.Read(f, binary.LittleEndian, &hlen); err != nil || hlen == 0 || hlen > 1<<24 {
		f.Close()
		return nil, fmt.Errorf("safetensors header invalid")
	}
	hdr := make([]byte, hlen)
	if _, err := f.ReadAt(hdr, 8); err != nil {
		f.Close()
		return nil, err
	}
	var meta map[string]struct {
		DType   string  `json:"dtype"`
		Shape   []int   `json:"shape"`
		Offsets []int64 `json:"data_offsets"`
	}
	if err := json.Unmarshal(hdr, &meta); err != nil {
		f.Close()
		return nil, err
	}
	t, ok := meta["embeddings"]
	if !ok || t.DType != "F32" || len(t.Shape) != 2 || len(t.Offsets) != 2 {
		f.Close()
		return nil, fmt.Errorf("unsupported safetensors layout")
	}
	return &StaticEmbedder{
		tok: tok, file: f, base: 8 + int64(hlen) + t.Offsets[0],
		rows: t.Shape[0], dim: t.Shape[1],
		name:  "static:" + filepath.Base(dir),
		cache: make(map[int][]float32),
	}, nil
}

// 嵌入器标识，写入向量库用于发现模型变更
func (e *StaticEmbedder) Name() string { return e.name }

// 向量维度
func (e *StaticEmbedder) Dim() int { return e.dim }

// 关闭模型文件
func (e *StaticEmbedder) Close() error { return e.file.Close() }

func (e *StaticEmbedder) row(id int) ([]float32, error) {
	e.mu.Lock()
	if v, ok := e.cache[id]; ok {
		e.mu.Unlock()
		return v, nil
	}
	e.mu.Unlock()
	buf := make([]byte, e.dim*4)
	if _, err := e.file.ReadAt(buf, e.base+int64(id)*int64(e.dim)*4); err != nil {
		return nil, err
	}
	v := make([]float32, e.dim)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	e.mu.Lock()
	if len(e.cache) < 20000 {
		e.cache[id] = v
	}
	e.mu.Unlock()
	return v, nil
}

// 嵌入文本
func (e *StaticEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	out := make([]float32, e.dim)
	n := 0
	for _, id := range e.tok.Encode(text) {
		if id < 0 || id >= e.rows {
			continue
		}
		v, err := e.row(id)
		if err != nil {
			return nil, err
		}
		for i, x := range v {
			out[i] += x
		}
		n++
	}
	if n == 0 {
		return out, nil
	}
	var sum float64
	for i := range out {
		out[i] /= float32(n)
		sum += float64(out[i]) * float64(out[i])
	}
	if sum > 0 {
		inv := float32(1 / math.Sqrt(sum))
		for i := range out {
			out[i] *= inv
		}
	}
	return out, nil
}

// Unigram 分词器，兼容 XLM-R 系列的 tokenizer.json
type unigramTokenizer struct {
	pieces  map[string]int
	scores  []float64
	maxLen  int
	unkID   int
	special map[int]bool
}

func loadUnigramTokenizer(path string) (*unigramTokenizer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg struct {
		Model struct {
			Type  string          `json:"type"`
			UnkID int             `json:"unk_id"`
			Vocab [][]interface{} `json:"vocab"`
		} `json:"model"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if cfg.Model.Type != "Unigram" {
		return nil, fmt.Errorf("unsupported tokenizer type %q", cfg.Model.Type)
	}
	t := &unigramTokenizer{
		pieces:  make(map[string]int, len(cfg.Model.Vocab)),
		scores:  make([]float64, len(cfg.Model.Vocab)),
		unkID:   cfg.Model.UnkID,
		special: map[int]bool{0: true, cfg.Model.UnkID: true},
	}
	for i, p := range cfg.Model.Vocab {
		s, _ := p[0].(string)
		sc, _ := p[1].(float64)
		t.pieces[s] = i
		t.scores[i] = sc
		if l := len([]rune(s)); l > t.maxLen {
			t.maxLen = l
		}
	}
	return t, nil
}

// 与 tokenizer.json 中的 normalizer 一致：NFKC、给标点两侧加空格、合并空白、首尾去空白
const asciiPunct = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"

func normalizeForUnigram(s string) string {
	s = norm.NFKC.String(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 128 && strings.ContainsRune(asciiPunct, r):
			b.WriteString(" ")
			b.WriteRune(r)
			b.WriteString(" ")
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// 对文本分词，返回词表编号，未登录字符以 unk 表示
func (t *unigramTokenizer) Encode(text string) []int {
	s := normalizeForUnigram(text)
	if s == "" {
		return nil
	}
	rs := []rune("▁" + strings.ReplaceAll(s, " ", "▁"))
	n := len(rs)
	best := make([]float64, n+1)
	prev := make([]int, n+1)
	pid := make([]int, n+1)
	for i := 1; i <= n; i++ {
		best[i] = math.Inf(-1)
	}
	for i := 0; i < n; i++ {
		if math.IsInf(best[i], -1) {
			continue
		}
		matched := false
		for l := 1; l <= t.maxLen && i+l <= n; l++ {
			id, ok := t.pieces[string(rs[i:i+l])]
			if !ok {
				continue
			}
			matched = true
			if sc := best[i] + t.scores[id]; sc > best[i+l] {
				best[i+l], prev[i+l], pid[i+l] = sc, i, id
			}
		}
		if !matched || math.IsInf(best[i+1], -1) {
			// 单字符兜底为未知符，保证路径连通
			if sc := best[i] - 20; sc > best[i+1] {
				best[i+1], prev[i+1], pid[i+1] = sc, i, t.unkID
			}
		}
	}
	var ids []int
	for i := n; i > 0; i = prev[i] {
		if !t.special[pid[i]] {
			ids = append(ids, pid[i])
		}
	}
	for l, r := 0, len(ids)-1; l < r; l, r = l+1, r-1 {
		ids[l], ids[r] = ids[r], ids[l]
	}
	return ids
}
