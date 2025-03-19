package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"norma/internal/config"
	"norma/internal/errs"
)

type ModelHandler struct {
	configPath string
	modelDir   string
}

func NewModelHandler(configPath, modelDir string) *ModelHandler {
	return &ModelHandler{configPath: configPath, modelDir: modelDir}
}

func (h *ModelHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/runtime/model/resolve", h.handleResolve)
}

func (h *ModelHandler) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}

	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, errs.Internal("config_load_failed", err.Error()))
		return
	}

	resolvedPath, modelName, err := resolveLive2DModelPath(h.modelDir, cfg.App.ModelName)
	if err != nil {
		writeAPIError(w, errs.Invalid(err.Error()))
		return
	}
	corePath := findCoreForModel(resolvedPath)

	rel, err := filepath.Rel(h.modelDir, resolvedPath)
	if err != nil {
		writeAPIError(w, errs.Internal("model_resolve_failed", err.Error()))
		return
	}
	urlPath := "/static/model/" + filepath.ToSlash(rel)

	out := map[string]any{
		"ok":         true,
		"model_name": modelName,
		"model_path": filepath.ToSlash(resolvedPath),
		"model_url":  urlPath,
	}
	if corePath != "" {
		if coreRel, relErr := filepath.Rel(h.modelDir, corePath); relErr == nil {
			out["core_url"] = "/static/model/" + filepath.ToSlash(coreRel)
			out["core_path"] = filepath.ToSlash(corePath)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func resolveLive2DModelPath(rootDir, configuredModelName string) (string, string, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return "", "", errs.Invalid("model dir is empty")
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", "", err
	}

	modelDir := ""
	modelName := strings.TrimSpace(configuredModelName)
	if modelName != "" {
		candidate := filepath.Join(absRoot, modelName)
		info, statErr := os.Stat(candidate)
		if statErr != nil || !info.IsDir() {
			return "", "", errs.Invalid("configured app.model_name not found: " + modelName)
		}
		modelDir = candidate
	} else {
		entries, readErr := os.ReadDir(absRoot)
		if readErr != nil {
			return "", "", readErr
		}
		for _, entry := range entries {
			if entry.IsDir() {
				modelName = entry.Name()
				modelDir = filepath.Join(absRoot, modelName)
				break
			}
		}
		if modelDir == "" {
			return "", "", errs.Invalid("no model folder found under resource/model")
		}
	}

	modelPath, findErr := findFirstModelFile(modelDir)
	if findErr != nil {
		return "", "", findErr
	}
	return modelPath, modelName, nil
}

func findFirstModelFile(dir string) (string, error) {
	var first string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(d.Name())
		if strings.HasSuffix(lower, ".model3.json") || strings.HasSuffix(lower, ".can3") || strings.HasSuffix(lower, ".cmo3") {
			first = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if first == "" {
		return "", errs.Invalid("no live2d model file found in selected model folder")
	}
	return first, nil
}

func findCoreForModel(modelFilePath string) string {
	if strings.TrimSpace(modelFilePath) == "" {
		return ""
	}
	current := filepath.Dir(modelFilePath)
	for {
		candidate := filepath.Join(current, "live2dcubismcore.min.js")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		next := filepath.Dir(current)
		if next == current {
			break
		}
		current = next
	}
	return ""
}
