package api

import (
	"encoding/binary"
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
	mux.HandleFunc("/runtime/model/list", h.handleList)
}

func (h *ModelHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, errs.Invalid("method not allowed"))
		return
	}
	items, err := listModelCandidates(h.modelDir)
	if err != nil {
		writeAPIError(w, errs.Internal("model_list_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"items": items,
	})
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
	if ok, detail := isModelCompatible(filepath.Join(h.modelDir, modelName)); !ok {
		writeAPIError(w, errs.Invalid("model not compatible with current Cubism Core: "+detail))
		return
	}
	corePath := findCoreForModel(filepath.Join(h.modelDir, modelName), resolvedPath)

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
	type candidate struct {
		path  string
		score int
	}
	best := candidate{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(d.Name())
		score := -1
		switch {
		case strings.HasSuffix(lower, ".model3.json"):
			score = 100
		case strings.HasSuffix(lower, ".can3"):
			score = 70
		case strings.HasSuffix(lower, ".cmo3"):
			score = 60
		default:
			return nil
		}
		pathLower := strings.ToLower(filepath.ToSlash(path))
		if strings.Contains(pathLower, "/runtime/") {
			score += 20
		}
		if score > best.score {
			best = candidate{path: path, score: score}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if best.path == "" {
		return "", errs.Invalid("no live2d model file found in selected model folder")
	}
	return best.path, nil
}

func findCoreForModel(modelRootDir, modelFilePath string) string {
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

	root := strings.TrimSpace(modelRootDir)
	if root != "" {
		var found string
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if strings.EqualFold(d.Name(), "live2dcubismcore.min.js") {
				found = path
				return filepath.SkipAll
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	return ""
}

func listModelCandidates(rootDir string) ([]map[string]any, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return nil, errs.Invalid("model dir is empty")
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(absRoot)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		modelDir := filepath.Join(absRoot, name)
		ok, detail := isModelCompatible(modelDir)
		if !ok {
			out = append(out, map[string]any{
				"name":       name,
				"label":      name + " (不兼容)",
				"model_url":  "",
				"compatible": false,
				"reason":     detail,
			})
			continue
		}
		modelPath, findErr := findFirstModelFile(modelDir)
		if findErr != nil {
			continue
		}
		rel, relErr := filepath.Rel(absRoot, modelPath)
		if relErr != nil {
			continue
		}
		out = append(out, map[string]any{
			"name":       name,
			"label":      name,
			"model_url":  "/static/model/" + filepath.ToSlash(rel),
			"compatible": true,
		})
	}
	return out, nil
}

func isModelCompatible(modelDir string) (bool, string) {
	maxVer, hasMoc3 := maxMoc3Version(modelDir)
	if !hasMoc3 {
		return true, ""
	}
	if maxVer > 5 {
		return false, "moc3 版本为 " + itoa(maxVer) + "，当前 Core 仅支持到 5"
	}
	return true, ""
}

func maxMoc3Version(dir string) (int, bool) {
	maxVer := 0
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".moc3") {
			return nil
		}
		ver, ok := readMoc3Version(path)
		if !ok {
			return nil
		}
		found = true
		if ver > maxVer {
			maxVer = ver
		}
		return nil
	})
	return maxVer, found
}

func readMoc3Version(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil || len(b) < 8 {
		return 0, false
	}
	if string(b[:4]) != "MOC3" {
		return 0, false
	}
	ver := int(binary.LittleEndian.Uint32(b[4:8]))
	if ver <= 0 {
		return 0, false
	}
	return ver, true
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	buf := make([]byte, 0, 12)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
