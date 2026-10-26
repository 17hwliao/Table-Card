package tetris

import (
	"testing"
	"time"
)

func TestReviewAccelerationWithoutClearsAndSoftDropClock(t *testing.T) {
	e := fixture(t, 2)
	e.advanceClock(e.startAt.Add(220 * time.Second))
	if e.level != 12 || fallInterval(e.level) != 100*time.Millisecond {
		t.Fatal("late-game acceleration missing")
	}
	previous := time.Second
	for level := 1; level <= 12; level++ {
		current := fallInterval(level)
		if current > previous || current < 100*time.Millisecond {
			t.Fatal("gravity curve not monotonic/bounded")
		}
		previous = current
	}
	e = fixture(t, 2)
	e.startAt = time.Now().Add(-time.Second)
	last := e.startAt
	e.players[0].lastFall = last
	if _, err := e.Apply("a", []byte(`{"type":"soft_drop"}`)); err != nil {
		t.Fatal(err)
	}
	if !e.players[0].lastFall.Equal(last) {
		t.Fatal("soft drop delays automatic gravity")
	}
}
