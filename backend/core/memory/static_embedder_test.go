package memory

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func staticModelDir(t *testing.T) string {
	dir := os.Getenv("NORMA_STATIC_MODEL_DIR")
	if dir == "" {
		dir = "/Users/skye/Documents/Environment/Package/Model2Vec/potion-multilingual-128M"
	}
	if _, err := os.Stat(filepath.Join(dir, "tokenizer.json")); err != nil {
		t.Skip("tokenizer not installed")
	}
	return dir
}

func staticWeightsDir(t *testing.T) string {
	dir := staticModelDir(t)
	if _, err := os.Stat(filepath.Join(dir, "model.safetensors")); err != nil {
		t.Skip("static embedding model not installed")
	}
	return dir
}

func TestUnigramTokenizerSegmentsChinese(t *testing.T) {
	dir := staticModelDir(t)
	tok, err := loadUnigramTokenizer(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		t.Fatal(err)
	}
	ids := tok.Encode("你好，世界")
	if len(ids) == 0 || len(ids) > 6 {
		t.Fatalf("unexpected token count %d", len(ids))
	}
}

func TestStaticEmbedderSemanticOrdering(t *testing.T) {
	e, err := NewStaticEmbedder(staticWeightsDir(t))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	ctx := context.Background()
	q, _ := e.Embed(ctx, "我平时喜欢喝什么饮料")
	near, _ := e.Embed(ctx, "用户喜欢喝咖啡和茶")
	far, _ := e.Embed(ctx, "明天下午三点要开项目评审会")
	cos := func(a, b []float32) float64 {
		var d float64
		for i := range a {
			d += float64(a[i]) * float64(b[i])
		}
		return d
	}
	if math.IsNaN(cos(q, near)) || cos(q, near) <= cos(q, far) {
		t.Fatalf("near=%.3f far=%.3f", cos(q, near), cos(q, far))
	}
}
