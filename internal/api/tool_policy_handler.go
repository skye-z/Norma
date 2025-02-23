package api

import (
	"encoding/json"
	"net/http"

	"norma/internal/config"
	"norma/internal/errs"
	"norma/internal/security"
)

type ToolPolicyHandler struct {
	configPath string
}

type toolPolicyReq struct {
	ToolName string `json:"tool_name"`
	HighRisk bool   `json:"high_risk"`
}

func NewToolPolicyHandler(configPath string) *ToolPolicyHandler {
	return &ToolPolicyHandler{configPath: configPath}
}

func (h *ToolPolicyHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/tools/policy/check", h.handleCheck)
}

func (h *ToolPolicyHandler) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	var req toolPolicyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, errs.Invalid("invalid json"))
		return
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, errs.Internal("config_load_failed", err.Error()))
		return
	}
	decision := security.NewToolPolicy(cfg.Security).Check(req.ToolName, req.HighRisk)
	if !decision.Allowed {
		writeAPIError(w, errs.Forbidden(decision.Reason))
		return
	}
	writeJSON(w, http.StatusOK, decision)
}
