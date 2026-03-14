// Package audio 音频处理
package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ASR 客户端
type HTTPASR struct {
	Endpoint       string
	StreamEndpoint string
	Client         *http.Client
}

// asrRequest ASR 请求
type asrRequest struct {
	SessionID string `json:"session_id"`
}

// asrResponse ASR 响应
type asrResponse struct {
	Text     string   `json:"text"`
	Partials []string `json:"partials"`
	Final    string   `json:"final"`
}

// 创建 HTTP ASR
func NewHTTPASR(endpoint string, timeout time.Duration) *HTTPASR {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPASR{
		Endpoint: strings.TrimRight(endpoint, "/"),
		Client:   &http.Client{Timeout: timeout},
	}
}

// 流式识别
func (a *HTTPASR) Stream(ctx context.Context, onPartial func(string), onFinal func(string)) error {
	if a.Endpoint == "" {
		return fmt.Errorf("asr endpoint is empty")
	}

	body, _ := json.Marshal(asrRequest{SessionID: "default"})
	resp, err := a.post(ctx, strings.TrimRight(firstNonEmpty(a.StreamEndpoint, a.Endpoint), "/")+"/transcribe", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("asr status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var out asrResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}

	partials := out.Partials
	if len(partials) == 0 {
		partials = []string{"processing"}
	}
	for _, partial := range partials {
		if onPartial != nil {
			onPartial(strings.TrimSpace(partial))
		}
	}
	final := strings.TrimSpace(firstNonEmpty(out.Final, out.Text))
	if final != "" && onFinal != nil {
		onFinal(final)
	}
	return nil
}

func (a *HTTPASR) post(ctx context.Context, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return a.Client.Do(req)
}

// calls a local XTTS-like HTTP service and streams by sentence.
type XTTSStreamer struct {
	Endpoint string
	Voice    string
	Client   *http.Client

	mu      sync.Mutex
	stopped bool
}

func (t *XTTSStreamer) SupportsProsodyHints() bool { return true }

func (t *XTTSStreamer) SampleRate() int { return 24000 }

func (t *XTTSStreamer) Channels() int { return 1 }

func (t *XTTSStreamer) Encoding() string { return "pcm_s16le" }

type ttsRequest struct {
	Text  string `json:"text"`
	Voice string `json:"voice"`
}

func NewXTTSStreamer(endpoint, voice string, timeout time.Duration) *XTTSStreamer {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &XTTSStreamer{
		Endpoint: strings.TrimRight(endpoint, "/"),
		Voice:    voice,
		Client:   &http.Client{Timeout: timeout},
	}
}

func (t *XTTSStreamer) Stream(ctx context.Context, text string, onChunk func(chunk []byte)) error {
	if t.Endpoint == "" {
		return fmt.Errorf("tts endpoint is empty")
	}

	t.mu.Lock()
	t.stopped = false
	t.mu.Unlock()

	parts := prioritizeSentenceParts(splitSentences(text))
	if len(parts) == 0 {
		parts = []string{text}
	}

	for _, part := range parts {
		if t.isStopped() {
			return context.Canceled
		}

		data, err := t.requestOnce(ctx, strings.TrimSpace(part))
		if err != nil {
			return err
		}

		chunkSize := 8 * 1024
		for i := 0; i < len(data); i += chunkSize {
			end := i + chunkSize
			if end > len(data) {
				end = len(data)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if t.isStopped() {
					return context.Canceled
				}
				onChunk(data[i:end])
			}
		}
	}

	return nil
}

func (t *XTTSStreamer) requestOnce(ctx context.Context, text string) ([]byte, error) {
	body, _ := json.Marshal(ttsRequest{Text: text, Voice: t.Voice})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.Endpoint+"/tts", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("tts status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("tts returned empty audio")
	}

	return data, nil
}

func (t *XTTSStreamer) Stop(sessionID string) error {
	_ = sessionID
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
	return nil
}

func (t *XTTSStreamer) isStopped() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stopped
}

func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	seps := map[rune]struct{}{
		'\u3002': {}, '\uFF0C': {}, '\uFF1B': {}, ';': {}, '\uFF01': {}, '.': {}, '!': {}, '?': {}, '\uFF1F': {}, '\n': {},
	}

	parts := make([]string, 0, 8)
	var cur strings.Builder
	for _, r := range text {
		cur.WriteRune(r)
		if _, ok := seps[r]; ok {
			p := strings.TrimSpace(cur.String())
			if p != "" {
				parts = append(parts, p)
			}
			cur.Reset()
		}
	}

	tail := strings.TrimSpace(cur.String())
	if tail != "" {
		parts = append(parts, tail)
	}

	return parts
}

func prioritizeSentenceParts(parts []string) []string {
	if len(parts) <= 1 {
		return parts
	}
	short := make([]string, 0, len(parts))
	long := make([]string, 0, len(parts))
	for _, part := range parts {
		if len([]rune(strings.TrimSpace(part))) <= 10 {
			short = append(short, part)
			continue
		}
		long = append(long, part)
	}
	return append(short, long...)
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return ""
}
