// Package ops 提供运维相关的工具函数
// 包括日志路由、文件管理等运维辅助功能
package ops

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// LogCloser 日志文件关闭器
// 管理所有日志文件的句柄，支持统一关闭
type LogCloser struct {
	closers []io.Closer
}

// Close 关闭所有日志文件句柄
// 返回第一个遇到的错误（如果有）
func (c *LogCloser) Close() error {
	var firstErr error
	for _, cl := range c.closers {
		if err := cl.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// SetupLogRouting 配置日志路由
// 将标准日志输出同时写入app.log、audio.log、rag.log文件和stdout
// 参数：logDir-日志目录路径，默认"logs"
// 返回：LogCloser用于关闭文件句柄，以及可能的错误
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

	// 创建应用日志文件
	appFile, err := os.OpenFile(appPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	// 创建音频处理日志文件
	audioFile, err := os.OpenFile(audioPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = appFile.Close()
		return nil, err
	}
	// 创建RAG检索日志文件
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

// categoryWriter 分类日志写入器
// 根据日志内容将消息同时写入多个目标：stdout、app.log、audio.log、rag.log
// 线程安全，支持并发写入
type categoryWriter struct {
	mu     sync.Mutex
	stdout io.Writer // 标准输出
	app    io.Writer // 应用日志
	audio  io.Writer // 音频处理日志
	rag    io.Writer // RAG检索日志
}

// Write 实现io.Writer接口，根据日志内容进行分类路由
// 所有日志都会写入stdout和app.log
// 包含特定关键词的日志会同时写入audio.log或rag.log
func (w *categoryWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	line := strings.ToLower(string(p))

	// 始终写入标准输出
	if _, err := w.stdout.Write(p); err != nil {
		return 0, err
	}

	// 始终写入应用日志
	if _, err := w.app.Write(p); err != nil {
		return 0, err
	}

	// 音频相关日志写入audio.log
	if isAudioLog(line) {
		if _, err := w.audio.Write(p); err != nil {
			return 0, err
		}
	}
	// RAG相关日志写入rag.log
	if isRAGLog(line) {
		if _, err := w.rag.Write(p); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

// isAudioLog 判断日志是否为音频处理相关
// 关键词：[asr], [tts], [audio], vad, whisper, xtts, tts.chunk
func isAudioLog(line string) bool {
	keys := []string{"[asr]", "[tts]", "[audio]", "vad", "whisper", "xtts", "tts.chunk"}
	for _, k := range keys {
		if strings.Contains(line, k) {
			return true
		}
	}
	return false
}

// isRAGLog 判断日志是否为RAG检索相关
// 关键词：[rag], [index], retriever, fts, hnsw
func isRAGLog(line string) bool {
	keys := []string{"[rag]", "[index]", "retriever", "fts", "hnsw"}
	for _, k := range keys {
		if strings.Contains(line, k) {
			return true
		}
	}
	return false
}
