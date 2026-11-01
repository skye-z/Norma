// Package audio 音频处理
package audio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// 一段参考音频及其对应文本，Genie 据此克隆语气与情绪
type GenieRef struct {
	Audio string `json:"audio"`
	Text  string `json:"text"`
}

// Genie 合成配置
type GenieConfig struct {
	// 启动命令前缀；生产环境为 [python, worker.py]，测试可替换为其他程序
	Command      []string
	DataDir      string
	CharacterDir string
	Language     string
	// 情绪名到参考音频，必须含 default
	Refs         map[string]GenieRef
	StartTimeout time.Duration
}

// 通过应用自己管理的子进程调用 Genie-TTS（GPT-SoVITS 的 ONNX 推理）
// 子进程经标准输入输出通信，不监听网络端口，随应用退出而结束
type GenieTTS struct {
	cfg GenieConfig

	mu      sync.Mutex // 保护进程状态与请求发送
	proc    *genieProc
	nextID  int64
	cancels map[int]context.CancelFunc
	seq     int
}

type genieResult struct {
	pcm  []byte
	rate int
	err  error
}

type genieProc struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	mu      sync.Mutex
	pending map[int64]chan genieResult
	dead    chan struct{}
}

func NewGenieTTS(cfg GenieConfig) *GenieTTS {
	if cfg.StartTimeout <= 0 {
		cfg.StartTimeout = 60 * time.Second
	}
	if cfg.Language == "" {
		cfg.Language = "zh"
	}
	return &GenieTTS{cfg: cfg, cancels: map[int]context.CancelFunc{}}
}

// Genie V2ProPlus 输出 32kHz 单声道 16 位
func (g *GenieTTS) SampleRate() int { return 32000 }

func (g *GenieTTS) Channels() int { return 1 }

func (g *GenieTTS) Encoding() string { return "pcm_s16le" }

func (g *GenieTTS) SupportsProsodyHints() bool { return true }

var genieEmotionRe = regexp.MustCompile(`\{\{\s*(?:[a-zA-Z]+\s*:\s*)?([A-Za-z_]+)\s*\}\}`)

// 取文本中第一个动作标记作为情绪名，没有时返回 default
func emotionFromText(text string) string {
	if m := genieEmotionRe.FindStringSubmatch(text); m != nil {
		return strings.ToLower(m[1])
	}
	return "default"
}

// 预先拉起子进程，避免第一句话承担加载耗时
func (g *GenieTTS) Warmup() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, err := g.ensureLocked()
	return err
}

func (g *GenieTTS) ensureLocked() (*genieProc, error) {
	if g.proc != nil {
		select {
		case <-g.proc.dead:
			g.proc = nil
		default:
			return g.proc, nil
		}
	}
	if len(g.cfg.Command) == 0 {
		return nil, fmt.Errorf("genie command is empty")
	}
	refs, err := json.Marshal(g.cfg.Refs)
	if err != nil {
		return nil, err
	}
	args := append(append([]string{}, g.cfg.Command[1:]...),
		"--data-dir", g.cfg.DataDir, "--char-dir", g.cfg.CharacterDir,
		"--language", g.cfg.Language, "--refs", string(refs))
	cmd := exec.Command(g.cfg.Command[0], args...)
	cmd.Env = append(os.Environ(), "GENIE_DATA_DIR="+g.cfg.DataDir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start genie worker: %w", err)
	}
	p := &genieProc{cmd: cmd, stdin: stdin, pending: map[int64]chan genieResult{}, dead: make(chan struct{})}
	go func() { // 子进程日志转到应用日志，避免管道写满阻塞
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			log.Printf("[tts/genie] %s", sc.Text())
		}
	}()
	ready := make(chan error, 1)
	go p.readLoop(bufio.NewReaderSize(stdout, 1<<20), ready)
	select {
	case err := <-ready:
		if err != nil {
			_ = cmd.Process.Kill()
			return nil, err
		}
	case <-time.After(g.cfg.StartTimeout):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("genie worker not ready within %s", g.cfg.StartTimeout)
	}
	log.Printf("[tts/genie] worker ready pid=%d", cmd.Process.Pid)
	g.proc = p
	return p, nil
}

