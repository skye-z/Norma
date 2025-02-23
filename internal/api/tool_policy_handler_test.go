package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"norma/internal/config"
)

func TestToolPolicyCheck(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := &config.Config{}
	cfg.Security.ToolWhitelist = []string{"export_diagnostics"}
	cfg.Security.HighRiskConfirm = true
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	h := NewToolPolicyHandler(cfgPath)
	mux := http.NewServeMux()
	h.Register(mux)

	body, _ := json.Marshal(map[string]any{"tool_name": "export_diagnostics", "high_risk": true})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tools/policy/check", bytes.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	blockedBody, _ := json.Marshal(map[string]any{"tool_name": "delete_all"})
	blockedRec := httptest.NewRecorder()
	blockedReq := httptest.NewRequest(http.MethodPost, "/tools/policy/check", bytes.NewReader(blockedBody))
	mux.ServeHTTP(blockedRec, blockedReq)
	if blockedRec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", blockedRec.Code, blockedRec.Body.String())
	}
}
