// Package api 提供 HTTP API
package api

import (
	"encoding/json"
	"net/http"

	"norma/backend/core/security"
	"norma/backend/utils"
)

// ToolPolicyHandler 工具策略处理器
type ToolPolicyHandler struct {
	configPath string
}

// toolPolicyReq 工具策略请求
type toolPolicyReq struct {
	ToolName string `json:"tool_name"`
	HighRisk bool   `json:"high_risk"`
}

// NewToolPolicyHandler 创建工具策略处理器
func NewToolPolicyHandler(configPath string) *ToolPolicyHandler {
	return &ToolPolicyHandler{configPath: configPath}
}

// Register 注册路由
func (h *ToolPolicyHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/tools/policy/check", h.handleCheck)
}

// handleCheck 检查工具策略
func (h *ToolPolicyHandler) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, config.Invalid("method not allowed"))
		return
	}
	var req toolPolicyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, config.Invalid("invalid json"))
		return
	}
	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, config.Internal("config_load_failed", err.Error()))
		return
	}
	decision := security.NewToolPolicy(cfg.Security).Check(req.ToolName, req.HighRisk)
	if !decision.Allowed {
		writeAPIError(w, config.Forbidden(decision.Reason))
		return
	}
	writeJSON(w, http.StatusOK, decision)
}