func (p *genieProc) readLoop(r *bufio.Reader, ready chan<- error) {
	defer func() {
		close(p.dead)
		_ = p.cmd.Wait()
		p.mu.Lock()
		for id, ch := range p.pending {
			ch <- genieResult{err: fmt.Errorf("genie worker exited")}
			delete(p.pending, id)
		}
		p.mu.Unlock()
	}()
	first := true
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if first {
				ready <- fmt.Errorf("genie worker exited before ready: %w", err)
			}
			return
		}
		var h struct {
			Ready      bool   `json:"ready"`
			ID         *int64 `json:"id"`
			OK         bool   `json:"ok"`
			SampleRate int    `json:"sample_rate"`
			Bytes      int    `json:"bytes"`
			Error      string `json:"error"`
		}
		if json.Unmarshal(line, &h) != nil {
			continue // 非协议行忽略
		}
		if first {
			first = false
			if h.Ready {
				ready <- nil
				continue
			}
			ready <- fmt.Errorf("unexpected first line from genie worker: %q", strings.TrimSpace(string(line)))
			return
		}
		var res genieResult
		if h.OK {
			buf := make([]byte, h.Bytes)
			if _, err := io.ReadFull(r, buf); err != nil {
				return
			}
			res = genieResult{pcm: buf, rate: h.SampleRate}
		} else {
			res = genieResult{err: fmt.Errorf("genie: %s", h.Error)}
		}
		if h.ID == nil {
			continue
		}
		p.mu.Lock()
		ch := p.pending[*h.ID]
		delete(p.pending, *h.ID)
		p.mu.Unlock()
		if ch != nil { // 已取消的请求没有等待者，结果直接丢弃
			ch <- res
		}
	}
}

// 合成一个分句；情绪取自文本中的动作标记
func (g *GenieTTS) Stream(ctx context.Context, text string, onChunk func(chunk []byte)) error {
	emotion := emotionFromText(text)
	clean := cleanSpeechText(text)
	if clean == "" {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	g.mu.Lock()
	g.seq++
	seq := g.seq
	g.cancels[seq] = cancel
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		delete(g.cancels, seq)
		g.mu.Unlock()
		cancel()
	}()

	g.mu.Lock()
	p, err := g.ensureLocked()
	if err != nil {
		g.mu.Unlock()
		return err
	}
	g.nextID++
	id := g.nextID
	ch := make(chan genieResult, 1)
	p.mu.Lock()
	p.pending[id] = ch
	p.mu.Unlock()
	req, _ := json.Marshal(map[string]any{"id": id, "text": clean, "emotion": emotion})
	_, werr := p.stdin.Write(append(req, '\n'))
	g.mu.Unlock()
	if werr != nil {
		return fmt.Errorf("send to genie worker: %w", werr)
	}

	select {
	case res := <-ch:
		if res.err != nil {
			return res.err
		}
		if res.rate != 0 && res.rate != g.SampleRate() {
			log.Printf("[tts/genie] sample rate mismatch got=%d expected=%d", res.rate, g.SampleRate())
		}
		if onChunk != nil && len(res.pcm) > 0 {
			onChunk(res.pcm)
		}
		return nil
	case <-ctx.Done():
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return ctx.Err()
	}
}

// 取消所有在途合成；子进程里已开始的推理会自行跑完，结果被丢弃
func (g *GenieTTS) Stop(sessionID string) error {
	_ = sessionID
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, c := range g.cancels {
		c()
	}
	return nil
}

// 结束子进程
func (g *GenieTTS) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.proc != nil {
		_ = g.proc.stdin.Close()
		select {
		case <-g.proc.dead:
		case <-time.After(2 * time.Second):
			_ = g.proc.cmd.Process.Kill()
		}
		g.proc = nil
	}
}

// 读取角色目录里 prompt_wav.json 的第一项作为默认参考音频
func DefaultGenieRef(characterDir string) (GenieRef, error) {
	raw, err := os.ReadFile(filepath.Join(characterDir, "prompt_wav.json"))
	if err != nil {
		return GenieRef{}, err
	}
	var m map[string]struct {
		Wav  string `json:"wav"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return GenieRef{}, err
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if m[k].Wav != "" {
			return GenieRef{Audio: filepath.Join(characterDir, "prompt_wav", m[k].Wav), Text: m[k].Text}, nil
		}
	}
	return GenieRef{}, fmt.Errorf("prompt_wav.json has no entries")
}
