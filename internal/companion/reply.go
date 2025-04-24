package companion

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type EmotionSpec struct {
	Emotion    string             `json:"emotion"`
	Motion     string             `json:"motion"`
	Expression string             `json:"expression"`
	Intensity  float64            `json:"intensity"`
	Params     map[string]float64 `json:"params,omitempty"`
}

type ReplyPlan struct {
	Raw        string         `json:"raw"`
	Styled     string         `json:"styled"`
	Conclusion string         `json:"conclusion"`
	Segments   []string       `json:"segments"`
	Delivery   []DeliverySpec `json:"delivery,omitempty"`
}

type DeliverySpec struct {
	Text      string  `json:"text"`
	Phase     string  `json:"phase"`
	PauseMs   int     `json:"pause_ms"`
	Emphasis  string  `json:"emphasis"`
	Energy    float64 `json:"energy"`
	Emotion   string  `json:"emotion"`
	StyleHint string  `json:"style_hint"`
}

type PersonaRuntimeConfig struct {
	KeepMicCaptureAlive bool `json:"keep_mic_capture_alive"`
	NeedRealAECNS       bool `json:"need_real_aec_ns"`
}

type VisemeFrame struct {
	Phoneme   string  `json:"phoneme"`
	Intensity float64 `json:"intensity"`
	Duration  int     `json:"duration_ms"`
}

var splitRE = regexp.MustCompile(`[。！？!?；;\n]+`)

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

func RewritePersona(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "??????????"
	}
	text = strings.ReplaceAll(text, "???", "?????")
	text = strings.ReplaceAll(text, "??", "???")
	text = strings.ReplaceAll(text, "?????", "")
	text = strings.ReplaceAll(text, "????", "")
	text = strings.ReplaceAll(text, "??????", "")
	text = strings.ReplaceAll(text, "?????", "")
	text = strings.TrimLeft(text, "?:?,? ")
	return strings.TrimSpace(text)
}

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
		if containsAny(segment, "注意", "风险", "不要", "必须") {
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

func AnalyzeEmotion(text string) EmotionSpec {
	lower := strings.ToLower(strings.TrimSpace(text))
	spec := EmotionSpec{Emotion: "neutral", Motion: "Idle", Intensity: 0.45}
	switch {
	case containsAny(lower, "抱歉", "sorry", "遗憾", "不好意思"):
		spec = EmotionSpec{Emotion: "apology", Motion: "Idle", Expression: "soft", Intensity: 0.35, Params: map[string]float64{"ParamBrowAngleY": -0.25, "ParamEyeLOpen": 0.85, "ParamEyeROpen": 0.85}}
	case containsAny(lower, "太好了", "很好", "完成", "成功", "great", "done"):
		spec = EmotionSpec{Emotion: "positive", Motion: "Tap", Expression: "smile", Intensity: 0.72, Params: map[string]float64{"ParamMouthForm": 0.55, "ParamCheek": 0.35, "ParamBrowAngleY": 0.18}}
	case containsAny(lower, "注意", "警告", "风险", "失败", "error", "warning"):
		spec = EmotionSpec{Emotion: "alert", Motion: "Flick", Expression: "alert", Intensity: 0.82, Params: map[string]float64{"ParamEyeBallX": 0.08, "ParamBrowAngleY": 0.32, "ParamMouthForm": -0.18}}
	case containsAny(lower, "建议", "可以这样", "步骤", "先", "然后"):
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

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(value)) {
			return true
		}
	}
	return false
}

func classifyRune(r rune) string {
	switch strings.ToLower(string(r)) {
	case "a", "啊", "呀":
		return "a"
	case "o", "喔", "哦":
		return "o"
	case "e", "欸", "诶":
		return "e"
	case "i", "y", "衣", "一":
		return "i"
	case "u", "w", "乌", "呜":
		return "u"
	default:
		return "mid"
	}
}

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
