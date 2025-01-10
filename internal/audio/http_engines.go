package audio

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

// HTTPASR calls a local ASR HTTP service.
type HTTPASR struct {
    Endpoint string
    Client   *http.Client
}

type asrRequest struct {
    SessionID string `json:"session_id"`
}

type asrResponse struct {
    Text string `json:"text"`
}

func NewHTTPASR(endpoint string, timeout time.Duration) *HTTPASR {
    if timeout <= 0 {
        timeout = 10 * time.Second
    }
    return &HTTPASR{
        Endpoint: strings.TrimRight(endpoint, "/"),
        Client:   &http.Client{Timeout: timeout},
    }
}

func (a *HTTPASR) Stream(ctx context.Context, onPartial func(string), onFinal func(string)) error {
    if a.Endpoint == "" {
        return fmt.Errorf("asr endpoint is empty")
    }

    body, _ := json.Marshal(asrRequest{SessionID: "default"})
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.Endpoint+"/transcribe", bytes.NewReader(body))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := a.Client.Do(req)
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

    onPartial("正在识别...")
    onFinal(strings.TrimSpace(out.Text))
    return nil
}

// HTTPTTS calls a local XTTS-like HTTP service.
type HTTPTTS struct {
    Endpoint string
    Voice    string
    Client   *http.Client
}

type ttsRequest struct {
    Text  string `json:"text"`
    Voice string `json:"voice"`
}

func NewHTTPTTS(endpoint, voice string, timeout time.Duration) *HTTPTTS {
    if timeout <= 0 {
        timeout = 30 * time.Second
    }
    return &HTTPTTS{
        Endpoint: strings.TrimRight(endpoint, "/"),
        Voice:    voice,
        Client:   &http.Client{Timeout: timeout},
    }
}

func (t *HTTPTTS) Stream(ctx context.Context, text string, onChunk func(chunk []byte)) error {
    if t.Endpoint == "" {
        return fmt.Errorf("tts endpoint is empty")
    }

    body, _ := json.Marshal(ttsRequest{Text: text, Voice: t.Voice})
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.Endpoint+"/tts", bytes.NewReader(body))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := t.Client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 300 {
        data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
        return fmt.Errorf("tts status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
    }

    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }

    if len(data) == 0 {
        return fmt.Errorf("tts returned empty audio")
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
            onChunk(data[i:end])
        }
    }

    return nil
}

func (t *HTTPTTS) Stop(sessionID string) error {
    _ = sessionID
    return nil
}
