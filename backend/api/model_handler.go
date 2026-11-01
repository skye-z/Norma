// Package api 提供 HTTP API
package api

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"norma/backend/utils"
)

// 模型处理器
type ModelHandler struct {
	configPath string
	modelDir   string
}

// 创建模型处理器
func NewModelHandler(configPath, modelDir string) *ModelHandler {
	return &ModelHandler{configPath: configPath, modelDir: modelDir}
}

// 注册路由
func (h *ModelHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/runtime/model/resolve", h.handleResolve)
	mux.HandleFunc("/runtime/model/list", h.handleList)
	mux.HandleFunc("/runtime/model/pose-groups", h.handlePoseGroups)
}

// handlePoseGroups 获取姿势组
func (h *ModelHandler) handlePoseGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, config.Invalid("method not allowed"))
		return
	}
	modelURL := strings.TrimSpace(r.URL.Query().Get("model_url"))
	if modelURL == "" {
		writeAPIError(w, config.Invalid("model_url required"))
		return
	}

	absPath, err := modelURLToAbsPath(h.modelDir, modelURL)
	if err != nil {
		writeAPIError(w, config.Invalid(err.Error()))
		return
	}
	poseFile, groups, err := loadPoseGroups(absPath)
	if err != nil {
		writeAPIError(w, config.Invalid(err.Error()))
		return
	}
	rel, relErr := filepath.Rel(h.modelDir, poseFile)
	poseURL := ""
	if relErr == nil {
		poseURL = "/static/model/" + filepath.ToSlash(rel)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"pose_file":   filepath.ToSlash(poseFile),
		"pose_url":    poseURL,
		"pose_groups": groups,
	})
}

// handleList 获取模型列表
func (h *ModelHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, config.Invalid("method not allowed"))
		return
	}
	items, err := listModelCandidates(h.modelDir)
	if err != nil {
		writeAPIError(w, config.Internal("model_list_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"items": items,
	})
}

// handleResolve 解析模型路径
func (h *ModelHandler) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, config.Invalid("method not allowed"))
		return
	}

	cfg, err := config.Load(h.configPath)
	if err != nil {
		writeAPIError(w, config.Internal("config_load_failed", err.Error()))
		return
	}

	resolvedPath, modelName, err := resolveLive2DModelPath(h.modelDir, cfg.App.ModelName)
	if err != nil {
		writeAPIError(w, config.Invalid(err.Error()))
		return
	}
	if ok, detail := isModelCompatible(filepath.Join(h.modelDir, modelName)); !ok {
		writeAPIError(w, config.Invalid("model not compatible with current Cubism Core: "+detail))
		return
	}

	rel, err := filepath.Rel(h.modelDir, resolvedPath)
	if err != nil {
		writeAPIError(w, config.Internal("model_resolve_failed", err.Error()))
		return
	}
	urlPath := "/static/model/" + filepath.ToSlash(rel)

	out := map[string]any{
		"ok":         true,
		"model_name": modelName,
		"model_path": filepath.ToSlash(resolvedPath),
		"model_url":  urlPath,
		"mapping":    loadModelMapping(filepath.Dir(resolvedPath)),
	}
	writeJSON(w, http.StatusOK, out)
}

// loadModelMapping 加载模型映射配置
func loadModelMapping(modelDir string) map[string]any {
	readJSON := func(path string) map[string]any {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			return nil
		}
		return out
	}
	combined := readJSON(filepath.Join(modelDir, "live2d.map.json"))
	motions := map[string]any{}
	expressions := map[string]any{}
	if combined != nil {
		if part, ok := combined["motions"].(map[string]any); ok {
			motions = part
		}
		if part, ok := combined["expressions"].(map[string]any); ok {
			expressions = part
		}
	}
	if len(motions) == 0 {
		if part := readJSON(filepath.Join(modelDir, "motion_map.json")); part != nil {
			motions = part
		}
	}
	if len(expressions) == 0 {
		if part := readJSON(filepath.Join(modelDir, "expression_map.json")); part != nil {
			expressions = part
		}
	}
	return map[string]any{"motions": motions, "expressions": expressions}
}

