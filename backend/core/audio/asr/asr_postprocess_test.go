package audio

import "testing"

func TestPostProcessASRText(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"wake near homophone", "弄嘛，请介绍一下你自己。", "诺玛，请介绍一下你自己。"},
		{"traditional wake keyword", "諾瑪，請介紹一下你自己", "诺玛，请介绍一下你自己"},
		{"clean sentence unchanged", "诺玛，请介绍一下你自己。", "诺玛，请介绍一下你自己。"},
		{"hallucination dropped", "谢谢大家收看!", ""},
		{"subtitle hallucination dropped", "字幕志愿者 张三", ""},
		{"normal sentence containing thanks kept", "谢谢你今天帮我查天气。", "谢谢你今天帮我查天气。"},
	}
	for _, c := range cases {
		if got := PostProcessASRText(c.in, "诺玛"); got != c.want {
			t.Errorf("%s: PostProcessASRText(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestPostProcessDropsWhisperPromptEcho(t *testing.T) {
	for _, in := range []string{"用简体中文。", "使用简体中文", "以下是普通话的句子，使用简体中文。", "的句子，使用简体中文"} {
		if got := PostProcessASRText(in, "诺玛"); got != "" {
			t.Errorf("%q should be treated as no speech, got %q", in, got)
		}
	}
}

func TestPostProcessKeepsRealSpeechThatMentionsSimilarWords(t *testing.T) {
	for _, in := range []string{"你好，请介绍一下你自己。", "今天天气怎么样", "帮我把这段话翻成简体中文再读一遍"} {
		if got := PostProcessASRText(in, "诺玛"); got == "" {
			t.Errorf("%q wrongly dropped", in)
		}
	}
}
