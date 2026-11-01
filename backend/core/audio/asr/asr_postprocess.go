package audio

import (
	"regexp"
	"strings"
	"unicode"
)

var wakeKeywordNearHomophoneRe = regexp.MustCompile(
	`^(弄嘛|弄吗|弄玛|诺吗|糯玛|喏玛|諾瑪|诺馬|诺麻|糯吗|糯麻|诺马)`,
)

// 静音或底噪下 Whisper 常见的幻觉输出，整句命中时视为无有效识别结果
var whisperHallucinationRe = regexp.MustCompile(
	`^[\s，。,.!！]*(谢谢(大家)?(收看|观看)|请不吝点赞.*|字幕(由|志愿者).*|多谢观看|Thanks for watching)[\s，。,.!！]*$`,
)

var zhHansCharReplacer = strings.NewReplacer(
	"請", "请",
	"介紹", "介绍",
	"這", "这",
	"個", "个",
	"嗎", "吗",
	"誰", "谁",
	"為", "为",
	"麼", "么",
	"說", "说",
	"聽", "听",
	"見", "见",
	"開", "开",
	"關", "关",
	"後", "后",
	"臺", "台",
	"語", "语",
	"視頻", "视频",
)

// PostProcessASRText 做繁简统一、唤醒词近音修正和幻觉过滤，不改写其余内容
func PostProcessASRText(text, wakeKeyword string) string {
	out := strings.TrimSpace(text)
	if out == "" {
		return ""
	}
	out = zhHansCharReplacer.Replace(out)
	if whisperHallucinationRe.MatchString(out) || isPromptEcho(out) {
		return ""
	}
	if keyword := strings.TrimSpace(wakeKeyword); keyword != "" {
		out = wakeKeywordNearHomophoneRe.ReplaceAllString(out, keyword)
	}
	out = strings.ReplaceAll(out, "，,", "，")
	out = strings.ReplaceAll(out, ",，", "，")
	out = strings.ReplaceAll(out, "。。", "。")
	return strings.TrimSpace(out)
}

// Whisper 在音频很短或不清晰时会把起始提示词原样吐出来（例如只识别出“用简体中文。”）。
// 去掉标点后，若结果是提示词的开头或结尾片段（至少 3 个字），就当作没有识别到内容
func isPromptEcho(text string) bool {
	t := stripPunctAndSpace(text)
	p := stripPunctAndSpace(defaultWhisperPrompt)
	if len([]rune(t)) < 3 || p == "" {
		return false
	}
	return strings.HasPrefix(p, t) || strings.HasSuffix(p, t) || t == p
}

func stripPunctAndSpace(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsPunct(r) || unicode.IsSpace(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
