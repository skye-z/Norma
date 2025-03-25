package providers

import "strings"

type Health struct {
	Status   string            `json:"status"`
	Required []string          `json:"required"`
	Found    []string          `json:"found"`
	Missing  []string          `json:"missing"`
	Paths    map[string]string `json:"paths,omitempty"`
}

func BuildHealth(mgr *Manager) map[string]Health {
	return map[string]Health{
		"wake": classifyHealth([]string{"encoder", "decoder", "joiner", "tokens", "keywords"}, map[string]string{
			"encoder":  mgr.FindGlob("wake", "encoder-epoch-*.onnx", "encoder.int8.onnx", "encoder.onnx"),
			"decoder":  mgr.FindGlob("wake", "decoder-epoch-*.onnx", "decoder.int8.onnx", "decoder.onnx"),
			"joiner":   mgr.FindGlob("wake", "joiner-epoch-*.onnx", "joiner.int8.onnx", "joiner.onnx"),
			"tokens":   mgr.FindFirst("wake", "tokens.txt"),
			"keywords": mgr.FindFirst("wake", "keywords.txt", "test_keywords.txt"),
		}),
		"asr": classifyHealth([]string{"model"}, map[string]string{
			"model": mgr.FindFirst("asr", "ggml-base.bin", "ggml-small.bin", "ggml-medium.bin", "ggml-large-v3.bin"),
		}),
		"tts": classifyHealth([]string{"binary", "voice_model", "voice_config"}, map[string]string{
			"binary":       mgr.FindFirst("tts", "piper.exe", "piper"),
			"voice_model":  mgr.FindGlob("tts", "*.onnx"),
			"voice_config": mgr.FindGlob("tts", "*.onnx.json"),
		}),
		"llm": classifyHealth([]string{"binary"}, map[string]string{
			"binary": mgr.FindFirst("llm", "ollama.exe", "ollama"),
		}),
		"embedding": classifyHealth([]string{"ollama"}, map[string]string{
			"ollama": mgr.FindFirst("llm", "ollama.exe", "ollama"),
		}),
	}
}

func classifyHealth(required []string, foundMap map[string]string) Health {
	found := make([]string, 0, len(required))
	missing := make([]string, 0, len(required))
	for _, key := range required {
		if strings.TrimSpace(foundMap[key]) != "" {
			found = append(found, key)
			continue
		}
		missing = append(missing, key)
	}
	status := "missing"
	if len(found) == len(required) {
		status = "ready"
	} else if len(found) > 0 {
		status = "partial"
	}
	paths := make(map[string]string, len(foundMap))
	for key, value := range foundMap {
		if strings.TrimSpace(value) != "" {
			paths[key] = value
		}
	}
	return Health{Status: status, Required: required, Found: found, Missing: missing, Paths: paths}
}
