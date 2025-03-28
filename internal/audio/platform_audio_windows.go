//go:build windows

package audio

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"norma/internal/runtime/bus"
)

const (
	waveFormatPCM    = 1
	callbackFunction = 0x00030000

	wimData = 0x03C0
	womDone = 0x03BD

	waveMapper = 0xFFFFFFFF
)

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	Size           uint16
}

type waveHdr struct {
	Data          *byte
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

var (
	winmmDLL               = syscall.NewLazyDLL("winmm.dll")
	procWaveInOpen         = winmmDLL.NewProc("waveInOpen")
	procWaveInPrepareHdr   = winmmDLL.NewProc("waveInPrepareHeader")
	procWaveInUnprepareHdr = winmmDLL.NewProc("waveInUnprepareHeader")
	procWaveInAddBuffer    = winmmDLL.NewProc("waveInAddBuffer")
	procWaveInStart        = winmmDLL.NewProc("waveInStart")
	procWaveInStop         = winmmDLL.NewProc("waveInStop")
	procWaveInReset        = winmmDLL.NewProc("waveInReset")
	procWaveInClose        = winmmDLL.NewProc("waveInClose")

	procWaveOutOpen         = winmmDLL.NewProc("waveOutOpen")
	procWaveOutPrepareHdr   = winmmDLL.NewProc("waveOutPrepareHeader")
	procWaveOutUnprepareHdr = winmmDLL.NewProc("waveOutUnprepareHeader")
	procWaveOutWrite        = winmmDLL.NewProc("waveOutWrite")
	procWaveOutReset        = winmmDLL.NewProc("waveOutReset")
	procWaveOutClose        = winmmDLL.NewProc("waveOutClose")

	captureCallbackProc  = syscall.NewCallback(captureWaveCallback)
	playbackCallbackProc = syscall.NewCallback(playbackWaveCallback)
	nativeSeq            atomic.Uint64
	captureRegistry      sync.Map
	playbackRegistry     sync.Map
)

type PlatformCaptureSource struct {
	id          uint64
	handle      uintptr
	frames      chan []float32
	done        chan struct{}
	closeOnce   sync.Once
	headers     []*waveHdr
	buffers     [][]byte
	sampleRate  int
	channels    int
	frameLength int
	bgCapture   atomic.Bool
	bgWindow    atomic.Int64
}

type platformPlaybackSink struct {
	mu         sync.Mutex
	id         uint64
	handle     uintptr
	bus        *bus.Bus
	sampleRate int
	channels   int
	encoding   string
	started    bool
	sessionID  string
	traceID    string
	pending    int
	complete   bool
	ended      bool
	done       chan struct{}
	buffers    map[*waveHdr][]byte
	flushStart time.Time
}

func NewPlatformCaptureSource(sampleRate, channels int) (*PlatformCaptureSource, error) {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	if channels <= 0 {
		channels = 1
	}
	frameLength := sampleRate / 50
	if frameLength <= 0 {
		frameLength = 320
	}
	s := &PlatformCaptureSource{
		id:          nativeSeq.Add(1),
		frames:      make(chan []float32, 32),
		done:        make(chan struct{}),
		sampleRate:  sampleRate,
		channels:    channels,
		frameLength: frameLength,
	}
	s.bgWindow.Store(700)
	format := waveFormatEx{
		FormatTag:      waveFormatPCM,
		Channels:       uint16(channels),
		SamplesPerSec:  uint32(sampleRate),
		BitsPerSample:  16,
		BlockAlign:     uint16(channels * 2),
		AvgBytesPerSec: uint32(sampleRate * channels * 2),
	}
	var handle uintptr
	if err := mmCall(procWaveInOpen, uintptr(unsafe.Pointer(&handle)), uintptr(waveMapper), uintptr(unsafe.Pointer(&format)), captureCallbackProc, uintptr(s.id), callbackFunction); err != nil {
		return nil, err
	}
	s.handle = handle
	captureRegistry.Store(s.id, s)
	for i := 0; i < 4; i++ {
		buf := make([]byte, frameLength*channels*2)
		hdr := &waveHdr{Data: &buf[0], BufferLength: uint32(len(buf))}
		if err := mmCall(procWaveInPrepareHdr, s.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr)); err != nil {
			s.Close()
			return nil, err
		}
		if err := mmCall(procWaveInAddBuffer, s.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr)); err != nil {
			s.Close()
			return nil, err
		}
		s.headers = append(s.headers, hdr)
		s.buffers = append(s.buffers, buf)
	}
	if err := mmCall(procWaveInStart, s.handle); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *PlatformCaptureSource) Next(ctx context.Context) ([]float32, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, ErrEndOfStream
	case frame, ok := <-s.frames:
		if !ok {
			return nil, ErrEndOfStream
		}
		return frame, nil
	}
}

