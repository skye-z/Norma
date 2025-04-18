// Package db 数据库操作
package db

import (
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
	"xorm.io/xorm"
	"xorm.io/xorm/log"

	"norma/backend/core/model"
)

// NewEngine 创建 xorm 引擎
func NewEngine(dbPath string, enableSQLLog bool) (*xorm.Engine, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", filepath.ToSlash(dbPath))
	engine, err := xorm.NewEngine("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("new engine: %w", err)
	}

	if enableSQLLog {
		engine.SetLogLevel(log.LOG_DEBUG)
		engine.ShowSQL(true)
	}

	if err := engine.Sync2(
		new(model.Conversation),
		new(model.Memory),
		new(model.IndexChunk),
	); err != nil {
		_ = engine.Close()
		return nil, fmt.Errorf("sync schema: %w", err)
	}

	if err := ensureFTS(engine); err != nil {
		_ = engine.Close()
		return nil, err
	}

	return engine, nil
}

// ensureFTS 确保 FTS 表存在
func ensureFTS(engine *xorm.Engine) error {
	_, err := engine.Exec(`
CREATE VIRTUAL TABLE IF NOT EXISTS index_chunks_fts
USING fts5(path, content, tokenize = 'unicode61');`)
	if err != nil {
		return fmt.Errorf("create fts table: %w", err)
	}
	return nil
}
