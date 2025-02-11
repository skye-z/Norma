package ops

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestCategoryWriterRouting(t *testing.T) {
	var stdout bytes.Buffer
	var app bytes.Buffer
	var audio bytes.Buffer
	var rag bytes.Buffer

	w := &categoryWriter{
		stdout: &stdout,
		app:    &app,
		audio:  &audio,
		rag:    &rag,
	}

	logger := log.New(w, "", 0)
	logger.Printf("[asr] partial hello")
	logger.Printf("[rag] use hybrid retriever")
	logger.Printf("[runtime] state changed")

	if !strings.Contains(app.String(), "[asr]") || !strings.Contains(app.String(), "[rag]") {
		t.Fatalf("expected app log to contain all entries")
	}
	if !strings.Contains(audio.String(), "[asr]") {
		t.Fatalf("expected audio log to contain asr entry")
	}
	if strings.Contains(audio.String(), "[runtime]") {
		t.Fatalf("did not expect runtime entry in audio log")
	}
	if !strings.Contains(rag.String(), "[rag]") {
		t.Fatalf("expected rag log to contain rag entry")
	}
	if strings.Contains(rag.String(), "[runtime]") {
		t.Fatalf("did not expect runtime entry in rag log")
	}
	if !strings.Contains(stdout.String(), "[runtime]") {
		t.Fatalf("expected stdout to contain runtime entry")
	}
}
