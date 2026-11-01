// Package memory 记忆服务
package memory

import (
	"context"
	"strconv"
	"unicode"
	"fmt"
	"regexp"
	"strings"

	"xorm.io/xorm"

	"norma/backend/core/model"
)

// 索引块存储接口
type Sink interface {
	Upsert(path string, chunks []string) error
	Delete(path string) error
}

// 存储，支持 FTS5 全文搜索
type SQLiteStore struct {
	engine *xorm.Engine
}

var (
	sqliteFTSPossessivePattern = regexp.MustCompile("(?i)([\\p{L}\\p{N}_]+)['\u2019]s\\b")
	sqliteFTSTokenPattern      = regexp.MustCompile(`[\p{L}\p{N}_]+`)
)

// 创建 SQLite 存储
func NewSQLiteStore(engine *xorm.Engine) *SQLiteStore {
	return &SQLiteStore{engine: engine}
}

// 插入或更新
func (s *SQLiteStore) Upsert(path string, chunks []string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Where("path = ?", path).Delete(&model.IndexChunk{}); err != nil {
		_ = sess.Rollback()
		return err
	}
	if _, err := sess.Exec("DELETE FROM index_chunks_fts WHERE path = ?", path); err != nil {
		_ = sess.Rollback()
		return err
	}

	for i, c := range chunks {
		row := &model.IndexChunk{Path: path, ChunkNo: i, Content: c}
		if _, err := sess.Insert(row); err != nil {
			_ = sess.Rollback()
			return err
		}
		if _, err := sess.Exec(
			"INSERT INTO index_chunks_fts(rowid, path, content) VALUES(?,?,?)",
			row.ID, row.Path, segmentForFTS(row.Content),
		); err != nil {
			_ = sess.Rollback()
			return err
		}
	}

	return sess.Commit()
}

func (s *SQLiteStore) Delete(path string) error {
	sess := s.engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if _, err := sess.Where("path = ?", path).Delete(&model.IndexChunk{}); err != nil {
		_ = sess.Rollback()
		return err
	}
	if _, err := sess.Exec("DELETE FROM index_chunks_fts WHERE path = ?", path); err != nil {
		_ = sess.Rollback()
		return err
	}
	return sess.Commit()
}

func (s *SQLiteStore) Search(ctx context.Context, query string, topK int) ([]Chunk, error) {
	return s.SearchText(ctx, query, topK)
}

func (s *SQLiteStore) SearchText(ctx context.Context, query string, topK int) ([]Chunk, error) {
	_ = ctx
	if topK <= 0 {
		topK = 5
	}
	query = normalizeFTSQuery(query)
	if query == "" {
		return nil, nil
	}

	rows, err := s.engine.Query(
		`SELECT c.path, c.chunk_no, c.content, bm25(index_chunks_fts) AS rank
		 FROM index_chunks_fts
		 JOIN index_chunk c ON c.id = index_chunks_fts.rowid
		 WHERE index_chunks_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		query, topK,
	)
	if err != nil {
		return nil, err
	}

	out := make([]Chunk, 0, len(rows))
	for _, r := range rows {
		out = append(out, Chunk{
			DocID:   string(r["path"]),
			ChunkID: fmt.Sprintf("%s#%s", string(r["path"]), string(r["chunk_no"])),
			Score:   -parseRank(string(r["rank"])),
			Content: string(r["content"]),
		})
	}
	return out, nil
}

// 中文没有空格分词，unicode61 会把一整段汉字当作一个词；
// 这里在入库和查询时把汉字展开成单字加相邻双字，其余文字按词切分
func segmentForFTS(text string) string {
	text = strings.ToLower(text)
	var out []string
	for _, tok := range sqliteFTSTokenPattern.FindAllString(text, -1) {
		runes := []rune(tok)
		if !isHan(runes[0]) {
			out = append(out, tok)
			continue
		}
		for i, r := range runes {
			out = append(out, string(r))
			if i+1 < len(runes) && isHan(runes[i+1]) {
				out = append(out, string(runes[i:i+2]))
			}
		}
	}
	return strings.Join(out, " ")
}

func isHan(r rune) bool { return unicode.Is(unicode.Han, r) }

func normalizeFTSQuery(query string) string {
	query = sqliteFTSPossessivePattern.ReplaceAllString(strings.ToLower(query), `${1}`)
	seg := strings.Fields(segmentForFTS(query))
	if len(seg) == 0 {
		return ""
	}
	seen := map[string]bool{}
	terms := make([]string, 0, len(seg))
	for _, t := range seg {
		if seen[t] {
			continue
		}
		seen[t] = true
		terms = append(terms, `"`+t+`"`)
	}
	return strings.Join(terms, " OR ")
}

func parseRank(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
