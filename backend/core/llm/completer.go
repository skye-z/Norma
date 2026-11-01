package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatCompleter 是非流式的一次性补全，用于后台整理记忆这类不需要边生成边播放的调用
type ChatCompleter struct {
	endpoint string
	model    string
	apiKey   string
	client   *http.Client
}

func NewChatCompleter(endpoint, model, apiKey string, timeout time.Duration) *ChatCompleter {
	return &ChatCompleter{endpoint: normalizeOpenAIEndpoint(endpoint), model: model, apiKey: apiKey, client: &http.Client{Timeout: timeout}}
}

// Complete 发送一条 system 与一条 user 消息，温度为 0，关闭思考模式，返回模型文本
func (c *ChatCompleter) Complete(ctx context.Context, system, user string, maxTokens int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":       c.model,
		"messages":    []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
		"stream":      false,
		"max_tokens":  maxTokens,
		"temperature": 0,
		"thinking":    map[string]string{"type": "disabled"},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.apiKey))
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("chat completion http %d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return "", fmt.Errorf("chat completion decode failed")
	}
	return out.Choices[0].Message.Content, nil
}
