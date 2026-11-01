// 记忆检索评测：比较整份 Markdown 载入、全文检索、向量检索和 RRF 融合检索
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"norma/backend/core/db"
	"norma/backend/core/llm"
	"norma/backend/core/memory"
)

type dataset struct {
	Memories []struct{ ID, Text string } `json:"memories"`
	Queries  []struct {
		ID, Kind, Text string
		Gold           []string
	} `json:"queries"`
}

type method struct {
	name   string
	search func(ctx context.Context, q string, k int) ([]memory.Chunk, error)
}

type result struct {
	Method       string             `json:"method"`
	Kind         string             `json:"kind"`
	N            int                `json:"n"`
	RecallAtK    float64            `json:"recall_at_k"`
	HitAtK       float64            `json:"hit_at_k"`
	MRR          float64            `json:"mrr"`
	MeanChars    float64            `json:"mean_context_chars"`
	MeanLatency  float64            `json:"mean_latency_ms"`
	PerQueryMiss map[string]float64 `json:"-"`
}

func main() {
	dataPath := flag.String("data", "tools/memeval/dataset.json", "dataset")
	k := flag.Int("k", 3, "top-k")
	embedEndpoint := flag.String("embed-endpoint", "", "ollama endpoint for real embeddings")
	embedModel := flag.String("embed-model", "", "ollama embedding model")
	staticModel := flag.String("static-model", "", "Model2Vec directory for local static embeddings")
	llmEndpoint := flag.String("llm-endpoint", "", "cloud chat endpoint for query expansion")
	llmModel := flag.String("llm-model", "deepseek-flash", "chat model for query expansion")
	out := flag.String("out", "", "json output")
	flag.Parse()

	raw, err := os.ReadFile(*dataPath)
	if err != nil {
		log.Fatal(err)
	}
	var ds dataset
	if err := json.Unmarshal(raw, &ds); err != nil {
		log.Fatal(err)
	}
	idByText := map[string]string{}
	var chunks []string
	var totalChars int
	for _, m := range ds.Memories {
		idByText[m.Text] = m.ID
		chunks = append(chunks, m.Text)
		totalChars += len([]rune(m.Text))
	}

	tmp, _ := os.MkdirTemp("", "memeval-")
	defer os.RemoveAll(tmp)
	engine, err := db.NewEngine(filepath.Join(tmp, "m.db"), false)
	if err != nil {
		log.Fatal(err)
	}
	fts := memory.NewSQLiteStore(engine)
	if err := fts.Upsert("profile", chunks); err != nil {
		log.Fatal(err)
	}

	mkVec := func(name string, e memory.Embedder) *memory.DiskVectorStore {
		v, err := memory.NewDiskVectorStoreWithEmbedder(filepath.Join(tmp, name+".json"), e)
		if err != nil {
			log.Fatal(err)
		}
		if err := v.Upsert("profile", chunks); err != nil {
			log.Fatal(err)
		}
		return v
	}

	var methods []method
	methods = append(methods, method{"markdown-all", func(ctx context.Context, q string, k int) ([]memory.Chunk, error) {
		// 整份文件载入：不排序，全部条目都进入上下文
		all := make([]memory.Chunk, 0, len(chunks))
		for _, c := range chunks {
			all = append(all, memory.Chunk{Content: c})
		}
		return all, nil
	}})
	methods = append(methods, method{"fts", func(ctx context.Context, q string, k int) ([]memory.Chunk, error) { return fts.SearchText(ctx, q, k) }})
	hv := mkVec("hash", memory.HashEmbedder{})
	methods = append(methods, method{"vector-hash", func(ctx context.Context, q string, k int) ([]memory.Chunk, error) { return hv.Search(ctx, q, k) }})
	methods = append(methods, method{"hybrid-hash", memory.NewHybridRetriever(hv, fts, nil).Search})
	if *embedEndpoint != "" && *embedModel != "" {
		e := memory.NewOllamaEmbedder(*embedEndpoint, *embedModel, 30*time.Second)
		rv := mkVec("real", e)
		methods = append(methods, method{"vector-" + *embedModel, func(ctx context.Context, q string, k int) ([]memory.Chunk, error) { return rv.Search(ctx, q, k) }})
		methods = append(methods, method{"hybrid-" + *embedModel, memory.NewHybridRetriever(rv, fts, nil).Search})
	}

	if *staticModel != "" {
		e, err := memory.NewStaticEmbedder(*staticModel)
		if err != nil {
			log.Fatal(err)
		}
		defer e.Close()
		sv := mkVec("static", e)
		methods = append(methods, method{"vector-static", func(ctx context.Context, q string, k int) ([]memory.Chunk, error) { return sv.Search(ctx, q, k) }})
		methods = append(methods, method{"hybrid-static", memory.NewHybridRetriever(sv, fts, nil).Search})
		// 记忆条目彼此独立，相邻合并会把无关条目拼在一起
		nm := memory.NewHybridRetriever(sv, fts, nil)
		nm.SetPostProcessOptions(memory.RetrieverPostProcessOptions{EnableDedup: true, EnableRerank: true, RerankWeight: 0.15})
		methods = append(methods, method{"hybrid-static-nomerge", nm.Search})
	}
	if *llmEndpoint != "" {
		exp := llm.NewChatQueryExpander(*llmEndpoint, *llmModel, os.Getenv("NORMA_LLM_API_KEY"), 30*time.Second)
		methods = append(methods, method{"fts+llm-expand", memory.NewExpandingRetriever(fts, exp).Search})
	}

	var results []result
	for _, m := range methods {
		for _, kind := range []string{"lexical", "paraphrase", "all"} {
			r := result{Method: m.name, Kind: kind}
			var recall, hit, mrr, chars, lat float64
			for _, q := range ds.Queries {
				if kind != "all" && q.Kind != kind {
					continue
				}
				t0 := time.Now()
				hits, err := m.search(context.Background(), q.Text, *k)
				lat += float64(time.Since(t0).Microseconds()) / 1000
				if err != nil {
					log.Fatalf("%s: %v", m.name, err)
				}
				gold := map[string]bool{}
				for _, g := range q.Gold {
					gold[g] = true
				}
				top := hits
				if m.name != "markdown-all" && len(top) > *k {
					top = top[:*k]
				}
				found, firstRank := 0, 0
				for i, h := range top {
					chars += float64(len([]rune(h.Content)))
					if id := idByText[strings.TrimSpace(h.Content)]; gold[id] {
						found++
						if firstRank == 0 {
							firstRank = i + 1
						}
						delete(gold, id)
					}
				}
				recall += float64(found) / float64(len(q.Gold))
				if found > 0 {
					hit++
				}
				if firstRank > 0 {
					mrr += 1 / float64(firstRank)
				}
				r.N++
			}
			n := float64(r.N)
			r.RecallAtK, r.HitAtK, r.MRR, r.MeanChars, r.MeanLatency = recall/n, hit/n, mrr/n, chars/n, lat/n
			results = append(results, r)
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return false })
	fmt.Printf("memories=%d queries=%d k=%d total_chars=%d\n", len(ds.Memories), len(ds.Queries), *k, totalChars)
	fmt.Printf("%-22s %-11s %3s %9s %7s %6s %9s %8s\n", "method", "kind", "n", "recall@k", "hit@k", "MRR", "ctx_chars", "lat_ms")
	for _, r := range results {
		fmt.Printf("%-22s %-11s %3d %9.3f %7.3f %6.3f %9.1f %8.3f\n", r.Method, r.Kind, r.N, r.RecallAtK, r.HitAtK, r.MRR, r.MeanChars, r.MeanLatency)
	}
	if *out != "" {
		b, _ := json.MarshalIndent(map[string]any{"k": *k, "memories": len(ds.Memories), "queries": len(ds.Queries), "results": results}, "", "  ")
		_ = os.WriteFile(*out, b, 0o644)
	}
}
