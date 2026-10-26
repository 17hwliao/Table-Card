package pokemon

import (
	"math/rand"
	"testing"
)

func reviewMove(t *testing.T, key string) int {
	t.Helper()
	for i, m := range Moves {
		if m.Key == key {
			return i
		}
	}
	t.Fatal("missing move", key)
	return -1
}

func TestReviewOnlyDisabledMoveCanAdvanceBattle(t *testing.T) {
	for _, online := range []bool{false, true} {
		m := newMonster(63, 10) // Abra knows only Teleport at this level.
		m.Combat.DisableSlot, m.Combat.DisableTurns = 1, 3
		if online {
			d, err := NewDuel([2]string{"A", "B"}, [2][]Monster{{m}, {newMonster(19, 10)}})
			if err != nil {
				t.Fatal(err)
			}
			d.Teams[0][0].Combat = m.Combat
			if err = d.Submit(0, DuelAction{Kind: "move", Slot: 0}); err != nil {
				t.Errorf("online sole disabled move deadlocks: %v", err)
			}
		} else {
			g := New("review")
			g.Party = []Monster{m}
			g.Battle = &Battle{Kind: "trainer", Foes: []Monster{newMonster(19, 10)}}
			if err := g.Command("move 1"); err != nil {
				t.Errorf("offline sole disabled move deadlocks: %v", err)
			}
		}
	}
}

func TestReviewDuelReleasesLastPPCharge(t *testing.T) {
	m := newMonster(1, 50)
	charge := reviewMove(t, "SOLARBEAM")
	m.Moves = [4]int{charge, -1, -1, -1}
	m.PP = [4]int{1, 0, 0, 0}
	d, err := NewDuel([2]string{"A", "B"}, [2][]Monster{{m}, {newMonster(19, 50)}})
	if err != nil {
		t.Fatal(err)
	}
	d.Teams[0][0].PP[0] = 0
	d.Teams[0][0].Combat.ChargeMove = charge + 1
	if err = d.Submit(0, DuelAction{Kind: "move", Slot: 0}); err != nil {
		t.Fatal(err)
	}
	if err = d.Submit(1, DuelAction{Kind: "move", Slot: 0}); err != nil {
		t.Fatal(err)
	}
	if d.Teams[0][0].Combat.ChargeMove != 0 {
		t.Fatal("charge did not release")
	}
}

func TestReviewDuelChargeWithOtherPP(t *testing.T) {
	m := newMonster(1, 50)
	charge := reviewMove(t, "SOLARBEAM")
	d, err := NewDuel([2]string{"A", "B"}, [2][]Monster{{m}, {newMonster(19, 50)}})
	if err != nil {
		t.Fatal(err)
	}
	d.Teams[0][0].Moves[0] = charge
	d.Teams[0][0].PP[0] = 0
	d.Teams[0][0].Combat.ChargeMove = charge + 1
	if err = d.Submit(0, DuelAction{Kind: "move", Slot: 0}); err != nil {
		t.Fatalf("last charge PP blocks release although other moves have PP: %v", err)
	}
}

func TestReviewTeleportClearsTemporaryCombat(t *testing.T) {
	g := New("review")
	g.rng = rand.New(rand.NewSource(1))
	g.Party = []Monster{newMonster(63, 20)}
	g.Party[0].Combat.Stages[0] = 3
	g.Party[0].Combat.Substitute = 20
	g.startBattle("wild", "wild", []Monster{newMonster(19, 2)}, "", 0)
	g.Party[0].Combat.Stages[0] = 3
	g.Party[0].Combat.Substitute = 20
	if err := g.Command("move 1"); err != nil {
		t.Fatal(err)
	}
	if g.Battle != nil {
		t.Fatal("teleport failed to exit")
	}
	if g.Party[0].Combat != (CombatState{}) {
		t.Fatal("battle-only effects survive teleport")
	}
}

func TestReviewExperienceCurvesAndLegalSpeciesMoves(t *testing.T) {
	for id := 1; id <= 151; id++ {
		last := -1
		for level := 1; level <= 100; level++ {
			x := ExperienceForSpecies(id, level)
			if x <= last {
				t.Fatalf("nonmonotonic %d/%d", id, level)
			}
			last = x
			m := newMonster(id, level)
			before := m
			SanitizeMonster(&m)
			if before != m {
				t.Fatalf("fresh monster data not normalized: %d/%d", id, level)
			}
			if level < 100 && m.ExperienceProgress().Remaining <= 0 {
				t.Fatal("missing next level")
			}
		}
	}
}
