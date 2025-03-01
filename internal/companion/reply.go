package companion

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

type EmotionSpec struct {
	Emotion   string  `json:"emotion"`
	Motion    string  `json:"motion"`
	Intensity float64 `json:"intensity"`
}

type ReplyPlan struct {
	Raw        string   `json:"raw"`
	Conclusion string   `json:"conclusion"`
	Segments   []string `json:"segments"`
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
	return ReplyPlan{Raw: cleaned, Conclusion: segments[0], Segments: segments}
}

func AnalyzeEmotion(text string) EmotionSpec {
	lower := strings.ToLower(strings.TrimSpace(text))
	spec := EmotionSpec{Emotion: "neutral", Motion: "Idle", Intensity: 0.45}
	switch {
	case containsAny(lower, "抱歉", "sorry", "遗憾", "不好意思"):
		spec = EmotionSpec{Emotion: "apology", Motion: "Idle", Intensity: 0.35}
	case containsAny(lower, "太好了", "很好", "完成", "成功", "great", "done"):
		spec = EmotionSpec{Emotion: "positive", Motion: "Tap", Intensity: 0.72}
	case containsAny(lower, "注意", "警告", "风险", "失败", "error", "warning"):
		spec = EmotionSpec{Emotion: "alert", Motion: "Flick", Intensity: 0.82}
	case containsAny(lower, "建议", "可以这样", "步骤", "先", "然后"):
		spec = EmotionSpec{Emotion: "guide", Motion: "Tap", Intensity: 0.58}
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
