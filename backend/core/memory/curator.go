// Package memory 记忆服务
package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Completer 是后台整理使用的一次性补全能力
type Completer interface {
	Complete(ctx context.Context, system, user string, maxTokens int) (string, error)
}

const (
	// 画像之外，自动整理只带最近的若干条长期记忆给模型参考，避免每轮都把整份记忆发出去
	curatorMemoryContext = 30
	curatorMaxOps        = 3
	// 少于这么多字的话基本是寒暄或语气词，不值得为它调用一次模型
	curatorMinRunes = 4
)

// 目标 daily 表示当天的事件与短期安排，写进每日笔记，之后随时间衰减
const TargetDaily Target = "daily"

const curatorSystemPrompt = `你是“记忆整理员”，负责维护语音陪伴助手对用户的长期记忆。每轮对话结束后，你判断这一轮里有没有值得长期记住的信息。

有三个去处：
- user：用户画像，每轮都会放进提示词，必须极简。只放稳定的个人信息：称呼、身份职业、所在地、长期的偏好与禁忌、希望被怎样对待。
- memory：长期记忆。放以后可能用到的事实：用户的宠物、家人、项目，做出的决定，约定，重要的人和事。
- daily：当天的事件和短期安排（会议、待办、今天发生的事）。这类内容会随时间淡化。

不要记录：寒暄、一次性的提问、常识或随时能查到的东西、助手自己说的话、没说完的话、任何密码或密钥。

规则：
0. 只依据【本轮用户说的话】。你看不到助手的回复，也不要猜它说了什么：助手可能说错、可能编造，绝不能当作用户信息的来源。用户只是在提问（“我叫什么名字？”“我养的猫叫什么？”）而没有陈述事实时，什么都不要记。
1. 条目用第三人称，以“用户……”开头，一条一个事实，尽量短，信息密集。
2. 已有条目已经表达了同样意思就不要重复添加。
3. 新信息与已有条目冲突时（换了称呼、搬了家、改了偏好），用 patch 修改那一条，不要再加一条互相矛盾的。old_text 必须是已有条目里能唯一定位它的一段原文。
4. 用户明确要求忘记某件事时，用 remove。
5. 最多 3 个操作。没有值得记的就返回空列表。

只输出 JSON，不要任何解释：
{"ops":[{"target":"user|memory|daily","action":"add|patch|remove","content":"新内容","old_text":"patch/remove 时用于定位的原文"}]}`

// Curator 在每轮对话结束后，让模型决定要不要记住什么，并交给 MemoryService 执行。
// 对应 Hermes 里“智能体主动保存”；语音场景没有让模型调用工具的交互，所以用后台整理代替，
// 不在回复的关键路径上，失败也只记日志
type Curator struct {
	svc     *MemoryService
	llm     Completer
	timeout time.Duration

	busy sync.Mutex // 同一时刻只整理一轮，忙时丢弃新的请求而不是排队
}

func NewCurator(svc *MemoryService, llm Completer) *Curator {
	return &Curator{svc: svc, llm: llm, timeout: 25 * time.Second}
}

// OnTurn 在一轮对话正常结束后异步触发
func (c *Curator) OnTurn(user, assistant string) {
	if !c.busy.TryLock() {
		return
	}
	go func() {
		defer c.busy.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		defer cancel()
		res, err := c.review(ctx, user, assistant)
		if err != nil {
			log.Printf("[memory] curate failed: %v", err)
			return
		}
		for _, r := range res {
			switch {
			case r.Err != nil:
				log.Printf("[memory] curate %s/%s skipped: %v", r.Op.Target, r.Op.Action, r.Err)
			case r.Applied:
				log.Printf("[memory] curate %s/%s ok: %q", r.Op.Target, r.Op.Action, firstNonBlank(r.Op.Content, r.Op.OldText))
			}
		}
	}()
}

// Review 同步执行一次整理，供测试和手动触发使用
func (c *Curator) Review(ctx context.Context, user, assistant string) ([]OpResult, error) {
	c.busy.Lock()
	defer c.busy.Unlock()
	return c.review(ctx, user, assistant)
}

// assistant 参数只为与观察者接口保持一致，整理时不使用，原因见 buildUserPrompt
func (c *Curator) review(ctx context.Context, user, _ string) ([]OpResult, error) {
	user = strings.TrimSpace(user)
	if utf8.RuneCountInString(user) < curatorMinRunes {
		return nil, nil
	}
	prompt, err := c.buildUserPrompt(user)
	if err != nil {
		return nil, err
	}
	out, err := c.llm.Complete(ctx, curatorSystemPrompt, prompt, 400)
	if err != nil {
		return nil, err
	}
	ops, err := parseCuratorOps(out)
	if err != nil {
		return nil, err
	}
	return c.apply(ops), nil
}

func (c *Curator) buildUserPrompt(user string) (string, error) {
	profile, err := c.svc.bank.Entries(TargetUser)
	if err != nil {
		return "", err
	}
	long, err := c.svc.bank.Entries(TargetMemory)
	if err != nil {
		return "", err
	}
	if len(long) > curatorMemoryContext {
		long = long[len(long)-curatorMemoryContext:]
	}
	var sb strings.Builder
	sb.WriteString("【已有的用户画像 USER】\n")
	writeList(&sb, profile)
	sb.WriteString("\n【已有的长期记忆 MEMORY（最近几条）】\n")
	writeList(&sb, long)
	// 刻意不带助手的回复：它可能是幻觉（例如在没有任何记录时编出一个名字），一旦被整理进记忆就会固化成“事实”
	sb.WriteString("\n【本轮用户说的话】\n" + user)
	return sb.String(), nil
}

func writeList(sb *strings.Builder, items []string) {
	if len(items) == 0 {
		sb.WriteString("（空）\n")
		return
	}
	for _, it := range items {
		sb.WriteString("- " + it + "\n")
	}
}

// parseCuratorOps 从模型输出里取出 JSON；兼容被 ``` 包裹或前后带说明文字的情况，并限制操作数量
func parseCuratorOps(out string) ([]Op, error) {
	s := strings.TrimSpace(out)
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, errors.New("curator output has no JSON object")
	}
	var parsed struct {
		Ops []Op `json:"ops"`
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), &parsed); err != nil {
		return nil, fmt.Errorf("curator output is not valid JSON: %w", err)
	}
	if len(parsed.Ops) > curatorMaxOps {
		parsed.Ops = parsed.Ops[:curatorMaxOps]
	}
	return parsed.Ops, nil
}

// apply 只接受画像、长期记忆、每日笔记三个目标；daily 只允许追加
func (c *Curator) apply(ops []Op) []OpResult {
	var normal []Op
	var results []OpResult
	for _, op := range ops {
		op.Target = Target(strings.ToLower(strings.TrimSpace(string(op.Target))))
		if op.Target == TargetDaily {
			r := OpResult{Op: op}
			if strings.ToLower(strings.TrimSpace(op.Action)) != "add" {
				r.Err = errors.New("daily notes only support add")
			} else if r.Err = c.svc.bank.AppendDaily(op.Content); r.Err == nil {
				r.Applied = true
			}
			results = append(results, r)
			continue
		}
		normal = append(normal, op)
	}
	results = append(results, c.svc.Apply(normal)...)
	if len(results) > 0 {
		_ = c.svc.Sync() // 每日笔记也要进索引
	}
	return results
}
