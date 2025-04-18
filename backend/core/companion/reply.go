// Package companion 陪伴功能
package companion

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// EmotionSpec 情感规格
type EmotionSpec struct {
	Emotion    string             `json:"emotion"`
	Motion     string             `json:"motion"`
	Expression string             `json:"expression"`
	Intensity  float64            `json:"intensity"`
	Params     map[string]float64 `json:"params,omitempty"`
}

// ReplyPlan 回复计划
type ReplyPlan struct {
	Raw        string         `json:"raw"`
	Styled     string         `json:"styled"`
	Conclusion string         `json:"conclusion"`
	Segments   []string       `json:"segments"`
	Delivery   []DeliverySpec `json:"delivery,omitempty"`
}

// DeliverySpec 投递规格
type DeliverySpec struct {
	Text      string  `json:"text"`
	Phase     string  `json:"phase"`
	PauseMs   int     `json:"pause_ms"`
	Emphasis  string  `json:"emphasis"`
	Energy    float64 `json:"energy"`
	Emotion   string  `json:"emotion"`
	StyleHint string  `json:"style_hint"`
}

// PersonaRuntimeConfig 角色运行时配置
type PersonaRuntimeConfig struct {
	KeepMicCaptureAlive bool `json:"keep_mic_capture_alive"`
	NeedRealAECNS       bool `json:"need_real_aec_ns"`
}

// VisemeFrame 视素帧
type VisemeFrame struct {
	Phoneme   string  `json:"phoneme"`
	Intensity float64 `json:"intensity"`
	Duration  int     `json:"duration_ms"`
}

// Live2DControl Live2D 控制
type Live2DControl struct {
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	RawMarker string         `json:"raw_marker"`
	Payload   map[string]any `json:"payload,omitempty"`
}

var (
	splitRE          = regexp.MustCompile("[\u3002\uff01\uff1f!??;\n]+")
	live2dMarkerRE   = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
	spaceBeforePunct = regexp.MustCompile("\\s+([\uFF0C\u3002\uFF01\uFF1F\uFF1B\uFF1A,.!?;:])")
	multiSpaceRE     = regexp.MustCompile(`\s{2,}`)
)

var live2DMotionAliases = map[string]string{
	"thinking":     "thinking",
	"think":        "thinking",
	"\u601d\u8003": "thinking",
	"idle":         "idle",
	"\u5f85\u673a": "idle",
	"tap":          "Tap",
	"nod":          "Tap",
	"\u70b9\u5934": "Tap",
	"flick":        "Flick",
	"alert":        "Flick",
	"\u8b66\u89c9": "Flick",
	"wave":         "Wave",
	"\u6325\u624b": "Wave",
}

var live2DExpressionAliases = map[string]string{
	"smile":        "smile",
	"\u5fae\u7b11": "smile",
	"soft":         "soft",
	"\u6e29\u548c": "soft",
	"focus":        "focus",
	"\u4e13\u6ce8": "focus",
	"alert":        "alert",
	"\u8b66\u89c9": "alert",
	"neutral":      "neutral",
	"\u5e73\u9759": "neutral",
}

// PlanReply 生成回复计划
func PlanReply(text string) ReplyPlan {
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return ReplyPlan{}
	}
	parts := splitRE.Split(cleaned, -1)
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments = append(segments, part)
	}
	if len(segments) == 0 {
		segments = []string{cleaned}
	}
	return ReplyPlan{Raw: cleaned, Styled: cleaned, Conclusion: segments[0], Segments: segments}
}

// RewritePersona 重写角色设定
func RewritePersona(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "\u62b1\u6b49\uff0c\u6211\u73b0\u5728\u6ca1\u6709\u53ef\u7528\u56de\u7b54\u3002"
	}
	replacements := []struct {
		old string
		new string
	}{
		{"\u5148\u7ed9\u7ed3\u8bba\uff1a", ""},
		{"\u4e00\u53e5\u8bdd\u603b\u7ed3\uff1a", ""},
		{"\u7ed3\u8bba\uff1a", ""},
		{"\u603b\u7ed3\uff1a", ""},
	}
	for _, item := range replacements {
		text = strings.ReplaceAll(text, item.old, item.new)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "\u62b1\u6b49\uff0c\u6211\u73b0\u5728\u6ca1\u6709\u53ef\u7528\u56de\u7b54\u3002"
	}
	return text
}

