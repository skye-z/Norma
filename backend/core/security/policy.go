// Package security 安全策略
package security

import (
	"strings"

	"norma/backend/utils"
)

// ToolDecision 工具决策
type ToolDecision struct {
	ToolName             string `json:"tool_name"`
	Allowed              bool   `json:"allowed"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	Reason               string `json:"reason,omitempty"`
}

// ToolPolicy 工具策略
type ToolPolicy struct {
	cfg config.SecurityConfig
}

// NewToolPolicy 创建工具策略
func NewToolPolicy(cfg config.SecurityConfig) ToolPolicy {
	return ToolPolicy{cfg: cfg}
}

// Check 检查工具是否允许执行
func (p ToolPolicy) Check(toolName string, highRisk bool) ToolDecision {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return ToolDecision{ToolName: name, Allowed: false, Reason: "tool name required"}
	}
	allowed := false
	for _, item := range p.cfg.ToolWhitelist {
		if strings.EqualFold(strings.TrimSpace(item), name) {
			allowed = true
			break
		}
	}
	if !allowed {
		return ToolDecision{ToolName: name, Allowed: false, Reason: "tool is not in whitelist"}
	}
	decision := ToolDecision{ToolName: name, Allowed: true}
	if highRisk && p.cfg.HighRiskConfirm {
		decision.RequiresConfirmation = true
		decision.Reason = "high risk tool requires explicit confirmation"
	}
	return decision
}
