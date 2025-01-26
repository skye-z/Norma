package memory

import "strings"

// Turn is a conversation turn used by STM budget trimming.
type Turn struct {
	Role    string
	Content string
}

// TrimToTokenBudget keeps recent turns within budget.
func TrimToTokenBudget(turns []Turn, maxTokens int) []Turn {
	if maxTokens <= 0 || len(turns) == 0 {
		return nil
	}

	total := 0
	start := len(turns)
	for i := len(turns) - 1; i >= 0; i-- {
		t := estimateTokens(turns[i].Content)
		if total+t > maxTokens {
			break
		}
		total += t
		start = i
	}
	if start >= len(turns) {
		return nil
	}
	out := make([]Turn, len(turns[start:]))
	copy(out, turns[start:])
	return out
}

func estimateTokens(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Rough estimate for mixed zh/en text.
	runes := len([]rune(s))
	if runes <= 4 {
		return 1
	}
	return (runes + 3) / 4
}
