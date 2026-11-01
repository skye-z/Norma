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
