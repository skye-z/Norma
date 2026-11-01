//go:build darwin

package audio

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"norma/backend/core/runtime/bus"
)

// 麦克风由前端 WebAudio 采集并推流，本机原生采集在 macOS 上不提供
type PlatformCaptureSource struct{}

func NewPlatformCaptureSource(sampleRate, channels int) (*PlatformCaptureSource, error) {
	_, _ = sampleRate, channels
	return nil, errors.New("native capture is not provided on macOS; microphone frames come from the frontend")
}

func (s *PlatformCaptureSource) Next(ctx context.Context) ([]float32, error) {
	_, _ = s, ctx
	return nil, ErrEndOfStream
}

func (s *PlatformCaptureSource) Close() error { return nil }

func (s *PlatformCaptureSource) EnableBackgroundCapture(sessionID string, enabled bool) {
	_, _, _ = s, sessionID, enabled
}

// 用系统 afplay 逐块播放 PCM；每个块写成临时 WAV 顺序播放，停止时终止子进程并清空队列
type platformPlaybackSink struct {
	bus        *bus.Bus
	sampleRate int
	channels   int

	mu   sync.Mutex
	cur  *playSession
	temp string
}

type playSession struct {
	id, trace string
	queue     chan []byte
	done      chan struct{}
	pending   int
	complete  bool
	stopped   bool
	closed    bool
	proc      *exec.Cmd
	started   bool
	stopAt    time.Time
}

func NewPlatformPlaybackSink(b *bus.Bus, sampleRate, channels int, encoding string) (*platformPlaybackSink, error) {
	if encoding != "" && encoding != "pcm_s16le" {
		return nil, errors.New("unsupported playback encoding: " + encoding)
	}
	if _, err := exec.LookPath("afplay"); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "norma-play-")
	if err != nil {
		return nil, err
	}
	return &platformPlaybackSink{bus: b, sampleRate: sampleRate, channels: channels, temp: dir}, nil
}

func (s *platformPlaybackSink) publish(name, sid, tid string, payload map[string]any) {
	if s.bus != nil {
		s.bus.Publish(bus.Event{Name: name, SessionID: sid, TraceID: tid, Payload: payload})
	}
}

func (s *platformPlaybackSink) session(sid, tid string) *playSession {
	if s.cur != nil && s.cur.id == sid && !s.cur.closed {
		return s.cur
	}
	ps := &playSession{id: sid, trace: tid, queue: make(chan []byte, 256), done: make(chan struct{})}
	s.cur = ps
	go s.worker(ps)
	return ps
}

func (s *platformPlaybackSink) Queue(sessionID, traceID string, chunk []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.session(sessionID, traceID)
	if ps.stopped || ps.complete {
		return nil
	}
	ps.pending++
	select {
	case ps.queue <- append([]byte(nil), chunk...):
	default:
		ps.pending--
		return errors.New("playback queue full")
	}
	return nil
}

func (s *platformPlaybackSink) worker(ps *playSession) {
	n := 0
	for chunk := range ps.queue {
		s.mu.Lock()
		stopped := ps.stopped
		if !stopped && !ps.started {
			ps.started = true
			s.mu.Unlock()
			s.publish("audio.play.start", ps.id, ps.trace, map[string]any{"source": "afplay", "sample_rate": s.sampleRate, "channels": s.channels})
			s.mu.Lock()
		}
		s.mu.Unlock()
		if !stopped {
			n++
			path := filepath.Join(s.temp, ps.id+"-"+itoa(n)+".wav")
			if err := writeWAVFile(path, chunk, s.sampleRate, s.channels); err == nil {
				cmd := exec.Command("afplay", path)
				s.mu.Lock()
				if ps.stopped {
					s.mu.Unlock()
				} else {
					ps.proc = cmd
					err = cmd.Start()
					s.mu.Unlock()
					if err == nil {
						_ = cmd.Wait()
					}
				}
			}
			_ = os.Remove(path)
		}
		s.mu.Lock()
		ps.pending--
		s.mu.Unlock()
	}
	s.mu.Lock()
	reason := "done"
	if ps.stopped {
		reason = "stopped"
	}
	ps.closed = true
	s.mu.Unlock()
	s.publish("audio.play.end", ps.id, ps.trace, map[string]any{"source": "afplay", "reason": reason, "pending": 0})
	close(ps.done)
}

func (s *platformPlaybackSink) Complete(sessionID, traceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.cur
	if ps == nil || ps.id != sessionID || ps.complete || ps.closed {
		return
	}
	ps.complete = true
	close(ps.queue)
}

func (s *platformPlaybackSink) Stop(sessionID, traceID, reason string) {
	s.mu.Lock()
	ps := s.cur
	if ps == nil || ps.id != sessionID || ps.closed || ps.stopped {
		s.mu.Unlock()
		return
	}
	ps.stopped = true
	ps.stopAt = time.Now()
	if ps.proc != nil && ps.proc.Process != nil {
		_ = ps.proc.Process.Kill()
	}
	if !ps.complete {
		ps.complete = true
		close(ps.queue)
	}
	s.mu.Unlock()
	s.publish("audio.play.flush", sessionID, traceID, map[string]any{"source": "afplay", "reason": reason, "elapsed_ms": time.Since(ps.stopAt).Milliseconds()})
	<-ps.done
}

func (s *platformPlaybackSink) Drain(ctx context.Context, sessionID string) error {
	for {
		s.mu.Lock()
		ps := s.cur
		idle := ps == nil || ps.id != sessionID || ps.closed || ps.pending == 0
		s.mu.Unlock()
		if idle {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (s *platformPlaybackSink) Wait(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	ps := s.cur
	s.mu.Unlock()
	if ps == nil || ps.id != sessionID {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ps.done:
		return nil
	}
}

func (s *platformPlaybackSink) Close() error {
	return os.RemoveAll(s.temp)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func writeWAVFile(path string, pcm []byte, rate, ch int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hdr := make([]byte, 44)
	copy(hdr[0:], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+len(pcm)))
	copy(hdr[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1)
	binary.LittleEndian.PutUint16(hdr[22:], uint16(ch))
	binary.LittleEndian.PutUint32(hdr[24:], uint32(rate))
	binary.LittleEndian.PutUint32(hdr[28:], uint32(rate*ch*2))
	binary.LittleEndian.PutUint16(hdr[32:], uint16(ch*2))
	binary.LittleEndian.PutUint16(hdr[34:], 16)
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(len(pcm)))
	if _, err := f.Write(hdr); err != nil {
		return err
	}
	_, err = f.Write(pcm)
	return err
}
