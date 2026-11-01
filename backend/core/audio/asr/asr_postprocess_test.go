package audio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"norma/backend/core/audio"
)

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

// 用一个假的 whisper 可执行文件验证：带提示词时复述提示词，重试不带提示词时给出真实内容
func fakeWhisper(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "whisper-fake")
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func transcribe(t *testing.T, bin string) (string, error) {
	t.Helper()
	w := NewWhisperCPPASR(bin, "model.bin", "zh", 1)
	w.TempDir = t.TempDir()
	var got string
	err := w.TranscribeFrames(context.Background(), [][]float32{make([]float32, 1600)}, nil, func(s string) { got = s })
	return got, err
}

func TestWhisperRetriesWithoutPromptWhenItEchoesThePrompt(t *testing.T) {
	bin := fakeWhisper(t, `case "$*" in *--prompt*) echo "用简体中文。";; *) echo "我叫斯凯";; esac`)
	got, err := transcribe(t, bin)
	if err != nil || got != "我叫斯凯" {
		t.Fatalf("got %q err %v, want the retried real content", got, err)
	}
}

func TestWhisperEmptyResultIsNoSpeechNotAFailure(t *testing.T) {
	_, err := transcribe(t, fakeWhisper(t, `echo ""`))
	if !errors.Is(err, audio.ErrNoSpeech) {
		t.Fatalf("empty transcription must be ErrNoSpeech, got %v", err)
	}
}

func TestWhisperRealFailureIsStillAnError(t *testing.T) {
	_, err := transcribe(t, fakeWhisper(t, `echo boom >&2; exit 3`))
	if err == nil || errors.Is(err, audio.ErrNoSpeech) {
		t.Fatalf("a crashing whisper must be a real error, got %v", err)
	}
}

type stubFrameASR struct{ result string }

func (s stubFrameASR) Stream(ctx context.Context, p func(string), f func(string)) error { return nil }
func (s stubFrameASR) TranscribeFrames(ctx context.Context, frames [][]float32, p func(string), f func(string)) error {
	if f != nil {
		f(s.result)
	}
	return nil
}

func TestDumpingASRSavesAudioAndPassesResultThrough(t *testing.T) {
	dir := t.TempDir()
	d := NewDumpingASR(stubFrameASR{result: "我叫斯凯"}, dir)
	frame := make([]float32, 1600)
	for i := range frame {
		frame[i] = 0.25
	}
	var got string
	if err := d.TranscribeFrames(context.Background(), [][]float32{frame, frame}, nil, func(s string) { got = s }); err != nil {
		t.Fatal(err)
	}
	if got != "我叫斯凯" {
		t.Fatalf("result not passed through: %q", got)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "utt-*.wav"))
	if len(files) != 1 {
		t.Fatalf("expected one dumped wav, got %v", files)
	}
	info, _ := os.Stat(files[0])
	if info.Size() != 44+3200*2 { // 44 字节头 + 3200 个 16 位采样
		t.Fatalf("wav size = %d", info.Size())
	}
}