// ExtractLive2DControls 提取 Live2D 控制指令
func ExtractLive2DControls(text string) (string, []Live2DControl) {
	matches := live2dMarkerRE.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return strings.TrimSpace(text), nil
	}
	controls := make([]Live2DControl, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		body := strings.TrimSpace(match[1])
		if control, ok := parseLive2DControl(body, match[0]); ok {
			controls = append(controls, control)
		}
	}
	cleaned := live2dMarkerRE.ReplaceAllString(text, " ")
	cleaned = spaceBeforePunct.ReplaceAllString(cleaned, "$1")
	cleaned = multiSpaceRE.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned, controls
}

// parseLive2DControl 解析 Live2D 控制指令
func parseLive2DControl(body, raw string) (Live2DControl, bool) {
	kind := ""
	name := body
	if idx := strings.Index(body, ":"); idx >= 0 {
		kind = strings.ToLower(strings.TrimSpace(body[:idx]))
		name = strings.TrimSpace(body[idx+1:])
	}
	aliasKey := strings.ToLower(strings.TrimSpace(name))
	aliasKey = strings.ReplaceAll(aliasKey, "\u52a8\u4f5c", "")
	aliasKey = strings.ReplaceAll(aliasKey, "\u8868\u60c5", "")
	aliasKey = strings.TrimSpace(aliasKey)
	if aliasKey == "" {
		return Live2DControl{}, false
	}
	if kind == "" || kind == "motion" || kind == "\u52a8\u4f5c" {
		if motion, ok := live2DMotionAliases[aliasKey]; ok {
			return Live2DControl{Kind: "motion", Name: motion, RawMarker: raw, Payload: map[string]any{"group": motion, "priority": 92, "interruptible": true, "source": "llm"}}, true
		}
	}
	if kind == "" || kind == "expression" || kind == "\u8868\u60c5" {
		if expression, ok := live2DExpressionAliases[aliasKey]; ok {
			return Live2DControl{Kind: "expression", Name: expression, RawMarker: raw, Payload: map[string]any{"name": expression, "hold_ms": 1400, "source": "llm"}}, true
		}
	}
	return Live2DControl{}, false
}

// BuildDeliveryPlan 构建投递计划
func BuildDeliveryPlan(plan ReplyPlan, emotion EmotionSpec) []DeliverySpec {
	out := make([]DeliverySpec, 0, len(plan.Segments))
	for index, segment := range plan.Segments {
		phase := "supplement"
		pauseMs := 180
		emphasis := "medium"
		energy := emotion.Intensity
		styleHint := "steady"
		if index == 0 {
			phase = "conclusion"
			pauseMs = 260
			emphasis = "strong"
			energy += 0.08
			styleHint = "lead_with_conclusion"
		} else if len(segment) > 24 {
			pauseMs = 220
			styleHint = "paced"
		}
		if containsAny(segment, "\u6ce8\u610f", "\u98ce\u9669", "\u4e0d\u8981", "\u5fc5\u987b") {
			emphasis = "strong"
			styleHint = "warning"
		}
		if energy < 0.22 {
			energy = 0.22
		}
		if energy > 1 {
			energy = 1
		}
		out = append(out, DeliverySpec{Text: segment, Phase: phase, PauseMs: pauseMs, Emphasis: emphasis, Energy: energy, Emotion: emotion.Emotion, StyleHint: styleHint})
	}
	return out
}

// ApplyDeliveryStyle 应用投递风格
func ApplyDeliveryStyle(text string, spec DeliverySpec, supportsProsody bool) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	if !supportsProsody {
		return text
	}
	prefix := ""
	suffix := ""
	if spec.Emphasis == "strong" {
		prefix += "<emphasis level=\"strong\">"
		suffix = "</emphasis>" + suffix
	}
	if spec.PauseMs > 0 {
		suffix += "<break time=\"" + strconv.Itoa(spec.PauseMs) + "ms\"/>"
	}
	return prefix + text + suffix
}

