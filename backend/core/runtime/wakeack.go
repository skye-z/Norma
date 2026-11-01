package runtime

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"norma/backend/core/audio"
	"norma/backend/core/runtime/bus"
)

// 唤醒应答语：唤醒词命中或用户点击麦克风按钮后，立刻播放一句预先合成好的“嗯？”，
// 让用户知道应用已经开始接收后续语音。音频在启动时合成一次并缓存到磁盘，之后每次直接播放，
// 与收音并行进行，不等待也不阻塞后面的识别。
type WakeAck struct {
	text     string
	tts      audio.TTS
	sink     playbackSink
	cacheDir string

	mu        sync.Mutex
	pcm       []byte
	busyUntil time.Time
	seq       int64
	played    atomic.Int64
}

// 请求播放应答语的事件名
const WakeAckEvent = "audio.wake.ack"

// 创建应答服务并订阅请求事件。sink 应是独立于回复播放的接收器，避免干扰回复的停止与打断
func NewWakeAck(b *bus.Bus, tts audio.TTS, sink playbackSink, text, cacheDir string) *WakeAck {
	a := &WakeAck{text: text, tts: tts, sink: sink, cacheDir: cacheDir}
	if b != nil {
		b.Subscribe(WakeAckEvent, a.onRequest)
	}
	return a
}

// 提供合成器身份，用于判断缓存是否仍然有效（换了声音、角色或参考音频就要重新合成）
type ttsIdentity interface {
	Identity() string
}

func (a *WakeAck) cacheKey() string {
	id := fmt.Sprintf("%T", a.tts)
	if v, ok := a.tts.(ttsIdentity); ok {
		id = v.Identity()
	}
	rate := 0
	if f, ok := a.tts.(audio.AudioFormatProvider); ok {
		rate = f.SampleRate()
	}
	// v2：缓存内容已裁掉首尾静音
	sum := sha1.Sum([]byte(fmt.Sprintf("v2|%s|%s|%d", a.text, id, rate)))
	return hex.EncodeToString(sum[:8])
}

// 加载磁盘缓存，没有则合成并写入缓存；已就绪后重复调用不做任何事
func (a *WakeAck) Prepare(ctx context.Context) error {
	if a.text == "" {
		return nil
	}
	a.mu.Lock()
	ready := a.pcm != nil
	a.mu.Unlock()
	if ready {
		return nil
	}
	path := filepath.Join(a.cacheDir, "wake_ack_"+a.cacheKey()+".pcm")
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		a.setPCM(b)
		log.Printf("[wake-ack] loaded cache %s bytes=%d", path, len(b))
		return nil
	}

	var pcm []byte
	if err := a.tts.Stream(ctx, a.text, func(c []byte) { pcm = append(pcm, c...) }); err != nil {
		return fmt.Errorf("synthesize wake ack: %w", err)
	}
	if len(pcm) == 0 {
		return errors.New("wake ack synthesis produced no audio")
	}
	pcm = a.trim(pcm)
	if len(pcm) == 0 {
		return errors.New("wake ack synthesis produced only silence")
	}
	a.setPCM(pcm)
	if a.cacheDir != "" {
		if err := os.MkdirAll(a.cacheDir, 0o755); err == nil {
			tmp := path + ".tmp"
			if err := os.WriteFile(tmp, pcm, 0o644); err == nil {
				_ = os.Rename(tmp, path)
			}
		}
	}
	log.Printf("[wake-ack] synthesized text=%q bytes=%d", a.text, len(pcm))
	return nil
}

func (a *WakeAck) setPCM(b []byte) {
	a.mu.Lock()
	a.pcm = b
	a.mu.Unlock()
}

// 已播放的次数，供测试和诊断使用
func (a *WakeAck) PlayedCount() int64 { return a.played.Load() }

func (a *WakeAck) onRequest(evt bus.Event) {
	a.mu.Lock()
	pcm := a.pcm
	now := time.Now()
	if pcm == nil || now.Before(a.busyUntil) {
		a.mu.Unlock()
		return // 还没准备好，或上一句还在播放，直接忽略，宁可少播也不叠音
	}
	a.seq++
	sid := fmt.Sprintf("wake-ack-%d", a.seq)
	a.busyUntil = now.Add(a.duration(len(pcm)))
	a.mu.Unlock()

	source := "-"
	if m, ok := evt.Payload.(map[string]any); ok {
		if v, ok := m["source"].(string); ok {
			source = v
		}
	}
	log.Printf("[wake-ack] play source=%s session=%s trigger_session=%s", source, sid, evt.SessionID)
	if err := a.sink.Queue(sid, evt.TraceID, pcm); err != nil {
		log.Printf("[wake-ack] queue failed: %v", err)
		return
	}
	a.sink.Complete(sid, evt.TraceID)
	a.played.Add(1)
}

// 按采样格式估算播放时长，用于避免相邻两次请求叠音；格式未知时取一个保守值
func (a *WakeAck) duration(n int) time.Duration {
	rate, ch := 24000, 1
	if f, ok := a.tts.(audio.AudioFormatProvider); ok {
		if r := f.SampleRate(); r > 0 {
			rate = r
		}
		if c := f.Channels(); c > 0 {
			ch = c
		}
	}
	return time.Duration(float64(n) / float64(rate*ch*2) * float64(time.Second))
}

// 裁掉首尾静音。GPT-SoVITS 对很短的文本会在语音前生成约一秒的近静音，
// 不裁掉的话“嗯？”会晚一秒才响，失去“立刻回应”的意义。仅处理 16 位单声道，其余格式原样返回
func (a *WakeAck) trim(pcm []byte) []byte {
	rate, ch := 24000, 1
	if f, ok := a.tts.(audio.AudioFormatProvider); ok {
		if r := f.SampleRate(); r > 0 {
			rate = r
		}
		if c := f.Channels(); c > 0 {
			ch = c
		}
		if f.Encoding() != "" && f.Encoding() != "pcm_s16le" {
			return pcm
		}
	}
	if ch != 1 {
		return pcm
	}
	return TrimSilencePCM16(pcm, rate)
}

// TrimSilencePCM16 按 20ms 窗口计算 RMS，以峰值窗口的 8% 为阈值找出语音起止，
// 前后各保留一小段以免切到发音边缘。全程低于绝对静音下限时返回 nil
func TrimSilencePCM16(pcm []byte, rate int) []byte {
	n := len(pcm) / 2
	win := rate / 50
	if n == 0 || win <= 0 {
		return pcm
	}
	rms := make([]float64, 0, n/win+1)
	peak := 0.0
	for i := 0; i < n; i += win {
		end := i + win
		if end > n {
			end = n
		}
		var sum float64
		for j := i; j < end; j++ {
			v := float64(int16(uint16(pcm[2*j]) | uint16(pcm[2*j+1])<<8))
			sum += v * v
		}
		r := math.Sqrt(sum / float64(end-i))
		rms = append(rms, r)
		if r > peak {
			peak = r
		}
	}
	const absoluteFloor = 50.0 // 16 位采样值，低于它视为没有声音
	if peak < absoluteFloor {
		return nil
	}
	thr := peak * 0.08
	if thr < absoluteFloor {
		thr = absoluteFloor
	}
	first, last := -1, -1
	for i, r := range rms {
		if r >= thr {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return nil
	}
	lead, tail := 2, 3 // 40ms 与 60ms
	start := (first - lead) * win
	if start < 0 {
		start = 0
	}
	end := (last + 1 + tail) * win
	if end > n {
		end = n
	}
	return append([]byte(nil), pcm[2*start:2*end]...)
}
