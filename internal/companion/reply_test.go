package companion

import "testing"

func TestRewritePersonaAddsConclusionLead(t *testing.T) {
	text := RewritePersona("这是一个测试回答。后面还有补充说明。")
	if text == "" {
		t.Fatalf("expected non-empty rewritten text")
	}
	if len([]rune(text)) < 4 || string([]rune(text)[:4]) != "先给结论" {
		t.Fatalf("expected conclusion-led rewrite, got %q", text)
	}
}

func TestApplyDeliveryStyleAddsProsodyMarkup(t *testing.T) {
	out := ApplyDeliveryStyle("你好", DeliverySpec{PauseMs: 240, Emphasis: "strong"}, true)
	if out == "你好" {
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
