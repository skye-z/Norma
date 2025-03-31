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
)

type OllamaStreamer struct {
	endpoint string
	model    string
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

func NewOllamaStreamer(endpoint, model string, timeout time.Duration) *OllamaStreamer {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = "http://127.0.0.1:11434/v1"
	}
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	return &OllamaStreamer{
		endpoint: normalizeOpenAIEndpoint(endpoint),
		model:    model,
		client:   &http.Client{Timeout: timeout},
	}
}

func PingOllama(endpoint string, timeout time.Duration) error {
	_, err := listModels(endpoint, timeout)
	return err
}

func DiscoverFirstLLMModel(endpoint string, timeout time.Duration) (string, error) {
	models, err := listModels(endpoint, timeout)
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

func listModels(endpoint string, timeout time.Duration) ([]ollamaModel, error) {
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
	var sb strings.Builder
	sb.WriteString("你是 Norma，一个中文桌面 AI 助手。回答要求：先给结论，再给 2-4 句自然解释；语气像靠谱同伴，不要官腔，不要空话；尽量口语化、简洁、有节奏。必要时用'先说结论'、'具体来说'、'如果你现在就做'这类过渡。\n")
	if len(req.Context) > 0 {
		sb.WriteString("你可参考以下本地检索上下文回答。\n")
		for i, c := range req.Context {
			sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, c))
		}
		sb.WriteString("---\n")
	}
	sb.WriteString(req.Prompt)
	return sb.String()
}
