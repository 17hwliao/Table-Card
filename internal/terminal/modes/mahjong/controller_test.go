package mahjong

import (
	"encoding/json"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	game "github.com/17hwliao/table-card-independent/internal/mahjong"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
)

func inputState(phase string) ui.Snapshot {
	s := game.Snapshot{Phase: phase, Hand: []game.Tile{{Suit: 0, Rank: 1}, {Suit: 0, Rank: 2}, {Suit: 0, Rank: 3}}, Players: []game.PublicPlayer{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}}
	raw, _ := json.Marshal(s)
	return ui.Snapshot{Game: raw, PlayerID: "a", Width: 100}
}

func TestSpaceToggleAndEscapeCancelStayAtTable(t *testing.T) {
	c := New()
	s := inputState("exchange")
	c.Key(s, " ")
	if len(c.selected) != 1 {
		t.Fatal("literal space must select tile")
	}
	c.Key(s, " ")
	if len(c.selected) != 0 {
		t.Fatal("deselection must remove entry")
	}
	c.Key(s, "space")
	c.Key(s, "right")
	c.Key(s, "space")
	c.Key(s, "right")
	c.Key(s, "space")
	result := c.Key(s, "enter")
	if !json.Valid(result.Action) {
		t.Fatal("three selected matching tiles must submit")
	}
	c.Key(s, "space")
	result = c.Key(s, "esc")
	if result.Exit || len(c.selected) != 0 {
		t.Fatal("Esc must cancel selection without leaving table")
	}
}

func TestClaimSpacePass(t *testing.T) {
	s := inputState("claim")
	var state game.Snapshot
	json.Unmarshal(s.Game, &state)
	state.ClaimOptions = []string{"peng"}
	s.Game, _ = json.Marshal(state)
	result := New().Key(s, " ")
	var action map[string]string
	if err := json.Unmarshal(result.Action, &action); err != nil || action["claim"] != "pass" {
		t.Fatalf("space must pass claim: %s", result.Action)
	}
}

func TestFourSeatsSurroundDiscardAndCompactWindowFits(t *testing.T) {
	s := inputState("turn")
	s.Width = 80
	s.Height = 24
	var state game.Snapshot
	json.Unmarshal(s.Game, &state)
	state.Players[0].Name = "SELF"
	state.Players[1].Name = "RIGHT"
	state.Players[2].Name = "TOP"
	state.Players[3].Name = "LEFT"
	state.LastDiscard = &game.Tile{Suit: 0, Rank: 4}
	state.Discarder = 0
	state.TurnPlayer = "a"
	state.HasDrawn = true
	for len(state.Hand) < 14 {
		state.Hand = append(state.Hand, game.Tile{Suit: 1, Rank: 8})
	}
	s.Game, _ = json.Marshal(state)
	view := New().View(s)
	lines := strings.Split(view, "\n")
	if len(lines) > 20 {
		t.Fatalf("compact table uses %d lines, leaves no space for app footer", len(lines))
	}
	for _, line := range lines {
		if lipgloss.Width(line) > s.Width {
			t.Fatalf("line exceeds window: %q", line)
		}
	}
	top, left, right, bottom := -1, -1, -1, -1
	for i, line := range lines {
		if strings.Contains(line, "TOP") {
			top = i
		}
		if strings.Contains(line, "LEFT") {
			left = i
		}
		if strings.Contains(line, "RIGHT") {
			right = i
		}
		if strings.Contains(line, "SELF") && !strings.Contains(line, "打出") {
			bottom = i
		}
	}
	if !(top >= 0 && top < left && left == right && right < bottom) {
		t.Fatalf("four-way seats misplaced: top%d left%d right%d bottom%d", top, left, right, bottom)
	}
	if !strings.Contains(view, "4万") || !strings.Contains(view, "🀊") {
		t.Fatal("discard must include recognisable glyph and text label")
	}
}
