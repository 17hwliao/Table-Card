package pokemon

import (
	"math/rand"
	"strings"
	"testing"
)

func TestOriginalSignatureTracksActualLastMove(t *testing.T) {
	g := New("signature tracking")
	g.Party = []Monster{newMonster(25, 25)}
	g.Badges = []string{"first", "second"}
	if err := g.Command("train 1"); err != nil {
		t.Fatal(err)
	}
	p := &g.Party[0]
	id, slot := signatureMove(p.Species), -1
	for i, move := range p.Moves {
		if move == id {
			slot = i
		}
	}
	if slot < 0 {
		t.Fatal("teaching did not install signature move")
	}
	foe := newMonster(113, 100)
	g.rng = rand.New(rand.NewSource(2))
	g.attack(p, &foe, slot)
	if p.Combat.LastMove != id+1 {
		t.Fatalf("signature tracked as %d, want %d (%s)", p.Combat.LastMove, id+1, Moves[id].Name)
	}

	// Disable must locate the actual last move in its owner's slots.
	foe.Moves, foe.PP = [4]int{reviewMove(t, "DISABLE"), -1, -1, -1}, [4]int{20}
	g.rng = rand.New(rand.NewSource(2)) // First accuracy draw hits Disable.
	g.attack(&foe, p, 0)
	if p.Combat.DisableSlot != slot+1 || p.Combat.DisableTurns <= 0 {
		t.Fatal("Disable could not identify the signature move", g.Log)
	}

	foe.Moves, foe.PP = [4]int{reviewMove(t, "MIRROR_MOVE"), -1, -1, -1}, [4]int{20}
	g.rng = rand.New(rand.NewSource(2))
	g.attack(&foe, p, 0)
	if !strings.Contains(strings.Join(g.Log, "\n"), "复制了"+Moves[id].Name) {
		t.Fatal("Mirror Move copied a different move after signature", g.Log)
	}
}

func TestOriginalSignatureIDsMatchTheirSlots(t *testing.T) {
	for species := 1; species <= 151; species++ {
		id := signatureMove(species)
		if Moves[id].ID != id {
			t.Fatalf("signature %d has ID %d, want %d", species, Moves[id].ID, id)
		}
	}
}
