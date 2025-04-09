package companion

import "testing"

func TestRewritePersonaRemovesConclusionLead(t *testing.T) {
	text := RewritePersona("\u5148\u7ed9\u7ed3\u8bba\uff1a\u8fd9\u662f\u4e00\u4e2a\u6d4b\u8bd5\u56de\u7b54\u3002")
	if text == "" {
		t.Fatalf("expected non-empty rewritten text")
	}
	if text == "\u5148\u7ed9\u7ed3\u8bba\uff1a\u8fd9\u662f\u4e00\u4e2a\u6d4b\u8bd5\u56de\u7b54\u3002" {
		t.Fatalf("expected rewrite to remove lead prefix, got %q", text)
	}
}

func TestExtractLive2DControls(t *testing.T) {
	cleaned, controls := ExtractLive2DControls("{{thinking}}\u6211\u5728\u3002{{expression:smile}}")
	if cleaned == "" {
		t.Fatalf("expected cleaned text")
	}
	if len(controls) != 2 {
		t.Fatalf("expected 2 controls, got %d", len(controls))
	}
	if controls[0].Kind != "motion" || controls[0].Name != "thinking" {
		t.Fatalf("unexpected first control: %+v", controls[0])
	}
	if controls[1].Kind != "expression" || controls[1].Name != "smile" {
		t.Fatalf("unexpected second control: %+v", controls[1])
	}
}

func TestApplyDeliveryStyleAddsProsodyMarkup(t *testing.T) {
	out := ApplyDeliveryStyle("\u4f60\u597d", DeliverySpec{PauseMs: 240, Emphasis: "strong"}, true)
	if out == "\u4f60\u597d" {
		t.Fatalf("expected styled output")
	}
}

func TestBuildPersonaRuntimeConfigKeepsMicAliveForInterrupt(t *testing.T) {
	cfg := BuildPersonaRuntimeConfig([]DeliverySpec{{Emphasis: "strong", StyleHint: "warning"}}, true)
	if !cfg.KeepMicCaptureAlive {
		t.Fatalf("expected mic capture keepalive enabled")
	}
	if !cfg.NeedRealAECNS {
		t.Fatalf("expected real aec/ns hint enabled")
	}
}
