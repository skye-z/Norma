//go:build windows

package desktop

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"norma/pkg/apiserver"
	"norma/pkg/layout"
)

func (a *App) startLocalAPI() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.apiServer != nil {
		return nil
	}

	root, err := detectProjectRoot()
	if err != nil {
		return err
	}
	if err := layout.EnsureRuntimeLayout(root); err != nil {
		return err
	}

	srv, err := apiserver.New(apiserver.Options{
		Addr:       ":8090",
		DBPath:     layout.Join(root, layout.DBPath),
		ModelDir:   layout.Join(root, layout.ModelDir),
		ConfigPath: layout.Join(root, layout.ConfigFileName),
		DebugDir:   layout.ResolveDebugDir(root),
		StartedAt:  time.Now(),
	})
	if err != nil {
		return err
	}
	if err := srv.Start(); err != nil {
		return err
	}
	log.Printf("embedded api server listening on %s", srv.Addr())
	a.apiServer = srv
	return nil
}

func (a *App) stopLocalAPI() {
	a.mu.Lock()
	srv := a.apiServer
	a.apiServer = nil
	a.mu.Unlock()
	if srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Close(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("close embedded api server: %v", err)
	}
}

func detectProjectRoot() (string, error) {
	candidates := make([]string, 0, 8)
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	for _, base := range candidates {
		for _, dir := range walkParents(base, 5) {
			if hasProjectMarkers(dir) {
				return dir, nil
			}
		}
	}
	return "", errors.New("project root not found")
}

func walkParents(start string, limit int) []string {
	out := make([]string, 0, limit+1)
	current := filepath.Clean(start)
	for i := 0; i <= limit; i++ {
		out = append(out, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return out
}

func hasProjectMarkers(dir string) bool {
	if layout.HasRuntimeMarkers(dir) {
		return true
	}
	legacyMarkers := []string{
		filepath.Join(dir, "configs", "config.yaml"),
		filepath.Join(dir, "model"),
		filepath.Join(dir, "web", "debug"),
	}
	for _, marker := range legacyMarkers {
		if _, err := os.Stat(marker); err != nil {
			return false
		}
	}
	return true
}
