package llm

import (
	"strings"
	"testing"
)

func TestPersonaGoesInSystemMessageNotInTheUsersMessage(t *testing.T) {
	msgs := buildMessages(Request{Prompt: "今天天气怎么样", Persona: "你是测试角色阿宝。"})
	if msgs[0].Role != "system" || !strings.Contains(msgs[0].Content, "你是测试角色阿宝") {
		t.Fatalf("first message must be system with the persona: %+v", msgs[0])
	}
	last := msgs[len(msgs)-1]
	if last.Role != "user" || last.Content != "今天天气怎么样" {
		t.Fatalf("last message must be the plain user text, got %+v", last)
	}
	// 回归：人设混在用户消息里时，模型每一轮都把它当成用户的自我介绍，反复说“我是诺玛……”
	if strings.Contains(last.Content, "阿宝") {
		t.Fatal("persona leaked into the user message")
	}
}

func TestSystemPromptTellsModelNotToReintroduceItself(t *testing.T) {
	sys := buildMessages(Request{Prompt: "x"})[0].Content
	if !strings.Contains(sys, "不要主动自我介绍") {
		t.Fatalf("system prompt lacks the no-self-introduction rule:\n%s", sys)
	}
}

func TestHistoryIsInterleavedBeforeTheCurrentUserTurn(t *testing.T) {
	msgs := buildMessages(Request{
		Prompt: "我叫什么？",
		History: []Turn{
			{User: "我叫skye", Assistant: "好的，skye。"},
			{User: "今天不错", Assistant: "那就好。"},
		},
	})
	var roles []string
	for _, m := range msgs {
		roles = append(roles, m.Role)
	}
	if strings.Join(roles, ",") != "system,user,assistant,user,assistant,user" {
		t.Fatalf("roles = %v", roles)
	}
	if msgs[1].Content != "我叫skye" || msgs[4].Content != "那就好。" || msgs[5].Content != "我叫什么？" {
		t.Fatalf("history order wrong: %+v", msgs)
	}
}

func TestIncompleteHistoryTurnsAreSkipped(t *testing.T) {
	msgs := buildMessages(Request{Prompt: "q", History: []Turn{{User: "只有用户", Assistant: ""}, {User: "", Assistant: "只有助手"}}})
	if len(msgs) != 2 {
		t.Fatalf("incomplete turns must be dropped, got %d messages", len(msgs))
	}
}

func TestRetrievalContextGoesWithTheCurrentUserMessage(t *testing.T) {
	msgs := buildMessages(Request{Prompt: "我的猫叫什么", Context: []string{"我的猫叫团子"}})
	last := msgs[len(msgs)-1].Content
	if !strings.Contains(last, "[1] 我的猫叫团子") || !strings.HasSuffix(last, "我的猫叫什么") {
		t.Fatalf("context/prompt layout wrong:\n%s", last)
	}
	if strings.Contains(msgs[0].Content, "团子") {
		t.Fatal("retrieved memory must not be in the system prompt")
	}
}

func TestProfileGoesAtTheEndOfTheSystemMessageOnly(t *testing.T) {
	msgs := buildMessages(Request{Prompt: "我叫什么", Profile: "- 用户叫 Skye\n- 用户喜欢美式"})
	sys := msgs[0].Content
	if !strings.Contains(sys, "- 用户叫 Skye") || !strings.HasSuffix(sys, "- 用户喜欢美式") {
		t.Fatalf("profile should end the system prompt:\n%s", sys)
	}
	if strings.Contains(msgs[len(msgs)-1].Content, "Skye") {
		t.Fatal("profile must not leak into the user message")
	}
}

// 画像为空时要明确告诉模型“还不了解用户”，而不是什么都不说让它自己脑补；这段话也要保持稳定（有利于前缀缓存）
func TestEmptyProfileSaysNothingIsKnownAndStaysStable(t *testing.T) {
	a := buildMessages(Request{Prompt: "x"})[0].Content
	b := buildMessages(Request{Prompt: "y", Profile: "  \n"})[0].Content
	if !strings.HasSuffix(a, "目前还不了解用户的任何个人信息。") || a != b {
		t.Fatalf("empty profile must say nothing is known and keep the prompt identical:\n%s", a)
	}
}

// 回归：旧措辞“要自然地用起来，不要反复确认”让模型在没有记录时编造（问名字 4/8 次编出同一个名字）
func TestSystemPromptForbidsInventingPersonalFacts(t *testing.T) {
	sys := buildMessages(Request{Prompt: "x"})[0].Content
	for _, want := range []string{"如实说你还不知道", "绝不要编造", "你只知道两类关于用户的信息"} {
		if !strings.Contains(sys, want) {
			t.Errorf("system prompt lacks %q", want)
		}
	}
	if strings.Contains(sys, "不要反复确认。\n") && !strings.Contains(sys, "绝不要编造") {
		t.Error("the old wording without the honesty boundary must not return")
	}
}
