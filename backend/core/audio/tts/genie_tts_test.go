package audio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// 假的 Genie 工作进程：测试二进制以 GENIE_FAKE 环境变量重新执行自己
func TestGenieFakeWorker(t *testing.T) {
	mode := os.Getenv("GENIE_FAKE")
	if mode == "" {
		t.Skip("helper process")
	}
	if mode == "exit-early" {
		os.Exit(3)
	}
	out := bufio.NewWriter(os.Stdout)
	fmt.Fprintln(out, `{"ready": true}`)
	out.Flush()
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		var req struct {
			ID      int64  `json:"id"`
			Text    string `json:"text"`
			Emotion string `json:"emotion"`
		}
		_ = json.Unmarshal(sc.Bytes(), &req)
		if req.Text == "slow" {
			time.Sleep(400 * time.Millisecond)
		}
		if req.Text == "crash" {
			os.Exit(4)
		}
		payload := []byte(req.Emotion + "|" + req.Text)
		fmt.Fprintf(out, `{"id": %d, "ok": true, "sample_rate": 32000, "bytes": %d}`+"\n", req.ID, len(payload))
		out.Write(payload)
		out.Flush()
	}
	os.Exit(0)
}

func fakeGenie(t *testing.T, mode string) *GenieTTS {
	t.Helper()
	t.Setenv("GENIE_FAKE", mode)
	g := NewGenieTTS(GenieConfig{
		Command:      []string{os.Args[0], "-test.run=TestGenieFakeWorker", "--"},
		DataDir:      "d",
		CharacterDir: "c",
		Refs:         map[string]GenieRef{"default": {Audio: "a.wav", Text: "t"}},
		StartTimeout: 10 * time.Second,
	})
	t.Cleanup(g.Close)
	return g
}

func synth(t *testing.T, g *GenieTTS, ctx context.Context, text string) (string, error) {
	t.Helper()
	var got []byte
	err := g.Stream(ctx, text, func(c []byte) { got = append(got, c...) })
	return string(got), err
}

func TestGenieEmotionFromText(t *testing.T) {
	cases := map[string]string{
		"{{smile}}你好":           "smile",
		"你好{{expression:soft}}": "soft",
		"{{motion:thinking}}嗯":  "thinking",
		"没有标记":                  "default",
	}
	for in, want := range cases {
		if got := emotionFromText(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestGenieStreamRoutesEmotionAndStripsTags(t *testing.T) {
	g := fakeGenie(t, "ok")
	got, err := synth(t, g, context.Background(), "{{smile}}你好呀{{idle}}")
	if err != nil {
		t.Fatal(err)
	}
	if got != "smile|你好呀" {
		t.Fatalf("worker saw %q", got)
	}
	got, _ = synth(t, g, context.Background(), "普通一句")
	if got != "default|普通一句" {
		t.Fatalf("worker saw %q", got)
	}
}

func TestGenieEmptyTextAfterCleaningIsNoop(t *testing.T) {
	g := fakeGenie(t, "ok")
	if got, err := synth(t, g, context.Background(), "{{idle}}"); err != nil || got != "" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestGenieCancelDoesNotLeakStaleResult(t *testing.T) {
	g := fakeGenie(t, "ok")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := synth(t, g, ctx, "slow"); done <- err }()
	time.Sleep(80 * time.Millisecond)
	start := time.Now()
	_ = g.Stop("s")
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("cancel did not return promptly")
	}
	if time.Since(start) > 250*time.Millisecond {
		t.Fatal("cancel too slow")
	}
	cancel()
	// 紧接着的新请求必须拿到自己的结果，而不是上一个被取消请求的迟到结果
	got, err := synth(t, g, context.Background(), "下一句")
	if err != nil || got != "default|下一句" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestGenieRestartsAfterCrash(t *testing.T) {
	g := fakeGenie(t, "ok")
	if _, err := synth(t, g, context.Background(), "crash"); err == nil {
		t.Fatal("expected error when worker dies")
	}
	got, err := synth(t, g, context.Background(), "恢复")
	if err != nil || got != "default|恢复" {
		t.Fatalf("after restart got %q err %v", got, err)
	}
}

func TestGenieStartFailureIsReported(t *testing.T) {
	g := fakeGenie(t, "exit-early")
	if err := g.Warmup(); err == nil || !strings.Contains(err.Error(), "before ready") {
		t.Fatalf("err = %v", err)
	}
}
