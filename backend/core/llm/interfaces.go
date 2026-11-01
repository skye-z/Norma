// Package llm LLM接口定义和实现
package llm

import (
	"context"
	"errors"
	"strings"
	"time"
)

// LLM请求结构
type Request struct {
	SessionID string   // 会话唯一标识符
	Prompt    string   // 用户输入的提示词
	Context   []string // 检索增强的上下文信息
	Persona   string   // 角色设定
	History   []Turn   // 最近几轮对话，按时间从早到晚
}

// Turn 是已经完成的一轮对话
type Turn struct {
	User      string
	Assistant string
}

// 流式输出接口
type Streamer interface {
	// Stream 执行流式推理
	// ctx-上下文，req-LLM请求，onToken-每个token生成时的回调函数
	Stream(ctx context.Context, req Request, onToken func(string)) error
}

// 开发用模拟LLM流处理器
type MockStreamer struct{}

// 模拟流式输出
func (m MockStreamer) Stream(ctx context.Context, req Request, onToken func(string)) error {
	_ = m
	text := "收到，我先给你结论：这是一个可运行的 mock 语音链路。"
	// 如果有用户输入，将其包含在响应中
	if strings.TrimSpace(req.Prompt) != "" {
		text += " 你刚刚说的是：" + strings.TrimSpace(req.Prompt) + "。"
	}
	// 如果有上下文信息，表明使用了RAG检索
	if len(req.Context) > 0 {
		text += " 我也参考了本地检索结果。"
	}

	// 逐字符流式输出，模拟真实LLM的行为
	for _, token := range strings.Split(text, "") {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			onToken(token)
			time.Sleep(25 * time.Millisecond)
		}
	}

	return nil
}

// 不可用的流处理器
type UnavailableStreamer struct {
	Reason string // 不可用原因
}

// 返回LLM不可用的错误
func (u UnavailableStreamer) Stream(ctx context.Context, req Request, onToken func(string)) error {
	_ = ctx
	_ = req
	_ = onToken
	if strings.TrimSpace(u.Reason) == "" {
		return errors.New("llm unavailable")
	}
	return errors.New("llm unavailable: " + strings.TrimSpace(u.Reason))
}
