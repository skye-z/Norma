package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"norma/internal/db"
	"norma/internal/memory"
)

func TestMemoryHandlerCRUD(t *testing.T) {
	engine, err := db.NewEngine(filepath.Join(t.TempDir(), "api.db"), false)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer engine.Close()

	h := NewMemoryHandler(memory.NewService(engine))
	mux := http.NewServeMux()
	h.Register(mux)

	body, _ := json.Marshal(map[string]any{"kind": "fact", "content": "诺玛喜欢本地部署", "importance": 7})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/memory", bytes.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("post code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/memory?q=本地&limit=5", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get code=%d body=%s", rec.Code, rec.Body.String())
	}
}
