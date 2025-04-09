package memory

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestTextExtractorDOCX(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.docx")

	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	_, _ = w.Write([]byte(`<w:document><w:body><w:p><w:r><w:t>Hello DOCX</w:t></w:r></w:p></w:body></w:document>`))
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("file close: %v", err)
	}

	tx := TextExtractor{}
	out, err := tx.Extract(p)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if out != "Hello DOCX" {
		t.Fatalf("unexpected docx text: %q", out)
	}
}

func TestTextExtractorPDF(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.pdf")

	pdfData := []byte("%PDF-1.4\n1 0 obj\n<<>>\nstream\nBT (Hello PDF) Tj ET\nendstream\nendobj\n%%EOF")
	if err := os.WriteFile(p, pdfData, 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}

	tx := TextExtractor{}
	out, err := tx.Extract(p)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty pdf text")
	}
}
