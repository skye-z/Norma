//go:build windows && cgo && sherpa_embedded

package audio

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/windows"
)

var sherpaDLLPathOnce sync.Once

func ensureSherpaDLLPath() {
	sherpaDLLPathOnce.Do(func() {
		if dir := findSherpaDLLDir(); dir != "" {
			if err := windows.SetDllDirectory(dir); err != nil {
				log.Printf("[sherpa] set dll directory failed: %v", err)
			} else {
				log.Printf("[sherpa] dll directory set: %s", dir)
			}
			prependPath(dir)
			return
		}
		log.Printf("[sherpa] dll directory not found")
	})
}

func findSherpaDLLDir() string {
	candidates := []string{}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, ".gomodcache"))
	}
	if gomodcache := strings.TrimSpace(os.Getenv("GOMODCACHE")); gomodcache != "" {
		candidates = append(candidates, gomodcache)
	}
	if gopath := strings.TrimSpace(os.Getenv("GOPATH")); gopath != "" {
		candidates = append(candidates, filepath.Join(gopath, "pkg", "mod"), filepath.Join(gopath, "Library"))
	}
	if out, err := exec.Command("go", "env", "GOMODCACHE").Output(); err == nil {
		candidates = append(candidates, strings.TrimSpace(string(out)))
	}

	seen := map[string]struct{}{}
	for _, root := range candidates {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		matches, _ := filepath.Glob(filepath.Join(root, "github.com", "k2-fsa", "sherpa-onnx-go-windows@*", "lib", "x86_64-pc-windows-gnu"))
		for _, match := range matches {
			if _, err := os.Stat(filepath.Join(match, "onnxruntime.dll")); err == nil {
				return match
			}
		}
	}
	return ""
}

func prependPath(dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return
	}
	parts := strings.Split(os.Getenv("PATH"), ";")
	for _, part := range parts {
		if strings.EqualFold(strings.TrimSpace(part), dir) {
			return
		}
	}
	_ = os.Setenv("PATH", dir+";"+os.Getenv("PATH"))
}
