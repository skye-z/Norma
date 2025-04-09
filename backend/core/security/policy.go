package security

import (
	"strings"

	"norma/backend/utils"
)

type ToolDecision struct {
	ToolName             string `json:"tool_name"`
	Allowed              bool   `json:"allowed"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	Reason               string `json:"reason,omitempty"`
}

type ToolPolicy struct {
	cfg config.SecurityConfig
}

func NewToolPolicy(cfg config.SecurityConfig) ToolPolicy {
	return ToolPolicy{cfg: cfg}
}

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