// BuildPersonaRuntimeConfig 构建角色运行时配置
func BuildPersonaRuntimeConfig(delivery []DeliverySpec, interruptDuringSpeak bool) PersonaRuntimeConfig {
	keepAlive := interruptDuringSpeak
	needAECNS := false
	for _, item := range delivery {
		if item.Emphasis == "strong" || item.StyleHint == "warning" {
			needAECNS = true
		}
	}
	return PersonaRuntimeConfig{KeepMicCaptureAlive: keepAlive, NeedRealAECNS: needAECNS}
}

// AnalyzeEmotion 分析情感
func AnalyzeEmotion(text string) EmotionSpec {
	lower := strings.ToLower(strings.TrimSpace(text))
	spec := EmotionSpec{Emotion: "neutral", Motion: "Idle", Intensity: 0.45}
	switch {
	case containsAny(lower, "\u62b1\u6b49", "sorry", "\u9057\u61be", "\u4e0d\u597d\u610f\u601d"):
		spec = EmotionSpec{Emotion: "apology", Motion: "Idle", Expression: "soft", Intensity: 0.35, Params: map[string]float64{"ParamBrowAngleY": -0.25, "ParamEyeLOpen": 0.85, "ParamEyeROpen": 0.85}}
	case containsAny(lower, "\u592a\u597d\u4e86", "\u5f88\u597d", "\u5b8c\u6210", "\u6210\u529f", "great", "done"):
		spec = EmotionSpec{Emotion: "positive", Motion: "Tap", Expression: "smile", Intensity: 0.72, Params: map[string]float64{"ParamMouthForm": 0.55, "ParamCheek": 0.35, "ParamBrowAngleY": 0.18}}
	case containsAny(lower, "\u6ce8\u610f", "\u8b66\u544a", "\u98ce\u9669", "\u5931\u8d25", "error", "warning"):
		spec = EmotionSpec{Emotion: "alert", Motion: "Flick", Expression: "alert", Intensity: 0.82, Params: map[string]float64{"ParamEyeBallX": 0.08, "ParamBrowAngleY": 0.32, "ParamMouthForm": -0.18}}
	case containsAny(lower, "\u5efa\u8bae", "\u53ef\u4ee5\u8fd9\u6837", "\u6b65\u9aa4", "\u5148", "\u7136\u540e"):
		spec = EmotionSpec{Emotion: "guide", Motion: "Tap", Expression: "focus", Intensity: 0.58, Params: map[string]float64{"ParamBrowAngleY": 0.12, "ParamMouthForm": 0.14, "ParamEyeLOpen": 0.96, "ParamEyeROpen": 0.96}}
	}
	if utf8.RuneCountInString(lower) > 120 {
		spec.Intensity -= 0.08
	}
	if spec.Intensity < 0.2 {
		spec.Intensity = 0.2
	}
	if spec.Intensity > 1 {
		spec.Intensity = 1
	}
	return spec
}

// EstimateVisemes 估算视素
func EstimateVisemes(text string) []VisemeFrame {
	frames := make([]VisemeFrame, 0, utf8.RuneCountInString(text))
	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\t' {
			continue
		}
		frames = append(frames, VisemeFrame{
			Phoneme:   classifyRune(r),
			Intensity: visemeIntensity(r),
			Duration:  70,
		})
	}
	if len(frames) == 0 {
		frames = append(frames, VisemeFrame{Phoneme: "rest", Intensity: 0, Duration: 60})
	}
	return frames
}

// containsAny 检查是否包含任意关键字
func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(value)) {
			return true
		}
	}
	return false
}

// classifyRune 分类字符为视素
func classifyRune(r rune) string {
	switch strings.ToLower(string(r)) {
	case "a", "\u554a", "\u5440":
		return "a"
	case "o", "\u5594", "\u54e6":
		return "o"
	case "e", "\u6b38", "\u8bf6":
		return "e"
	case "i", "y", "\u8863", "\u4e00":
		return "i"
	case "u", "w", "\u4e4c", "\u545c":
		return "u"
	default:
		return "mid"
	}
}

// visemeIntensity 获取视素强度
func visemeIntensity(r rune) float64 {
	switch classifyRune(r) {
	case "a":
		return 1
	case "o", "e":
		return 0.82
	case "i", "u":
		return 0.6
	case "mid":
		return 0.42
	default:
		return 0.3
	}
}