// resolveLive2DModelPath 解析 Live2D 模型路径
func resolveLive2DModelPath(rootDir, configuredModelName string) (string, string, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return "", "", config.Invalid("model dir is empty")
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", "", err
	}

	modelDir := ""
	modelName := strings.TrimSpace(configuredModelName)
	if strings.EqualFold(modelName, "default") {
		modelName = "" // 历史默认值，等同于未配置
	}
	if modelName != "" {
		candidate := filepath.Join(absRoot, modelName)
		info, statErr := os.Stat(candidate)
		if statErr != nil || !info.IsDir() {
			return "", "", config.Invalid("configured app.model_name not found: " + modelName)
		}
		modelDir = candidate
	} else {
		entries, readErr := os.ReadDir(absRoot)
		if readErr != nil {
			return "", "", readErr
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			candidateName := entry.Name()
			candidateDir := filepath.Join(absRoot, candidateName)
			if _, findErr := findFirstModelFile(candidateDir); findErr != nil {
				continue
			}
			modelName = candidateName
			modelDir = candidateDir
			break
		}
		if modelDir == "" {
			return "", "", config.Invalid("no model folder found under resource/model")
		}
	}

	modelPath, findErr := findFirstModelFile(modelDir)
	if findErr != nil {
		return "", "", findErr
	}
	return modelPath, modelName, nil
}

// findFirstModelFile 查找第一个模型文件
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
		return "", config.Invalid("no live2d model file found in selected model folder")
	}
	return best.path, nil
}

// findCoreForModel 查找模型对应的 Cubism Core 文件
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

// listModelCandidates 列出可用模型
func listModelCandidates(rootDir string) ([]map[string]any, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return nil, config.Invalid("model dir is empty")
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
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		name := entry.Name()
		modelDir := filepath.Join(absRoot, name)
		modelPath, findErr := findFirstModelFile(modelDir)
		if findErr != nil {
			continue
		}
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

// isModelCompatible 检查模型兼容性
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

// maxMoc3Version 获取最大 moc3 版本号
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

// readMoc3Version 读取 moc3 文件版本号
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

// itoa 整数转字符串
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

// modelURLToAbsPath 将模型 URL 转换为绝对路径
func modelURLToAbsPath(modelRootDir, modelURL string) (string, error) {
	marker := "/static/model/"
	idx := strings.Index(modelURL, marker)
	if idx < 0 {
		return "", config.Invalid("invalid model_url")
	}
	rel := strings.TrimPrefix(modelURL[idx+len(marker):], "/")
	rel = filepath.FromSlash(rel)
	absRoot, err := filepath.Abs(modelRootDir)
	if err != nil {
		return "", err
	}
	abs := filepath.Clean(filepath.Join(absRoot, rel))
	if !strings.HasPrefix(strings.ToLower(abs), strings.ToLower(absRoot)) {
		return "", config.Invalid("model_url out of model root")
	}
	if _, err := os.Stat(abs); err != nil {
		return "", config.Invalid("model file not found")
	}
	return abs, nil
}

// loadPoseGroups 加载姿势组配置
func loadPoseGroups(modelFilePath string) (string, []map[string]any, error) {
	modelDir := filepath.Dir(modelFilePath)
	base := strings.TrimSuffix(filepath.Base(modelFilePath), filepath.Ext(modelFilePath))
	candidates := []string{
		filepath.Join(modelDir, base+".pose3.json"),
	}
	_ = filepath.WalkDir(modelDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".pose3.json") {
			candidates = append(candidates, path)
		}
		return nil
	})

	seen := map[string]struct{}{}
	for _, path := range candidates {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc struct {
			Groups [][]struct {
				ID string `json:"Id"`
			} `json:"Groups"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			continue
		}
		if len(doc.Groups) == 0 {
			continue
		}
		out := make([]map[string]any, 0, len(doc.Groups))
		for i, group := range doc.Groups {
			parts := make([]string, 0, len(group))
			for _, item := range group {
				if strings.TrimSpace(item.ID) != "" {
					parts = append(parts, item.ID)
				}
			}
			out = append(out, map[string]any{
				"value": "group_" + itoa(i),
				"label": "姿势组 " + itoa(i+1),
				"parts": parts,
			})
		}
		return path, out, nil
	}

	return "", nil, config.Invalid("pose3.json not found")
}
