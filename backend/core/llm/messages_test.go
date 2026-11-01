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
