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

// 基于对话接口的查询扩展器，用云端大模型把问题改写为检索关键词
type ChatQueryExpander struct {
	endpoint string
	model    string
	apiKey   string
	client   *http.Client
}

// 创建查询扩展器
func NewChatQueryExpander(endpoint, model, apiKey string, timeout time.Duration) *ChatQueryExpander {
	return &ChatQueryExpander{
		endpoint: normalizeOpenAIEndpoint(endpoint),
		model:    model,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: timeout},
	}
}

const expandPrompt = "你是记忆检索助手。用户的问题会拿去匹配一份个人记忆笔记，笔记里的措辞可能与问题不同。" +
	"请列出笔记中可能出现的关键词与同义表达（名词、动词、具体事物，不要解释），用空格分隔，不超过15个词，只输出词语。"

// 扩展查询，返回关键词列表
func (e *ChatQueryExpander) Expand(ctx context.Context, query string) ([]string, error) {
	body, _ := json.Marshal(map[string]any{
		"model": e.model,
		"messages": []map[string]string{
			{"role": "system", "content": expandPrompt},
			{"role": "user", "content": query},
		},
		"stream":      false,
		"max_tokens":  80,
		"temperature": 0.2,
		"thinking":    map[string]string{"type": "disabled"},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("query expand http %d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return nil, fmt.Errorf("query expand decode failed")
	}
	return strings.Fields(strings.NewReplacer("，", " ", "、", " ", ",", " ", "\n", " ").Replace(out.Choices[0].Message.Content)), nil
}
