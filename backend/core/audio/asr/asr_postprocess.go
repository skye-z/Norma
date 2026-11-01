package audio

import (
	"regexp"
	"strings"
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
	if whisperHallucinationRe.MatchString(out) {
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
