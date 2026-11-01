// Package config 配置管理
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-viper/mapstructure/v2"
	"go.yaml.in/yaml/v3"
)

// 保存配置到 YAML 文件
func Save(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// 环境变量注入的密钥只在运行时使用，不能被规范化回存时写进配置文件
	toSave := *cfg
	if env := os.Getenv("NORMA_LLM_API_KEY"); env != "" && toSave.LLM.APIKey == env {
		toSave.LLM.APIKey = ""
	}

	doc := map[string]any{}
	if err := mapstructure.Decode(&toSave, &doc); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
