package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"norma/backend/core/db"
	"norma/backend/core/memory"
	"norma/backend/core/model"
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

	body, _ := json.Marshal(map[string]any{
		"kind": "fact", "content": "诺玛喜欢本地部署", "importance": 7,
		"memory_type": "preference", "source": "user", "evidence": "用户明确表示",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/memory", bytes.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("post code=%d body=%s", rec.Code, rec.Body.String())
	}

	var created model.Memory
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	id := strconv.FormatInt(created.ID, 10)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/memory?q=本地&limit=5", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get code=%d body=%s", rec.Code, rec.Body.String())
	}

	updateBody, _ := json.Marshal(map[string]any{"content": "诺玛优先本地部署", "importance": 9, "disabled": true})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/memory/"+id, bytes.NewReader(updateBody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put code=%d body=%s", rec.Code, rec.Body.String())
	}

	summaryBody, _ := json.Marshal(map[string]any{"session_id": "s1", "turns": []map[string]any{{"role": "user", "content": "今天继续完善记忆系统", "importance": 8}}})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/memory/summary", bytes.NewReader(summaryBody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("summary code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/memory/governance/run", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("governance code=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/memory/"+id, nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete code=%d body=%s", rec.Code, rec.Body.String())
	}
}
