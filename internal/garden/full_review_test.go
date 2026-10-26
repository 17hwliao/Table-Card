package garden

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/17hwliao/table-card-independent/internal/table"
)

func reviewEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := NewEngine([]table.Player{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}})
	if err != nil {
		t.Fatal(err)
	}
	e.startAt = time.Now().Add(-time.Second)
	e.last = e.startAt
	e.nextWave = 1000000
	e.rng = rand.New(rand.NewSource(1))
	return e
}

func TestReviewRelayEverySpeciesRestoresState(t *testing.T) {
	for kind := Normal; kind <= Door; kind++ {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			e := reviewEngine(t)
			z := e.makeZombie(kind, 3, 7)
			z.HP, z.Armor, z.Slow, z.Jumped, z.Summoned = 0, 0, 40, true, true
			e.lawns[0].Zombies = append(e.lawns[0].Zombies, z)
			e.step()
			if len(e.lawns[0].Zombies) != 0 || len(e.lawns[1].Zombies) != 1 {
				t.Fatal("not transferred exactly once")
			}
			r := e.lawns[1].Zombies[0]
			if r.Kind != kind || r.Row != 3 || r.Hops != 8 || r.HP != Zombies[kind].HP || r.Armor != Zombies[kind].Armor || r.X != Cols*Cell || r.Slow != 0 || r.Jumped || r.Summoned || r.ID == z.ID {
				t.Fatalf("bad revival: %+v", r)
			}
			e.lawns[1].Zombies[0].HP = 0
			e.step()
			if len(e.lawns[0].Zombies) != 1 || e.lawns[0].Zombies[0].Hops != 9 || e.lawns[0].Kills != 1 || e.lawns[1].Kills != 1 {
				t.Fatal("repeat relay lost a zombie")
			}
		})
	}
}

func TestReviewAllPlantKillPathsRelay(t *testing.T) {
	for _, kind := range []PlantKind{Peashooter, Snowpea, Repeater, Cherry, Pepper, Chomper, Mine, Squash} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			e := reviewEngine(t)
			e.lawns[0].Grid[2][2] = Plant{Kind: kind, HP: Plants[kind].HP}
			z := e.makeZombie(Normal, 2, 0)
			z.HP, z.X = 1, 2250
			e.lawns[0].Zombies = append(e.lawns[0].Zombies, z)
			e.step()
			if e.lawns[0].Kills != 1 || e.lawns[1].Received != 1 {
				t.Fatalf("plant %d kill did not relay", kind)
			}
		})
	}
}

func TestReviewSimultaneousLossAndSnapshotOwnership(t *testing.T) {
	e := reviewEngine(t)
	for i := range e.lawns {
		z := e.makeZombie(Normal, 0, 0)
		z.X = 1
		e.lawns[i].Zombies = append(e.lawns[i].Zombies, z)
	}
	v := e.View("a").(Snapshot)
	v.Players[0].Zombies[0].X = 9000
	e.step()
	if !e.finished || !e.draw || e.winner != "" {
		t.Fatal("simultaneous breach is not a draw or snapshot aliases engine")
	}
}

func TestReviewPlacementRejectsWithoutMutation(t *testing.T) {
	e := reviewEngine(t)
	for _, a := range []Action{{Type: "plant", Row: -1, Plant: Sunflower}, {Type: "plant", Col: Cols, Plant: Sunflower}, {Type: "plant", Plant: 99}, {Type: "plant", Plant: Repeater}, {Type: "shovel"}} {
		before, _ := json.Marshal(e.snapshot())
		raw, _ := json.Marshal(a)
		if _, err := e.Apply("a", raw); err == nil {
			t.Fatalf("accepted %+v", a)
		}
		after, _ := json.Marshal(e.snapshot())
		if string(before) != string(after) {
			t.Fatal("rejected action changed state")
		}
	}
}

func TestReviewBotsRemainBoundedAndFinish(t *testing.T) {
	for seed := int64(1); seed <= 12; seed++ {
		e := reviewEngine(t)
		e.nextWave = 150
		e.rng = rand.New(rand.NewSource(seed))
		for n := 1; n <= 18000 && !e.finished; n++ {
			e.Tick(e.startAt.Add(time.Duration(n)*Step), []string{"a", "b"})
			if e.totalZombies() > MaxZombies {
				t.Fatal("zombie cap exceeded")
			}
			for _, l := range e.lawns {
				if len(l.Shots) > MaxShots || len(l.Effects) > 32 || l.Sun < 0 || l.Sun > 9999 {
					t.Fatal("resource bounds exceeded")
				}
			}
		}
		if !e.finished {
			t.Fatalf("seed %d still ongoing after 30 simulated minutes", seed)
		}
	}
}

func BenchmarkReviewGardenTick(b *testing.B) {
	e, _ := NewEngine([]table.Player{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}})
	e.nextWave = 1000000000
	for i := 0; i < MaxZombies/2; i++ {
		for seat := range e.lawns {
			z := e.makeZombie(Bucket, i%Rows, 0)
			z.X = 8500
			e.lawns[seat].Zombies = append(e.lawns[seat].Zombies, z)
		}
	}
	b.ReportAllocs()
	for range b.N {
		for seat := range e.lawns {
			for i := range e.lawns[seat].Zombies {
				e.lawns[seat].Zombies[i].X = 8500
			}
		}
		e.step()
	}
}
