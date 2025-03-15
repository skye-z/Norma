package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"norma/internal/config"
	"norma/pkg/layout"
)

type PrivacyHandler struct {
	purgeProofDir string
	dbPath        string
	logDir        string
}

func NewPrivacyHandler(cfgPath string) *PrivacyHandler {
	cfg, _ := config.Load(cfgPath)
	h := &PrivacyHandler{purgeProofDir: layout.PurgeProofDir, dbPath: layout.DBPath, logDir: layout.LogsDir}
	if cfg != nil {
		h.purgeProofDir = cfg.Security.PurgeProofDir
		h.dbPath = cfg.Database.Path
		h.logDir = cfg.App.LogDir
	}
	return h
}

func (h *PrivacyHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/privacy/purge", h.handlePurge)
}

func (h *PrivacyHandler) handlePurge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	proof, err := h.purge()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, proof)
}

func (h *PrivacyHandler) purge() (map[string]any, error) {
	_ = os.MkdirAll(h.purgeProofDir, 0o755)
	removed := []string{}
	for _, path := range []string{h.dbPath, filepath.Join(h.logDir, "app.log"), filepath.Join(h.logDir, "audio.log"), filepath.Join(h.logDir, "rag.log")} {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err == nil || os.IsNotExist(err) {
			removed = append(removed, path)
		}
	}
	proof := map[string]any{"timestamp_utc": time.Now().UTC().Format(time.RFC3339), "removed": removed}
	b, _ := json.MarshalIndent(proof, "", "  ")
	proofPath := filepath.Join(h.purgeProofDir, "delete-proof-"+time.Now().UTC().Format("20060102-150405")+".json")
	if err := os.WriteFile(proofPath, b, 0o644); err != nil {
		return nil, err
	}
	proof["proof_path"] = proofPath
	return proof, nil
}
