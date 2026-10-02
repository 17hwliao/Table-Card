package snake

import "testing"

func TestTurnQueueAndSelfCollision(t *testing.T) {
	g := New(1)
	if g.Turn(Left) || g.Turn(Right) {
		t.Fatal("reverse or duplicate accepted")
	}
	if !g.Turn(Up) || !g.Turn(Left) || g.Turn(Down) {
		t.Fatal("turn queue is not bounded at two")
	}
	g.Step()
	if g.direction != Up {
		t.Fatal("multiple turns consumed")
	}
	g.Step()
	if g.direction != Left {
		t.Fatal("second queued turn not consumed")
	}
	g.Turn(Down)
	g.Step()
	if !g.finished || g.reason != "碰到自身" {
		t.Fatal("current body including tail must be fatal")
	}
}
func TestGrowthWallAndFullBoard(t *testing.T) {
	g := New(2)
	g.food = Point{g.body[0].X + 1, g.body[0].Y}
	if !g.Step() || len(g.body) != 5 || g.eaten != 1 || g.occupied[g.food.Y][g.food.X] {
		t.Fatal("growth or food placement failed")
	}
	for !g.finished {
		g.Step()
	}
	if g.reason != "撞到边界" {
		t.Fatal(g.reason)
	}
	g = New(3)
	g.body = make([]Point, Width*Height)
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			g.occupied[y][x] = true
			g.body[y*Width+x] = Point{x, y}
		}
	}
	g.placeFood()
	if !g.finished || !g.won {
		t.Fatal("full board not won")
	}
}
func TestSpeedAndClearedTurns(t *testing.T) {
	g := New(4)
	g.Turn(Up)
	g.ClearTurns()
	g.Step()
	if g.direction != Right {
		t.Fatal("pause retained buffered turn")
	}
	g.eaten = 1000
	if g.Interval().Milliseconds() != 80 {
		t.Fatal("minimum interval not bounded")
	}
}