func (s *PlatformCaptureSource) EnableBackgroundCapture(sessionID string, enabled bool) {
	_ = sessionID
	s.bgCapture.Store(enabled)
	if enabled {
		s.bgWindow.Store(time.Now().Add(700 * time.Millisecond).UnixMilli())
	} else {
		s.bgWindow.Store(0)
	}
}

func (s *PlatformCaptureSource) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		captureRegistry.Delete(s.id)
		if s.handle != 0 {
			_, _, _ = procWaveInStop.Call(s.handle)
			_, _, _ = procWaveInReset.Call(s.handle)
			for _, hdr := range s.headers {
				_, _, _ = procWaveInUnprepareHdr.Call(s.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
			}
			_, _, _ = procWaveInClose.Call(s.handle)
		}
		close(s.frames)
	})
	return nil
}

func captureWaveCallback(_ uintptr, msg uint32, instance, param1, _ uintptr) uintptr {
	if msg != wimData || param1 == 0 {
		return 0
	}
	v, ok := captureRegistry.Load(instance)
	if !ok {
		return 0
	}
	s := v.(*PlatformCaptureSource)
	hdr := (*waveHdr)(unsafe.Pointer(param1))
	if hdr == nil || hdr.BytesRecorded == 0 {
		_, _, _ = procWaveInAddBuffer.Call(s.handle, param1, unsafe.Sizeof(*hdr))
		return 0
	}
	frame := pcmBytesToFloat32(unsafe.Slice(hdr.Data, int(hdr.BytesRecorded)), s.channels)
	if s.bgCapture.Load() {
		if until := s.bgWindow.Load(); until > 0 && time.Now().UnixMilli() > until {
			s.bgCapture.Store(false)
			s.bgWindow.Store(0)
		}
	}
	select {
	case <-s.done:
		return 0
	case s.frames <- frame:
	default:
	}
	hdr.BytesRecorded = 0
	_, _, _ = procWaveInAddBuffer.Call(s.handle, param1, unsafe.Sizeof(*hdr))
	return 0
}

func NewPlatformPlaybackSink(b *bus.Bus, sampleRate, channels int, encoding string) (*platformPlaybackSink, error) {
	if sampleRate <= 0 {
		sampleRate = 22050
	}
	if channels <= 0 {
		channels = 1
	}
	if encoding == "" {
		encoding = "pcm_s16le"
	}
	if sampleRate > 24000 {
		sampleRate = 24000
	}
	s := &platformPlaybackSink{
		id:         nativeSeq.Add(1),
		bus:        b,
		sampleRate: sampleRate,
		channels:   channels,
		encoding:   encoding,
		done:       make(chan struct{}),
		buffers:    make(map[*waveHdr][]byte),
	}
	format := waveFormatEx{
		FormatTag:      waveFormatPCM,
		Channels:       uint16(channels),
		SamplesPerSec:  uint32(sampleRate),
		BitsPerSample:  16,
		BlockAlign:     uint16(channels * 2),
		AvgBytesPerSec: uint32(sampleRate * channels * 2),
	}
	var handle uintptr
	if err := mmCall(procWaveOutOpen, uintptr(unsafe.Pointer(&handle)), uintptr(waveMapper), uintptr(unsafe.Pointer(&format)), playbackCallbackProc, uintptr(s.id), callbackFunction); err != nil {
		return nil, err
	}
	s.handle = handle
	playbackRegistry.Store(s.id, s)
	return s, nil
}

func (s *platformPlaybackSink) Queue(sessionID, traceID string, chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		s.started = true
		s.sessionID = sessionID
		s.traceID = traceID
		s.complete = false
		s.ended = false
		s.done = make(chan struct{})
		if s.bus != nil {
			s.bus.Publish(bus.Event{Name: "audio.play.start", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{"source": "waveOut", "sample_rate": s.sampleRate, "channels": s.channels, "encoding": s.encoding}})
		}
	}
	s.flushStart = time.Now()
	if len(chunk) > 4096 {
		chunk = chunk[:4096]
	}
	data := make([]byte, len(chunk))
	copy(data, chunk)
	hdr := &waveHdr{Data: &data[0], BufferLength: uint32(len(data)), User: uintptr(len(data))}
	if err := mmCall(procWaveOutPrepareHdr, s.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr)); err != nil {
		return err
	}
	if err := mmCall(procWaveOutWrite, s.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr)); err != nil {
		_, _, _ = procWaveOutUnprepareHdr.Call(s.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
		return err
	}
	s.pending++
	s.buffers[hdr] = data
	return nil
}

