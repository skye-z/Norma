// Package providers 提供商管理器
package providers

import (
	"strings"

	asrAudio "norma/backend/core/audio/asr"
)

// Health 提供商健康状态
type Health struct {
	Status   string            `json:"status"`
	Required []string          `json:"required"`
	Found    []string          `json:"found"`
	Missing  []string          `json:"missing"`
	Paths    map[string]string `json:"paths,omitempty"`
}

// BuildHealth 构建健康检查状态
func BuildHealth(mgr *Manager) map[string]Health {
	return map[string]Health{
		"wake": classifyHealth([]string{"encoder", "decoder", "joiner", "tokens", "keywords"}, map[string]string{
			"encoder":  mgr.FindGlob("wake", "encoder-epoch-*.onnx", "encoder.int8.onnx", "encoder.onnx"),
			"decoder":  mgr.FindGlob("wake", "decoder-epoch-*.onnx", "decoder.int8.onnx", "decoder.onnx"),
			"joiner":   mgr.FindGlob("wake", "joiner-epoch-*.onnx", "joiner.int8.onnx", "joiner.onnx"),
			"tokens":   mgr.FindFirst("wake", "tokens.txt"),
			"keywords": mgr.FindFirst("wake", "keywords.txt", "test_keywords.txt"),
		}),
		"asr": classifyHealth(asrRequired(mgr), asrFound(mgr)),
		"tts": classifyHealth([]string{"model", "voices", "tokens", "data_dir", "lexicon"}, map[string]string{
			"model":    mgr.FindFirst("tts", "model.onnx"),
			"voices":   mgr.FindFirst("tts", "voices.bin"),
			"tokens":   mgr.FindFirst("tts", "tokens.txt"),
			"data_dir": mgr.FindGlob("tts", "espeak-ng-data"),
			"lexicon":  mgr.FindFirst("tts", "lexicon-us-en.txt", "lexicon-zh.txt"),
		}),
		"llm": classifyHealth([]string{"binary"}, map[string]string{
			"binary": mgr.FindFirst("llm", "ollama.exe", "ollama"),
		}),
		"embedding": classifyHealth([]string{"ollama"}, map[string]string{
			"ollama": mgr.FindFirst("llm", "ollama.exe", "ollama"),
		}),
	}
}

// asrRequired ASR 必需文件
func asrRequired(mgr *Manager) []string {
	_ = mgr
	return []string{"model", "runtime"}
}

// asrFound 查找 ASR 文件
func asrFound(mgr *Manager) map[string]string {
	model := mgr.FindFirst("asr", "ggml-base.bin", "ggml-small.bin", "ggml-medium.bin", "ggml-large-v3.bin")
	runtimePath := mgr.FindFirst("asr", "whisper-cli.exe", "main.exe", "whisper-cli", "main")
	if runtimePath == "" && strings.TrimSpace(model) != "" {
		if _, err := asrAudio.NewWhisperEmbeddedASR(model, "zh", 4); err == nil {
			runtimePath = "embedded:whisper"
		}
	}
	return map[string]string{
		"model":   model,
		"runtime": runtimePath,
	}
}

// classifyHealth 分类健康状态
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
