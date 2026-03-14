// Package security 安全策略
package security

import (
	"strings"

	"norma/backend/utils"
)

// 工具决策
type ToolDecision struct {
	ToolName             string `json:"tool_name"`
	Allowed              bool   `json:"allowed"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	Reason               string `json:"reason,omitempty"`
}

// 工具策略
type ToolPolicy struct {
	cfg config.SecurityConfig
}

// 创建工具策略
func NewToolPolicy(cfg config.SecurityConfig) ToolPolicy {
	return ToolPolicy{cfg: cfg}
}

// 检查工具是否允许执行
func (p ToolPolicy) Check(toolName string, highRisk bool) ToolDecision {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return ToolDecision{ToolName: name, Allowed: false, Reason: "tool name required"}
	}
	decision := ToolDecision{ToolName: name, Allowed: true, RequiresConfirmation: highRisk}
	if highRisk {
		decision.Reason = "high risk tool requires explicit confirmation"
	}
	return decision
}
