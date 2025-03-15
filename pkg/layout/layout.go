package layout

import (
	"os"
	"path/filepath"
)

const (
	ConfigFileName        = "config.yaml"
	ConfigExampleFileName = "config.example.yaml"
	DebugDir              = "web/debug"
	ResourceDir           = "resource"
	ModelDir              = "resource/model"
	DataDir               = "resource/data"
	LogsDir               = "resource/logs"
	DiagnosticsDir        = "resource/diagnostics"
	RuntimeDir            = "resource/runtime"
	ProvidersDir          = "resource/runtime/providers"
	DBPath                = "resource/data/norma.db"
	VectorIndexPath       = "resource/data/vector_index.json"
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

func HasRuntimeMarkers(root string) bool {
	markers := []string{
		ConfigFileName,
		ModelDir,
		DebugDir,
	}
	for _, marker := range markers {
		if _, err := os.Stat(Join(root, marker)); err != nil {
			return false
		}
	}
	return true
}
