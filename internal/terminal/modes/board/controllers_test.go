package board

import (
	"charm.land/lipgloss/v2"
	"encoding/json"
	"github.com/17hwliao/table-card-independent/internal/chinesechess"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
	"strings"
	"testing"
)

func TestXiangqiTenthRankKeyboardAndMouse(t *testing.T) {
	game, _ := chinesechess.New([]table.Player{{ID: "a"}, {ID: "b"}})
	raw, _ := json.Marshal(game.Snapshot())
	s := ui.Snapshot{Game: raw, PlayerID: "a"}
	c := &ChineseChess{}
	c.Reset()
	c.Key(s, "down")
	if c.y != 9 {
		t.Fatalf("last rank unreachable: %d", c.y)
	}
	if r := c.Mouse(s, 4, 11); len(r.Action) != 0 || c.from == nil || c.from[1] != 9 {
		t.Fatal("mouse cannot select last rank")
	}
	r := c.Mouse(s, 4, 10)
	var a struct{ FromY, ToY int }
	if err := json.Unmarshal(r.Action, &a); err != nil {
		t.Fatal(err)
	}
	if a.FromY != 9 || a.ToY != 8 {
		t.Fatalf("incorrect mouse mapping: %+v", a)
	}
	rows := strings.Split(c.View(s), "\n")
	for i := 2; i < 12; i++ {
		if lipgloss.Width(rows[i]) != 41 {
			t.Fatalf("misaligned row %d width %d", i, lipgloss.Width(rows[i]))
		}
	}
}
func TestMouseRejectsBorderAndOutside(t *testing.T) {
	for _, p := range [][2]int{{3, 2}, {4, 1}, {40, 2}, {4, 12}} {
		if _, _, ok := hitCell(p[0], p[1], 9, 10); ok {
			t.Fatalf("outside click accepted: %v", p)
		}
	}
}
