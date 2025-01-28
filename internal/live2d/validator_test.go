package live2d

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateModelDir(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.can3"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.cmo3"), []byte("x"), 0o644)

	res := ValidateModelDir(dir)
	if !res.OK() {
		t.Fatalf("expected ok, issues=%v", res.Issues)
	}
}
