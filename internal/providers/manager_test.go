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
