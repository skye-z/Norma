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

type OllamaStreamer struct {
	endpoint string
	model    string
	apiKey   string
	client   *http.Client
}

type openAIChatRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatChunk struct {
	Choices []openAIChatChoice `json:"choices"`
	Error   *openAIError       `json:"error"`
}

type openAIChatChoice struct {
	Delta        openAIChatDelta `json:"delta"`
	FinishReason string          `json:"finish_reason"`
}

type openAIChatDelta struct {
	Content string `json:"content"`
}

type openAIError struct {
	Message string `json:"message"`
}

type openAIModelsResponse struct {
	Data []openAIModel `json:"data"`
}

type openAIModel struct {
	ID string `json:"id"`
}

type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name string `json:"name"`
}

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

func PingOllama(endpoint, apiKey string, timeout time.Duration) error {
	_, err := listModels(endpoint, apiKey, timeout)
	return err
}

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

func (o *OllamaStreamer) Stream(ctx context.Context, req Request, onToken func(string)) error {
	prompt := buildPrompt(req)
	body, err := json.Marshal(openAIChatRequest{Model: o.model, Messages: []openAIMessage{{Role: "user", Content: prompt}}, Stream: true})
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

func buildPrompt(req Request) string {
	persona := strings.TrimSpace(req.Persona)
	if persona == "" {
		persona = config.DefaultLLMPersona
	}
	var sb strings.Builder
	sb.WriteString(persona)
	sb.WriteString("\n\u8bf7\u81ea\u7136\u3001\u53e3\u8bed\u5316\u5730\u56de\u7b54\uff0c\u63a7\u5236\u5728 2-4 \u53e5\uff0c\u4f18\u5148\u76f4\u63a5\u56de\u5e94\u7528\u6237\uff0c\u4e0d\u8981\u5199\u6210\u6d41\u7a0b\u8bf4\u660e\uff0c\u4e0d\u8981 OOC\u3002\n")
	sb.WriteString("\u4f60\u53ef\u4ee5\u5728\u56de\u7b54\u4e2d\u5c11\u91cf\u63d2\u5165 Live2D \u63a7\u5236\u6807\u8bc6\uff0c\u683c\u5f0f\u662f {{\u52a8\u4f5c}} \u6216 {{\u7c7b\u578b:\u52a8\u4f5c}}\u3002\n")
	sb.WriteString("\u53ef\u7528\u793a\u4f8b\uff1a{{thinking}}\u3001{{idle}}\u3001{{tap}}\u3001{{smile}}\u3001{{soft}}\u3001{{focus}}\u3001{{motion:thinking}}\u3001{{expression:smile}}\u3002\n")
	sb.WriteString("\u8fd9\u4e9b\u6807\u8bc6\u53ea\u7528\u4e8e\u9a71\u52a8 Live2D \u7684\u52a8\u4f5c\u6216\u8868\u60c5\uff0c\u4e0d\u5c5e\u4e8e\u8981\u8bfb\u51fa\u6765\u7684\u6b63\u6587\uff1b\u6bcf\u6b21\u56de\u7b54\u6700\u591a\u4f7f\u7528 1 \u5230 3 \u4e2a\uff0c\u4e0d\u8981\u8fde\u7eed\u5806\u53e0\u3002\n")
	if len(req.Context) > 0 {
		sb.WriteString("\u4ee5\u4e0b\u662f\u53ef\u53c2\u8003\u7684\u68c0\u7d22\u4e0a\u4e0b\u6587\uff1a\n")
		for i, c := range req.Context {
			sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, c))
		}
		sb.WriteString("---\n")
	}
	sb.WriteString(req.Prompt)
	return sb.String()
}
