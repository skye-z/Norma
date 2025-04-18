// Package providers 提供商管理器
package providers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"norma/pkg/layout"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Manager 提供商管理器
type Manager struct {
	root string
}

// KnownKinds 已知的提供商类型
var KnownKinds = []string{"wake", "asr", "tts", "embedding", "llm"}

// Manifest 提供商清单
type Manifest struct {
	Version   int              `json:"version"`
	Providers []ManifestRecord `json:"providers"`
}

// ManifestRecord 提供商记录
type ManifestRecord struct {
	ID               string `json:"id,omitempty"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	Kind             string `json:"kind"`
	Binary           string `json:"binary"`
	Model            string `json:"model"`
	Endpoint         string `json:"endpoint"`
	Installed        bool   `json:"installed"`
	Integrity        string `json:"integrity,omitempty"`
	IntegrityNote    string `json:"integrity_note,omitempty"`
	Source           string `json:"source,omitempty"`
	LocalSHA256      string `json:"local_sha256,omitempty"`
	ExtraLocalSHA256 string `json:"extra_local_sha256,omitempty"`
	InstalledAt      string `json:"installed_at,omitempty"`
}

// NewManager 创建提供商管理器
func NewManager(root string) *Manager {
	root = strings.TrimSpace(root)
	if root == "" {
		root = filepath.ToSlash(layout.ProvidersDir)
	}
	return &Manager{root: filepath.Clean(root)}
}

// Root 获取根目录
func (m *Manager) Root() string {
	return m.root
}

// EnsureLayout 确保目录结构
func (m *Manager) EnsureLayout() error {
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return err
	}
	for _, dir := range KnownKinds {
		if err := os.MkdirAll(filepath.Join(m.root, dir), 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(m.root, dir, "current"), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// EnsureWakeKeywords 确保唤醒词文件
func (m *Manager) EnsureWakeKeywords(keyword string) (string, error) {
	return EnsureWakeKeywordsFile(m.currentDir("wake"), keyword)
}

// ResolveBinary 解析二进制文件路径
func (m *Manager) ResolveBinary(kind, version, explicit string, candidates ...string) string {
	if p := cleanIfExists(explicit); p != "" {
		return p
	}
	for _, candidate := range candidates {
		if p := cleanIfExists(filepath.Join(m.providerDir(kind, version), candidate)); p != "" {
			return p
		}
		if p := cleanIfExists(filepath.Join(m.currentDir(kind), candidate)); p != "" {
			return p
		}
	}
	return ""
}

// ResolveFile 解析文件路径
func (m *Manager) ResolveFile(kind, version, explicit string, candidates ...string) string {
	if p := cleanIfExists(explicit); p != "" {
		return p
	}
	for _, candidate := range candidates {
		if p := cleanIfExists(filepath.Join(m.providerDir(kind, version), candidate)); p != "" {
			return p
		}
		if p := cleanIfExists(filepath.Join(m.currentDir(kind), candidate)); p != "" {
			return p
		}
	}
	return ""
}

// FindFirst 查找第一个匹配的文件
func (m *Manager) FindFirst(kind string, candidates ...string) string {
	root := m.currentDir(kind)
	for _, candidate := range candidates {
		if p := cleanIfExists(filepath.Join(root, candidate)); p != "" {
			return p
		}
		if p := findRecursive(root, func(path string) bool {
			return strings.EqualFold(filepath.Base(path), candidate)
		}); p != "" {
			return p
		}
	}
	return ""
}

// FindGlob 查找匹配 glob 模式的所有文件
func (m *Manager) FindGlob(kind string, patterns ...string) string {
	root := m.currentDir(kind)
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(root, pattern))
		for _, match := range matches {
			if p := cleanIfExists(match); p != "" {
				return p
			}
		}
		if p := findRecursive(root, func(path string) bool {
			ok, _ := filepath.Match(pattern, filepath.Base(path))
			return ok
		}); p != "" {
			return p
		}
	}
	return ""
}

// CurrentFiles 获取当前版本的所有文件
func (m *Manager) CurrentFiles(kind string) ([]string, error) {
	entries, err := os.ReadDir(m.currentDir(kind))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Name())
	}
	return out, nil
}

// currentDir 获取当前版本目录
func (m *Manager) currentDir(kind string) string {
	return filepath.Join(m.root, kind, "current")
}

// providerDir 获取指定版本目录
func (m *Manager) providerDir(kind, version string) string {
	kind = strings.TrimSpace(kind)
	version = strings.TrimSpace(version)
	if version == "" {
		version = "current"
	}
	return filepath.Join(m.root, kind, version)
}

// ManifestPath 获取清单文件路径
func (m *Manager) ManifestPath() string {
	return filepath.Join(m.root, "manifest.json")
}

// LoadManifest 加载清单
func (m *Manager) LoadManifest() (Manifest, error) {
	var out Manifest
	path := m.ManifestPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{Version: 1}, nil
		}
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, fmt.Errorf("decode provider manifest: %w", err)
	}
	if out.Version == 0 {
		out.Version = 1
	}
	return out, nil
}

// SaveManifest 保存清单
func (m *Manager) SaveManifest(manifest Manifest) error {
	if manifest.Version == 0 {
		manifest.Version = 1
	}
	if err := m.EnsureLayout(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode provider manifest: %w", err)
	}
	return os.WriteFile(m.ManifestPath(), data, 0o644)
}

// cleanIfExists 检查路径是否存在并清理
func cleanIfExists(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return filepath.Clean(path)
}

// findRecursive 递归查找文件
func findRecursive(root string, match func(path string) bool) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	var found string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || found != "" {
			return nil
		}
		if match(path) {
			found = cleanIfExists(path)
		}
		return nil
	})
	return found
}

// EnsureWakeKeywordsFile 确保唤醒词文件存在
func EnsureWakeKeywordsFile(modelDir, keyword string) (string, error) {
	modelDir = strings.TrimSpace(modelDir)
	if modelDir == "" {
		return "", nil
	}
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		return "", err
	}

	required := []string{
		firstExistingInDir(modelDir, "encoder-epoch-*.onnx", "encoder.int8.onnx", "encoder.onnx"),
		firstExistingInDir(modelDir, "decoder-epoch-*.onnx", "decoder.int8.onnx", "decoder.onnx"),
		firstExistingInDir(modelDir, "joiner-epoch-*.onnx", "joiner.int8.onnx", "joiner.onnx"),
		cleanIfExists(filepath.Join(modelDir, "tokens.txt")),
	}
	for _, path := range required {
		if strings.TrimSpace(path) == "" {
			return "", nil
		}
	}

	formatted, err := FormatWakeKeyword(modelDir, keyword)
	if err != nil {
		return "", err
	}
	keywordsPath := filepath.Join(modelDir, "keywords.txt")
	if err := os.WriteFile(keywordsPath, []byte(formatted), 0o644); err != nil {
		return "", err
	}
	return filepath.Clean(keywordsPath), nil
}

// FormatWakeKeyword 格式化唤醒词
func FormatWakeKeyword(modelDir, keyword string) (string, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		keyword = "诺玛"
	}
	if mapped := builtinWakeKeywordTokens(keyword); mapped != "" {
		return mapped + "\n", nil
	}
	if !isASCIIWakeKeyword(keyword) {
		return "", fmt.Errorf("wake keyword %q is not supported by the current phone+ppinyin model; use 诺玛 or an English keyword from en.phone", keyword)
	}
	phones, err := lookupWakeKeywordPhones(filepath.Join(modelDir, "en.phone"), keyword)
	if err != nil {
		return "", err
	}
	return phones + " @" + strings.ToUpper(keyword) + "\n", nil
}

// builtinWakeKeywordTokens 内置唤醒词 tokens
func builtinWakeKeywordTokens(keyword string) string {
	switch strings.TrimSpace(keyword) {
	case "诺玛":
		return "n uò m ǎ @诺玛"
	default:
		return ""
	}
}

// isASCIIWakeKeyword 检查是否为 ASCII 唤醒词
func isASCIIWakeKeyword(keyword string) bool {
	for _, r := range keyword {
		if r > unicode.MaxASCII {
			return false
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == '-' || r == '_' || r == '\'' {
			continue
		}
		return false
	}
	return true
}

// lookupWakeKeywordPhones 查找唤醒词对应的音素
func lookupWakeKeywordPhones(dictPath, keyword string) (string, error) {
	file, err := os.Open(dictPath)
	if err != nil {
		return "", fmt.Errorf("open wake lexicon: %w", err)
	}
	defer file.Close()

	target := strings.ToUpper(strings.Join(strings.Fields(keyword), " "))
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		if strings.ToUpper(parts[0]) == target {
			return strings.Join(parts[1:], " "), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read wake lexicon: %w", err)
	}
	return "", fmt.Errorf("wake keyword %q not found in %s", keyword, filepath.Base(dictPath))
}

// firstExistingInDir 查找目录中第一个存在的文件
func firstExistingInDir(root string, patterns ...string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(root, pattern))
		for _, match := range matches {
			if p := cleanIfExists(match); p != "" {
				return p
			}
		}
	}
	return ""
}
