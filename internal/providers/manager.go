package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Manager struct {
	root string
}

var KnownKinds = []string{"wake", "asr", "tts", "embedding", "llm"}

type Manifest struct {
	Version   int              `json:"version"`
	Providers []ManifestRecord `json:"providers"`
}

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

func NewManager(root string) *Manager {
	root = strings.TrimSpace(root)
	if root == "" {
		root = filepath.ToSlash("runtime/providers")
	}
	return &Manager{root: filepath.Clean(root)}
}

func (m *Manager) Root() string {
	return m.root
}

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

func (m *Manager) FindFirst(kind string, candidates ...string) string {
	for _, candidate := range candidates {
		if p := cleanIfExists(filepath.Join(m.currentDir(kind), candidate)); p != "" {
			return p
		}
	}
	return ""
}

func (m *Manager) FindGlob(kind string, patterns ...string) string {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(m.currentDir(kind), pattern))
		for _, match := range matches {
			if p := cleanIfExists(match); p != "" {
				return p
			}
		}
	}
	return ""
}

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

func (m *Manager) currentDir(kind string) string {
	return filepath.Join(m.root, kind, "current")
}

func (m *Manager) providerDir(kind, version string) string {
	kind = strings.TrimSpace(kind)
	version = strings.TrimSpace(version)
	if version == "" {
		version = "current"
	}
	return filepath.Join(m.root, kind, version)
}

func (m *Manager) ManifestPath() string {
	return filepath.Join(m.root, "manifest.json")
}

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
