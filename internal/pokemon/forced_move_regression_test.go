package pokemon

import (
	"math/rand"
	"strings"
	"testing"
)

func TestForcedMovesKeepTypeAtZeroPP(t *testing.T) {
	for _, key := range []string{"FLY", "THRASH"} {
		t.Run(key, func(t *testing.T) {
			g := New("forced move")
			g.rng = rand.New(rand.NewSource(7))
			p, foe := newMonster(151, 50), newMonster(92, 50)
			if key == "FLY" {
				foe = newMonster(1, 50)
			}
			p.Moves = [4]int{reviewMove(t, key), -1, -1, -1}
			p.PP = [4]int{}
			if key == "FLY" {
				p.Combat.ChargeMove = p.Moves[0] + 1
			} else {
				p.Combat.RepeatMove, p.Combat.RepeatTurns = 1, 2
			}
			before := foe.HP
			g.attack(&p, &foe, 0)
			log := strings.Join(g.Log, "\n")
			if key == "THRASH" && (foe.HP != before || !strings.Contains(log, "没有效果")) {
				t.Fatalf("normal repeated move bypassed ghost immunity: HP %d -> %d; %s", before, foe.HP, log)
			}
			if key == "THRASH" && p.Combat.RepeatTurns != 1 {
				t.Fatal("immune target stalled repeated move countdown")
			}
			if key == "FLY" && (foe.HP >= before || !strings.Contains(log, "效果拔群")) {
				t.Fatalf("charged flying move lost grass matchup: HP %d -> %d; %s", before, foe.HP, log)
			}
			if p.PP[0] != 0 {
				t.Fatal("forced release consumed PP again")
			}
		})
	}
}

func TestRepeatedMoveMissStillExpires(t *testing.T) {
	g := New("repeat miss")
	g.rng = rand.New(rand.NewSource(1))
	p, foe := newMonster(151, 50), newMonster(113, 50)
	p.Moves = [4]int{reviewMove(t, "THRASH"), -1, -1, -1}
	p.PP = [4]int{}
	p.Combat.RepeatMove, p.Combat.RepeatTurns = 1, 1
	p.Combat.Stages[4], foe.Combat.Stages[5] = -6, 6
	before := foe.HP
	g.attack(&p, &foe, 0)
	if foe.HP != before || !strings.Contains(strings.Join(g.Log, "\n"), "没有命中") {
		t.Fatal("fixture did not miss")
	}
	if p.Combat.RepeatTurns != 0 || p.Combat.Confusion <= 0 {
		t.Fatal("missed final repeated attack did not expire and apply confusion")
	}
}

func TestForcedMovePriorityUsesCommittedMove(t *testing.T) {
	p := newMonster(151, 50)
	p.Moves = [4]int{reviewMove(t, "FLY"), reviewMove(t, "QUICK_ATTACK"), -1, -1}
	p.PP = [4]int{0, 5}
	for _, phase := range []string{"charge", "repeat", "bide", "recharge", "struggle"} {
		t.Run(phase, func(t *testing.T) {
			m := p
			switch phase {
			case "charge":
				m.Combat.ChargeMove = m.Moves[0] + 1
			case "repeat":
				m.Combat.RepeatMove, m.Combat.RepeatTurns = 1, 2
			case "bide":
				m.Combat.BideTurns = 1
			case "recharge":
				m.Combat.Recharge = 1
			case "struggle":
				m.PP = [4]int{}
			}
			if got := movePriority(&m, 1); got != 0 {
				t.Fatalf("selected Quick Attack changed forced %s priority to %d", phase, got)
			}
		})
	}
	if got := movePriority(&p, 1); got != 1 {
		t.Fatal("ordinary Quick Attack lost priority")
	}
}
