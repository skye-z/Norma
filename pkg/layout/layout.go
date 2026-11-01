package layout

import (
	"os"
	"path/filepath"
)

const (
	ConfigFileName        = "config.yaml"
	ConfigExampleFileName = "config.example.yaml"
	DebugSourceDir        = "frontend/public/console"
	DebugDistDir          = "frontend/dist/console"
	DebugReleaseDir       = "console"
	ResourceDir           = "resource"
	ModelDir              = "resource/model"
	DataDir               = "resource/data"
	LogsDir               = "resource/logs"
	DiagnosticsDir        = "resource/diagnostics"
	RuntimeDir            = "resource/runtime"
	ProvidersDir          = "resource/runtime/providers"
	DBPath                = "resource/data/norma.db"
	VectorIndexPath       = "resource/data/vector_index.json"
	MemoryPath            = "resource/data/memory.md"
	PurgeProofDir         = "resource/diagnostics/purge-proofs"
	StartupReportPath     = "resource/logs/startup_report.json"
)

func Join(root, rel string) string {
	if root == "" {
		return filepath.Clean(rel)
	}
	return filepath.Join(root, filepath.FromSlash(rel))
}

func EnsureRuntimeLayout(root string) error {
	dirs := []string{
		ResourceDir,
		ModelDir,
		DataDir,
		LogsDir,
		DiagnosticsDir,
		RuntimeDir,
		ProvidersDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(Join(root, dir), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func ResolveDebugDir(root string) string {
	candidates := []string{DebugReleaseDir, DebugDistDir, DebugSourceDir}
	for _, rel := range candidates {
		full := Join(root, rel)
		if info, err := os.Stat(full); err == nil && info.IsDir() {
			return full
		}
	}
	return Join(root, DebugSourceDir)
}

func HasRuntimeMarkers(root string) bool {
	markers := []string{
		ConfigFileName,
		ModelDir,
	}
	for _, marker := range markers {
		if _, err := os.Stat(Join(root, marker)); err != nil {
			return false
		}
	}
	return true
}
