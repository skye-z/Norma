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

// Whisper 在音频很短或不清晰时会把起始提示词原样或稍作改动地吐出来（如“用简体中文。”“这个是普通话的句子，使用简体中文。”）。
// 去掉标点后，短结果按“是提示词的开头或结尾”判断；较长的结果按与提示词的最长公共子序列占比判断，
// 占比达到 75% 以上视为复述。正常提到“简体中文”的句子（如“帮我用简体中文写一句话”）占比远低于此
func isPromptEcho(text string) bool {
	t := []rune(stripPunctAndSpace(text))
	p := []rune(stripPunctAndSpace(defaultWhisperPrompt))
	if len(t) < 3 || len(p) == 0 {
		return false
	}
	ts, ps := string(t), string(p)
	if ts == ps || strings.HasPrefix(ps, ts) || strings.HasSuffix(ps, ts) {
		return true
	}
	if len(t) < 6 {
		return false
	}
	return float64(lcsLen(t, p))/float64(len(t)) >= 0.75
}

// 最长公共子序列长度
func lcsLen(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else if prev[j] >= cur[j-1] {
				cur[j] = prev[j]
			} else {
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
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
