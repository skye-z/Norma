// Package llm LLM 接口
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"norma/backend/utils"
)

// 流式响应器
type OllamaStreamer struct {
	endpoint string
	model    string
	apiKey   string
	client   *http.Client

	disableThinking bool
	maxTokens       int
	temperature     *float64
}

// 关闭思考模式，并设置生成上限与温度；用于 DeepSeek 等支持 thinking 开关的接口
func (o *OllamaStreamer) WithOptions(disableThinking bool, maxTokens int, temperature float64) *OllamaStreamer {
	o.disableThinking = disableThinking
	o.maxTokens = maxTokens
	if temperature > 0 {
		o.temperature = &temperature
	}
	return o
}

// openAIChatRequest OpenAI 聊天请求
type openAIChatRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	// DeepSeek 默认开启思考，语音场景需要关闭以缩短首字等待
	Thinking    *openAIThinking `json:"thinking,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
}

// openAIThinking 思考开关
type openAIThinking struct {
	Type string `json:"type"`
}

// openAIMessage OpenAI 消息
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIChatChunk OpenAI 聊天块
type openAIChatChunk struct {
	Choices []openAIChatChoice `json:"choices"`
	Error   *openAIError       `json:"error"`
}

// openAIChatChoice OpenAI 聊天选择
type openAIChatChoice struct {
	Delta        openAIChatDelta `json:"delta"`
	FinishReason string          `json:"finish_reason"`
}

// openAIChatDelta OpenAI 聊天增量
type openAIChatDelta struct {
	Content string `json:"content"`
}

// openAIError OpenAI 错误
type openAIError struct {
	Message string `json:"message"`
}

// openAIModelsResponse OpenAI 模型列表响应
type openAIModelsResponse struct {
	Data []openAIModel `json:"data"`
}

// openAIModel OpenAI 模型
type openAIModel struct {
	ID string `json:"id"`
}

// ollamaTagsResponse Ollama 标签响应
type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

// ollamaModel Ollama 模型
type ollamaModel struct {
	Name string `json:"name"`
}

// 创建 Ollama 流式响应器
func NewOllamaStreamer(endpoint, model, apiKey string, timeout time.Duration) *OllamaStreamer {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = "http://127.0.0.1:11434/v1"
	}
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	return &OllamaStreamer{
		endpoint: normalizeOpenAIEndpoint(endpoint),
		model:    model,
		apiKey:   strings.TrimSpace(apiKey),
		client:   &http.Client{Timeout: timeout},
	}
}

// 检查 Ollama 是否可用
func PingOllama(endpoint, apiKey string, timeout time.Duration) error {
	_, err := listModels(endpoint, apiKey, timeout)
	return err
}

// 发现第一个 LLM 模型
func DiscoverFirstLLMModel(endpoint, apiKey string, timeout time.Duration) (string, error) {
	models, err := listModels(endpoint, apiKey, timeout)
	if err != nil {
		return "", err
	}
	for _, m := range models {
		if isLikelyLLM(m.Name) {
			return m.Name, nil
		}
	}
	if len(models) > 0 {
		return models[0].Name, nil
	}
	return "", fmt.Errorf("no ollama models found")
}

// listModels 列出模型
func listModels(endpoint, apiKey string, timeout time.Duration) ([]ollamaModel, error) {
	endpoint = normalizeOpenAIEndpoint(endpoint)
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11434/v1"
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, endpoint+"/models", nil)
	if err != nil {
		return nil, err
	}

	if key := strings.TrimSpace(apiKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d", resp.StatusCode)
	}

	var out openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}
	models := make([]ollamaModel, 0, len(out.Data))
	for _, m := range out.Data {
		if strings.TrimSpace(m.ID) == "" {
			continue
		}
		models = append(models, ollamaModel{Name: m.ID})
	}
	return models, nil
}

// isLikelyLLM 检查是否可能是 LLM 模型
func isLikelyLLM(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return false
	}
	if strings.Contains(n, "embed") || strings.Contains(n, "rerank") {
		return false
	}
	return true
}

// 流式调用
func (o *OllamaStreamer) Stream(ctx context.Context, req Request, onToken func(string)) error {
	chatReq := openAIChatRequest{Model: o.model, Messages: buildMessages(req), Stream: true, MaxTokens: o.maxTokens, Temperature: o.temperature}
	if o.disableThinking {
		chatReq.Thinking = &openAIThinking{Type: "disabled"}
	}
	body, err := json.Marshal(chatReq)
	if err != nil {
		return fmt.Errorf("marshal ollama request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(o.apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(o.apiKey))
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("call ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ollama status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "[DONE]" {
			return nil
		}
		var chunk openAIChatChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return fmt.Errorf("decode ollama stream: %w", err)
		}
		if chunk.Error != nil && strings.TrimSpace(chunk.Error.Message) != "" {
			return fmt.Errorf("ollama stream error: %s", strings.TrimSpace(chunk.Error.Message))
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				onToken(choice.Delta.Content)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read ollama stream: %w", err)
	}
	return nil
}

// normalizeOpenAIEndpoint 标准化 OpenAI 端点
func normalizeOpenAIEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return endpoint
	}
	if !strings.HasSuffix(endpoint, "/v1") {
		endpoint = endpoint + "/v1"
	}
	return endpoint
}

// buildSystemPrompt 构建 system 消息：人设、回答风格与 Live2D 标识规则
func buildSystemPrompt(req Request) string {
	persona := strings.TrimSpace(req.Persona)
	if persona == "" {
		persona = config.DefaultLLMPersona
	}
	var sb strings.Builder
	sb.WriteString(persona)
	sb.WriteString("\n请自然、口语化地回答，控制在 2-4 句，优先直接回应用户，不要写成流程说明，不要 OOC。\n")
	// 没有这一条时模型会把人设当成“用户刚做的自我介绍”，每一轮都先说一遍“我是诺玛……”
	sb.WriteString("你和用户已经在持续对话中：不要主动自我介绍，也不要复述自己的身份、职责或所属机构，除非用户明确问你是谁。直接回应用户刚说的这句话。\n")
	// 注意措辞：只说“要自然地用起来”会让模型觉得自己“应该记得”，没有记录时就编造（实测问名字 4/8 次编出同一个名字）。
	// 所以同时划清边界：只有下面“已了解的信息”和本次对话里用户亲口说的才算数
	sb.WriteString("你只知道两类关于用户的信息：下面“已了解的信息”，以及这次对话里用户亲口说过的话；这些要自然地用起来，不必反复确认。")
	sb.WriteString("被问到你并不知道的个人信息（名字、喜好、家人、宠物、经历等）时，如实说你还不知道，可以请对方告诉你；绝不要编造，也不要用“你之前提过”“我记得”去支撑一个并不存在的记忆。\n")
	sb.WriteString("你可以在回答中少量插入 Live2D 控制标识，格式是 {{动作}} 或 {{类型:动作}}。\n")
	sb.WriteString("可用示例：{{thinking}}、{{idle}}、{{tap}}、{{smile}}、{{soft}}、{{focus}}、{{motion:thinking}}、{{expression:smile}}。\n")
	sb.WriteString("这些标识只用于驱动 Live2D 的动作或表情，不属于要读出来的正文；每次回答最多使用 1 到 3 个，不要连续堆叠。")
	// 画像放在 system 末尾：它只在被整理出新内容时才变化，前面的人设与规则保持不变，有利于提示词前缀缓存
	if profile := strings.TrimSpace(req.Profile); profile != "" {
		sb.WriteString("\n\n已了解的信息（自然地用，不要逐条复述，也不要说“我记得你说过”）：\n")
		sb.WriteString(profile)
	} else {
		sb.WriteString("\n\n已了解的信息：目前还不了解用户的任何个人信息。")
	}
	return sb.String()
}

// buildUserPrompt 是本轮用户消息：检索到的参考内容在前，用户这句话在后
func buildUserPrompt(req Request) string {
	var sb strings.Builder
	if len(req.Context) > 0 {
		sb.WriteString("以下是可参考的检索上下文（相关时再用，不要提及它的来源）：\n")
		for i, c := range req.Context {
			sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, c))
		}
		sb.WriteString("---\n")
	}
	sb.WriteString(req.Prompt)
	return sb.String()
}

// buildMessages 按聊天接口的角色拼装：人设与规则放 system，历史按轮次交替，最后是本轮用户消息
func buildMessages(req Request) []openAIMessage {
	msgs := []openAIMessage{{Role: "system", Content: buildSystemPrompt(req)}}
	for _, t := range req.History {
		if strings.TrimSpace(t.User) == "" || strings.TrimSpace(t.Assistant) == "" {
			continue
		}
		msgs = append(msgs,
			openAIMessage{Role: "user", Content: t.User},
			openAIMessage{Role: "assistant", Content: t.Assistant})
	}
	return append(msgs, openAIMessage{Role: "user", Content: buildUserPrompt(req)})
}
