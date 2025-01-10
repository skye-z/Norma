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

type ollamaGenerateRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
    Stream bool   `json:"stream"`
}

type ollamaGenerateChunk struct {
    Response string `json:"response"`
    Done     bool   `json:"done"`
    Error    string `json:"error"`
}

func NewOllamaStreamer(endpoint, model string, timeout time.Duration) *OllamaStreamer {
    if strings.TrimSpace(endpoint) == "" {
        endpoint = "http://127.0.0.1:11434"
    }
    if timeout <= 0 {
        timeout = 45 * time.Second
    }
    return &OllamaStreamer{
        endpoint: strings.TrimRight(endpoint, "/"),
        model:    model,
        client:   &http.Client{Timeout: timeout},
    }
}

func PingOllama(endpoint string, timeout time.Duration) error {
    endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
    if endpoint == "" {
        endpoint = "http://127.0.0.1:11434"
    }
    if timeout <= 0 {
        timeout = 3 * time.Second
    }
    client := &http.Client{Timeout: timeout}
    req, err := http.NewRequest(http.MethodGet, endpoint+"/api/tags", nil)
    if err != nil {
        return err
    }
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        return fmt.Errorf("status=%d", resp.StatusCode)
    }
    return nil
}

func (o *OllamaStreamer) Stream(ctx context.Context, req Request, onToken func(string)) error {
    prompt := buildPrompt(req)
    body, err := json.Marshal(ollamaGenerateRequest{Model: o.model, Prompt: prompt, Stream: true})
    if err != nil {
        return fmt.Errorf("marshal ollama request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint+"/api/generate", bytes.NewReader(body))
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
        var chunk ollamaGenerateChunk
        if err := json.Unmarshal([]byte(line), &chunk); err != nil {
            return fmt.Errorf("decode ollama stream: %w", err)
        }
        if chunk.Error != "" {
            return fmt.Errorf("ollama stream error: %s", chunk.Error)
        }
        if chunk.Response != "" {
            onToken(chunk.Response)
        }
        if chunk.Done {
            return nil
        }
    }
    if err := scanner.Err(); err != nil {
        return fmt.Errorf("read ollama stream: %w", err)
    }
    return nil
}

func buildPrompt(req Request) string {
    var sb strings.Builder
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
