package index

import (
	"context"
	"fmt"
	"strings"

	"xorm.io/xorm"

	"norma/internal/model"
)

// Sink persists index chunks.
type Sink interface {
	Upsert(path string, chunks []string) error
	Delete(path string) error
}

// SQLiteStore persists chunks and supports FTS5 text search.
type SQLiteStore struct {
	engine *xorm.Engine
}

func NewSQLiteStore(engine *xorm.Engine) *SQLiteStore {
	return &SQLiteStore{engine: engine}
}

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
			row.ID, row.Path, row.Content,
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
	_ = ctx
	if topK <= 0 {
		topK = 5
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
			Score:   1.0,
			Content: string(r["content"]),
		})
	}
	return out, nil
}