func (s *platformPlaybackSink) Complete(sessionID, traceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionID != sessionID {
		return
	}
	s.complete = true
	if s.pending == 0 {
		s.finishLocked(traceID, "done")
	}
}

func (s *platformPlaybackSink) Stop(sessionID, traceID, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionID != sessionID || s.ended {
		return
	}
	_, _, _ = procWaveOutReset.Call(s.handle)
	for hdr := range s.buffers {
		_, _, _ = procWaveOutUnprepareHdr.Call(s.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
		delete(s.buffers, hdr)
	}
	s.pending = 0
	s.complete = true
	if s.bus != nil {
		s.bus.Publish(bus.Event{Name: "audio.play.flush", SessionID: sessionID, TraceID: traceID, Payload: map[string]any{"source": "waveOut", "reason": reason, "elapsed_ms": time.Since(s.flushStart).Milliseconds()}})
	}
	s.finishLocked(traceID, reason)
}

func (s *platformPlaybackSink) Wait(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	if s.sessionID != sessionID || s.ended {
		s.mu.Unlock()
		return nil
	}
	done := s.done
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (s *platformPlaybackSink) Close() error {
	playbackRegistry.Delete(s.id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handle != 0 {
		_, _, _ = procWaveOutReset.Call(s.handle)
		for hdr := range s.buffers {
			_, _, _ = procWaveOutUnprepareHdr.Call(s.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
		}
		_, _, _ = procWaveOutClose.Call(s.handle)
		s.handle = 0
	}
	return nil
}

func playbackWaveCallback(_ uintptr, msg uint32, instance, param1, _ uintptr) uintptr {
	if msg != womDone || param1 == 0 {
		return 0
	}
	v, ok := playbackRegistry.Load(instance)
	if !ok {
		return 0
	}
	s := v.(*platformPlaybackSink)
	hdr := (*waveHdr)(unsafe.Pointer(param1))
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.buffers[hdr]; !exists {
		return 0
	}
	bytes := int(hdr.User)
	_, _, _ = procWaveOutUnprepareHdr.Call(s.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
	delete(s.buffers, hdr)
	if s.pending > 0 {
		s.pending--
	}
	if s.bus != nil {
		s.bus.Publish(bus.Event{Name: "audio.play.progress", SessionID: s.sessionID, TraceID: s.traceID, Payload: map[string]any{"source": "waveOut", "bytes": bytes}})
	}
	if s.complete && s.pending == 0 {
		s.finishLocked(s.traceID, "done")
	}
	return 0
}

func (s *platformPlaybackSink) finishLocked(traceID, reason string) {
	if s.ended {
		return
	}
	s.ended = true
	if s.bus != nil {
		s.bus.Publish(bus.Event{Name: "audio.play.end", SessionID: s.sessionID, TraceID: traceID, Payload: map[string]any{"source": "waveOut", "reason": reason, "pending": s.pending}})
	}
	close(s.done)
}

func pcmBytesToFloat32(data []byte, channels int) []float32 {
	if channels <= 0 {
		channels = 1
	}
	samples := len(data) / 2
	if samples == 0 {
		return nil
	}
	if channels == 1 {
		out := make([]float32, samples)
		for i := 0; i < samples; i++ {
			v := int16(uint16(data[i*2]) | uint16(data[i*2+1])<<8)
			out[i] = float32(v) / 32768
		}
		return out
	}
	frames := samples / channels
	out := make([]float32, frames)
	for i := 0; i < frames; i++ {
		sum := float32(0)
		for ch := 0; ch < channels; ch++ {
			idx := (i*channels + ch) * 2
			v := int16(uint16(data[idx]) | uint16(data[idx+1])<<8)
			sum += float32(v) / 32768
		}
		out[i] = sum / float32(channels)
	}
	return out
}

func mmCall(proc *syscall.LazyProc, a ...uintptr) error {
	r1, _, _ := proc.Call(a...)
	if r1 != 0 {
		return fmt.Errorf("%s failed: %d", proc.Name, r1)
	}
	return nil
}
