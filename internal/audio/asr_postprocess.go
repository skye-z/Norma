package audio

import (
	"regexp"
	"strings"
)

var wakeKeywordNearHomophoneRe = regexp.MustCompile(
	`^(弄嘛|弄吗|弄玛|诺吗|糯玛|喏玛|諾瑪|诺馬|诺麻|糯吗|糯麻)([s,，。！？!?.：:]*)`,
)

var introTailNoiseRe = regexp.MustCompile(
	`(请介绍一下你自己)(的?(影片|视频|視頻))([。.!！？?]?)`,
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

func PostProcessASRText(text, wakeKeyword string) string {
	out := strings.TrimSpace(text)
	if out == "" {
		return ""
	}

	out = zhHansCharReplacer.Replace(out)

	keyword := strings.TrimSpace(wakeKeyword)
	if keyword != "" {
		out = wakeKeywordNearHomophoneRe.ReplaceAllString(out, keyword+`2`)
	}

	out = introTailNoiseRe.ReplaceAllString(out, `14`)
	out = strings.ReplaceAll(out, "，,", "，")
	out = strings.ReplaceAll(out, ",，", "，")
	out = strings.ReplaceAll(out, "。。", "。")
	return strings.TrimSpace(out)
}
