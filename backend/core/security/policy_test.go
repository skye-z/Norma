package security

import (
	"testing"

	config "norma/backend/utils"
)

func TestToolPolicyCheck(t *testing.T) {
	policy := NewToolPolicy(config.SecurityConfig{
		ToolWhitelist:   []string{"open_local_file", "export_diagnostics"},
		HighRiskConfirm: true,
	})

	decision := policy.Check("open_local_file", false)
	if !decision.Allowed || decision.RequiresConfirmation {
		t.Fatalf("unexpected decision: %#v", decision)
	}

	highRisk := policy.Check("export_diagnostics", true)
	if !highRisk.Allowed || !highRisk.RequiresConfirmation {
		t.Fatalf("expected confirmation for high risk tool: %#v", highRisk)
	}

	blocked := policy.Check("delete_system32", false)
	if blocked.Allowed {
		t.Fatalf("unexpected allow: %#v", blocked)
	}
	if empty := policy.Check("  ", false); empty.Allowed {
		t.Fatalf("empty tool name must be rejected: %#v", empty)
	}
}
