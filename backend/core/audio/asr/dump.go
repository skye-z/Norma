package audio

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	coreaudio "norma/backend/core/audio"
)

// DumpingASR 包装一个帧识别器，把管道交给它的每段音频存成 wav，并在日志里写出识别结果。
// 只用于排查“说了没识别/识别错了”这类问题，由 audio.debug_dump_dir 开启，默认关闭
type DumpingASR struct {
	inner coreaudio.FrameASR
	dir   string

	mu  sync.Mutex
	seq int
}

func NewDumpingASR(inner coreaudio.FrameASR, dir string) *DumpingASR {
	return &DumpingASR{inner: inner, dir: dir}
}

func (d *DumpingASR) Stream(ctx context.Context, onPartial func(string), onFinal func(string)) error {
	return d.inner.Stream(ctx, onPartial, onFinal)
}

func (d *DumpingASR) TranscribeFrames(ctx context.Context, frames [][]float32, onPartial func(string), onFinal func(string)) error {
	path := d.save(frames)
	var result string
	err := d.inner.TranscribeFrames(ctx, frames, onPartial, func(s string) {
		result = s
		if onFinal != nil {
			onFinal(s)
		}
	})
	samples := 0
	for _, f := range frames {
		samples += len(f)
	}
	log.Printf("[asr-dump] file=%s duration=%.2fs result=%q err=%v", path, float64(samples)/16000, result, err)
	return err
}

func (d *DumpingASR) save(frames [][]float32) string {
	d.mu.Lock()
	d.seq++
	n := d.seq
	d.mu.Unlock()
	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		return ""
	}
	path := filepath.Join(d.dir, fmt.Sprintf("utt-%s-%03d.wav", time.Now().Format("150405"), n))
	f, err := os.Create(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	pcm := flattenToPCM16(frames)
	if err := writeWavHeader(f, 16000, uint32(len(pcm)*2)); err != nil {
		return path
	}
	buf := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		buf[2*i], buf[2*i+1] = byte(uint16(v)), byte(uint16(v)>>8)
	}
	_, _ = f.Write(buf)
	return path
}
