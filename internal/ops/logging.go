package ops

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// LogCloser closes underlying file handles created for logging.
type LogCloser struct {
	closers []io.Closer
}

func (c *LogCloser) Close() error {
	var firstErr error
	for _, cl := range c.closers {
		if err := cl.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// SetupLogRouting configures standard log output to app/audio/rag files plus stdout.
func SetupLogRouting(logDir string) (*LogCloser, error) {
	if strings.TrimSpace(logDir) == "" {
		logDir = "logs"
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	appPath := filepath.Join(logDir, "app.log")
	audioPath := filepath.Join(logDir, "audio.log")
	ragPath := filepath.Join(logDir, "rag.log")

	appFile, err := os.OpenFile(appPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	audioFile, err := os.OpenFile(audioPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = appFile.Close()
		return nil, err
	}
	ragFile, err := os.OpenFile(ragPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = appFile.Close()
		_ = audioFile.Close()
		return nil, err
	}

	router := &categoryWriter{
		stdout: os.Stdout,
		app:    appFile,
		audio:  audioFile,
		rag:    ragFile,
	}
	log.SetOutput(router)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	return &LogCloser{closers: []io.Closer{appFile, audioFile, ragFile}}, nil
}

type categoryWriter struct {
	mu     sync.Mutex
	stdout io.Writer
	app    io.Writer
	audio  io.Writer
	rag    io.Writer
}

func (w *categoryWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	line := strings.ToLower(string(p))

	if _, err := w.stdout.Write(p); err != nil {
		return 0, err
	}

	if _, err := w.app.Write(p); err != nil {
		return 0, err
	}

	if isAudioLog(line) {
		if _, err := w.audio.Write(p); err != nil {
			return 0, err
		}
	}
	if isRAGLog(line) {
		if _, err := w.rag.Write(p); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

func isAudioLog(line string) bool {
	keys := []string{"[asr]", "[tts]", "[audio]", "vad", "whisper", "xtts", "tts.chunk"}
	for _, k := range keys {
		if strings.Contains(line, k) {
			return true
		}
	}
	return false
}

func isRAGLog(line string) bool {
	keys := []string{"[rag]", "[index]", "retriever", "fts", "hnsw"}
	for _, k := range keys {
		if strings.Contains(line, k) {
			return true
		}
	}
	return false
}
