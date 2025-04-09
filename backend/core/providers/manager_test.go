package providers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerResolvesManagedFiles(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "asr", "current")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	binary := filepath.Join(binDir, "whisper-cli.exe")
	if err := os.WriteFile(binary, []byte("test"), 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	mgr := NewManager(root)
	resolved := mgr.ResolveBinary("asr", "", "", "whisper-cli.exe")
	if resolved != binary {
		t.Fatalf("expected %s, got %s", binary, resolved)
	}
}

func TestManagerManifestRoundTrip(t *testing.T) {
	mgr := NewManager(t.TempDir())
	manifest := Manifest{Providers: []ManifestRecord{{Name: "whisper.cpp", Kind: "asr", Version: "1.7.6", Binary: "whisper-cli.exe", Installed: true}}}
	if err := mgr.SaveManifest(manifest); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
	loaded, err := mgr.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if loaded.Version != 1 {
		t.Fatalf("expected version 1, got %d", loaded.Version)
	}
	if len(loaded.Providers) != 1 || loaded.Providers[0].Name != "whisper.cpp" {
		t.Fatalf("unexpected providers: %+v", loaded.Providers)
	}
}

func TestEnsureWakeKeywordsFileCreatesDefaultKeyword(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"encoder.onnx", "decoder.onnx", "joiner.onnx", "tokens.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	path, err := EnsureWakeKeywordsFile(root, "")
	if err != nil {
		t.Fatalf("ensure wake keywords: %v", err)
	}
	if filepath.Base(path) != "keywords.txt" {
		t.Fatalf("unexpected keywords path: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read keywords: %v", err)
	}
	if string(data) != "n uò m ǎ @诺玛\n" {
		t.Fatalf("unexpected keywords content: %q", string(data))
	}
}
