package pokemon

import (
	"math/rand"
	"strings"
	"testing"
)

func TestBattleMoveValidationAndForcedActions(t *testing.T) {
	m := newMonster(1, 50)
	m.Moves = [4]int{reviewMove(t, "TACKLE"), reviewMove(t, "GROWL"), -1, -1}
	m.PP = [4]int{0, 5}
	if validateBattleMove(&m, 0) == nil || validateBattleMove(&m, 2) == nil {
		t.Fatal("empty or exhausted slot accepted with another usable move")
	}
	m.PP[0] = 5
	m.Combat.DisableSlot, m.Combat.DisableTurns = 1, 3
	if validateBattleMove(&m, 0) == nil || validateBattleMove(&m, 1) != nil {
		t.Fatal("Disable ignored while another move is available")
	}
	m.PP[1] = 0
	if err := validateBattleMove(&m, 0); err != nil {
		t.Fatal("Struggle cannot be selected", err)
	}
	g := New("regression")
	g.rng = rand.New(rand.NewSource(7))
	foe := newMonster(113, 50)
	g.attack(&m, &foe, 0)
	if m.Combat.DisableTurns != 2 || !strings.Contains(strings.Join(g.Log, "\n"), "挣扎") || foe.HP == foe.MaxHP() {
		t.Fatal("Struggle failed to damage and advance Disable")
	}
	for _, key := range []string{"SOLARBEAM", "THRASH"} {
		t.Run(key, func(t *testing.T) {
			p := newMonster(1, 50)
			p.Moves = [4]int{reviewMove(t, key), reviewMove(t, "GROWL"), -1, -1}
			p.PP = [4]int{0, 5}
			if key == "SOLARBEAM" {
				p.Combat.ChargeMove = p.Moves[0] + 1
			} else {
				p.Combat.RepeatMove, p.Combat.RepeatTurns = 1, 2
			}
			d, err := NewDuel([2]string{"A", "B"}, [2][]Monster{{p}, {newMonster(113, 50)}})
			if err != nil {
				t.Fatal(err)
			}
			d.g.rng = rand.New(rand.NewSource(7))
			d.Teams[0][0] = p
			before := d.Teams[1][0].HP
			if err := d.Submit(0, DuelAction{Kind: "move", Slot: 0}); err != nil {
				t.Fatal(err)
			}
			if err := d.Submit(1, DuelAction{Kind: "move", Slot: 0}); err != nil {
				t.Fatal(err)
			}
			if d.Teams[1][0].HP >= before || d.Teams[0][0].PP[0] != 0 {
				t.Fatal("committed move did not hit or charged PP twice")
			}
			if key == "THRASH" && d.Teams[0][0].Combat.RepeatTurns != 1 {
				t.Fatal("repeat did not advance")
			}
		})
	}
}

func TestTeleportClearsWholePartyAndRestoresTransform(t *testing.T) {
	g := New("regression")
	g.Party = []Monster{newMonster(63, 20), newMonster(151, 20)}
	g.Party[0].HP--
	g.Party[0].Status = "中毒"
	original := g.Party[1]
	g.Party[1].Combat.TransformedSpecies = 19
	g.Party[1].Combat.OriginalMoves, g.Party[1].Combat.OriginalPP = original.Moves, original.PP
	g.Party[1].Moves = [4]int{reviewMove(t, "TACKLE"), -1, -1, -1}
	g.Party[0].Combat.Substitute = 20
	if err := g.startBattle("wild", "wild", []Monster{newMonster(19, 2)}, "", 0); err != nil {
		t.Fatal(err)
	}
	g.attack(&g.Party[0], &g.Battle.Foes[0], 0)
	if g.Battle != nil || g.Party[0].Combat != (CombatState{}) || g.Party[1].Combat != (CombatState{}) || g.Party[1].Moves != original.Moves || g.Party[1].PP != original.PP {
		t.Fatal("exit failed to clear entire party and restore transformed moves")
	}
	if g.Party[0].Status != "中毒" || g.Party[0].HP != g.Party[0].MaxHP()-1 {
		t.Fatal("exit cleared persistent health/status")
	}
}
