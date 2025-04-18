// Package live2d Live2D 相关功能
package live2d

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidationResult 验证结果
type ValidationResult struct {
	ModelDir      string
	HasCan3       bool
	HasModel3JSON bool
	HasCmo3       bool
	Issues        []string
}

// ValidateModelDir 验证模型目录
func ValidateModelDir(dir string) ValidationResult {
	res := ValidationResult{ModelDir: dir, Issues: make([]string, 0, 8)}

	fi, err := os.Stat(dir)
	if err != nil {
		res.Issues = append(res.Issues, fmt.Sprintf("model dir not found: %v", err))
		return res
	}
	if !fi.IsDir() {
		res.Issues = append(res.Issues, "model path is not a directory")
		return res
	}

	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".can3":
			res.HasCan3 = true
		case ".json":
			if strings.HasSuffix(strings.ToLower(path), ".model3.json") {
				res.HasModel3JSON = true
			}
		case ".cmo3":
			res.HasCmo3 = true
		}
		return nil
	})

	if !res.HasCan3 && !res.HasModel3JSON {
		res.Issues = append(res.Issues, "missing Live2D model config (.can3 or .model3.json)")
	}
	if !res.HasCmo3 {
		res.Issues = append(res.Issues, "missing Live2D binary model (.cmo3)")
	}

	return res
}

// OK 验证是否通过
func (r ValidationResult) OK() bool {
	return len(r.Issues) == 0
}
