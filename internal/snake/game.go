// Package snake contains the local single-player rules. Its owner calls Turn
// and Step sequentially; keys enqueue turns but never move the snake directly.
package snake

import (
	"math/rand"
	"time"
)

const Width, Height = 28, 18

type Point struct{ X, Y int }
type Direction uint8

const (
	Up Direction = iota
	Right
	Down
	Left
)

type Snapshot struct {
	Occupied                    [Height][Width]bool
	Head                        Point
	Food                        Point
	Direction                   Direction
	Length, Eaten, Score, Level int
	Finished, Won               bool
	Reason                      string
}
type Game struct {
	body      []Point
	occupied  [Height][Width]bool
	direction Direction
	// Two distinct turns may be buffered, but only one is consumed per step.
	turns         [2]Direction
	turnCount     int
	food          Point
	eaten         int
	finished, won bool
	reason        string
	rng           *rand.Rand
}

func New(seed int64) *Game {
	g := &Game{direction: Right, rng: rand.New(rand.NewSource(seed)), body: make([]Point, 4, Width*Height)}
	for i := range g.body {
		g.body[i] = Point{Width/2 - i, Height / 2}
		g.occupied[g.body[i].Y][g.body[i].X] = true
	}
	g.placeFood()
	return g
}
func (g *Game) Finished() bool { return g.finished }
func (g *Game) Interval() time.Duration {
	return time.Duration(max(80, 220-(g.eaten/5)*20)) * time.Millisecond
}
func (g *Game) Snapshot() Snapshot {
	return Snapshot{Occupied: g.occupied, Head: g.body[0], Food: g.food, Direction: g.direction, Length: len(g.body), Eaten: g.eaten, Score: g.eaten * 10, Level: 1 + g.eaten/5, Finished: g.finished, Won: g.won, Reason: g.reason}
}

// Turn validates against the final buffered direction, not a direction mutated
// by another key. Repeats, immediate reversals and overflow are ignored.
func (g *Game) Turn(direction Direction) bool {
	if g.finished || direction > Left || g.turnCount == len(g.turns) {
		return false
	}
	last := g.direction
	if g.turnCount > 0 {
		last = g.turns[g.turnCount-1]
	}
	if direction == last || (int(direction)+2)%4 == int(last) {
		return false
	}
	g.turns[g.turnCount] = direction
	g.turnCount++
	return true
}
func (g *Game) ClearTurns() { g.turnCount = 0 }

// Step returns true when food was eaten. A tick performs exactly one move,
// even after a slow frame; there are no catch-up bursts after pausing.
func (g *Game) Step() bool {
	if g.finished {
		return false
	}
	if g.turnCount > 0 {
		g.direction = g.turns[0]
		g.turns[0] = g.turns[1]
		g.turnCount--
	}
	next := g.body[0]
	switch g.direction {
	case Up:
		next.Y--
	case Right:
		next.X++
	case Down:
		next.Y++
	case Left:
		next.X--
	}
	if next.X < 0 || next.X >= Width || next.Y < 0 || next.Y >= Height {
		g.finished = true
		g.reason = "撞到边界"
		g.ClearTurns()
		return false
	}
	if g.occupied[next.Y][next.X] {
		g.finished = true
		g.reason = "碰到自身"
		g.ClearTurns()
		return false
	}
	ate := next == g.food
	if ate {
		g.body = append(g.body, Point{})
	} else {
		tail := g.body[len(g.body)-1]
		g.occupied[tail.Y][tail.X] = false
	}
	copy(g.body[1:], g.body[:len(g.body)-1])
	g.body[0] = next
	g.occupied[next.Y][next.X] = true
	if ate {
		g.eaten++
		g.placeFood()
	}
	return ate
}
func (g *Game) placeFood() {
	free := Width*Height - len(g.body)
	if free == 0 {
		g.finished = true
		g.won = true
		g.reason = "填满棋盘"
		g.ClearTurns()
		return
	}
	index := g.rng.Intn(free)
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			if !g.occupied[y][x] {
				if index == 0 {
					g.food = Point{x, y}
					return
				}
				index--
			}
		}
	}
}
