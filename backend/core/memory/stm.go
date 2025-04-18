// Package memory 记忆服务
package memory

import (
	"sort"
	"strings"
)

// Turn 对话轮次，用于 STM 预算修剪
type Turn struct {
	Role       string
	Content    string
	Importance int
	Pinned     bool
}

// TrimToTokenBudget 修剪对话历史以适应 Token 预算
func TrimToTokenBudget(turns []Turn, maxTokens int) []Turn {
	if maxTokens <= 0 || len(turns) == 0 {
		return nil
	}

	type scoredTurn struct {
		idx    int
		turn   Turn
		tokens int
		score  int
	}

	scored := make([]scoredTurn, 0, len(turns))
	for i, turn := range turns {
		tokens := estimateTokens(turn.Content)
		if tokens == 0 {
			continue
		}
		score := turn.Importance*100 + i
		if turn.Pinned {
			score += 10000
		}
		scored = append(scored, scoredTurn{idx: i, turn: turn, tokens: tokens, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].idx > scored[j].idx
		}
		return scored[i].score > scored[j].score
	})

	selected := make([]scoredTurn, 0, len(scored))
	used := 0
	for _, item := range scored {
		if used+item.tokens > maxTokens && len(selected) > 0 {
			continue
		}
		if item.tokens > maxTokens && len(selected) == 0 {
			selected = append(selected, item)
			break
		}
		selected = append(selected, item)
		used += item.tokens
	}

	sort.Slice(selected, func(i, j int) bool { return selected[i].idx < selected[j].idx })
	out := make([]Turn, 0, len(selected))
	for _, item := range selected {
		out = append(out, item.turn)
	}
	return out
}

func estimateTokens(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	runes := len([]rune(s))
	if runes <= 4 {
		return 1
	}
	return (runes + 3) / 4
}
