package ui

import (
	"charm.land/lipgloss/v2"
	"testing"
)

func TestOptimizedLayoutMatchesExistingAlignment(t *testing.T) {
	cases := [][]string{nil, {""}, {"短", "abc\n12345", ""}, {"\x1b[31m棋子\x1b[0m\n◆", "  ", "e\u0301\n123\n"}, {"🀄\n牌桌", "长一些的标题\n", "\n"}}
	for _, blocks := range cases {
		if got, want := JoinLeft(blocks...), lipgloss.JoinVertical(lipgloss.Left, blocks...); got != want {
			t.Fatalf("vertical mismatch: %q / %q", got, want)
		}
		if got, want := JoinTop(blocks...), lipgloss.JoinHorizontal(lipgloss.Top, blocks...); got != want {
			t.Fatalf("horizontal mismatch: %q / %q", got, want)
		}
	}
}
