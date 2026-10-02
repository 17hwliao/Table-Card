// Package tetris implements a simultaneous survival match on independent boards.
package tetris

import (
	"encoding/json"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/17hwliao/table-card-independent/internal/table"
)

const Width, Height = 10, 20

type Point struct{ X, Y int }
type Piece struct {
	Kind     int `json:"kind"`
	Rotation int `json:"rotation"`
	X        int `json:"x"`
	Y        int `json:"y"`
}
type Board [Height][Width]uint8
type Player struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Board  Board  `json:"board"`
	Active Piece  `json:"active"`
	Next   int    `json:"next"`
	Alive  bool   `json:"alive"`
	Lines  int    `json:"lines"`
	Score  int    `json:"score"`
	Locked int    `json:"locked"`
	Place  int    `json:"place"`
}
type Snapshot struct {
	Players  []Player `json:"players"`
	Started  bool     `json:"started"`
	Winner   string   `json:"winner"`
	Draw     bool     `json:"draw"`
	Finished bool     `json:"finished"`
	StartAt  int64    `json:"startAt"`
}
type playerState struct {
	Player
	rng      *rand.Rand
	bag      [7]int
	bagIndex int
	lastFall time.Time
	lastBot  time.Time
}
type Engine struct {
	mu                      sync.RWMutex
	players                 []playerState
	startAt                 time.Time
	winner                  string
	started, draw, finished bool
}

// The seven pieces are I, O, T, S, Z, J and L. Coordinates rotate inside
// a 3x3 box, except I (4x4) and O (no rotation).
var shapes = [7][4]Point{
	{{0, 1}, {1, 1}, {2, 1}, {3, 1}},
	{{1, 0}, {2, 0}, {1, 1}, {2, 1}},
	{{1, 0}, {0, 1}, {1, 1}, {2, 1}},
	{{1, 0}, {2, 0}, {0, 1}, {1, 1}},
	{{0, 0}, {1, 0}, {1, 1}, {2, 1}},
	{{0, 0}, {0, 1}, {1, 1}, {2, 1}},
	{{2, 0}, {0, 1}, {1, 1}, {2, 1}},
}

func Cells(p Piece) [4]Point {
	if p.Kind < 1 || p.Kind > 7 {
		return [4]Point{}
	}
	cells := shapes[p.Kind-1]
	box := 3
	if p.Kind == 1 {
		box = 4
	}
	for i, c := range cells {
		if p.Kind != 2 {
			for r := 0; r < (p.Rotation%4+4)%4; r++ {
				c.X, c.Y = box-1-c.Y, c.X
			}
		}
		cells[i] = Point{c.X + p.X, c.Y + p.Y}
	}
	return cells
}

func Fits(board Board, piece Piece) bool {
	if piece.Kind < 1 || piece.Kind > 7 {
		return false
	}
	for _, c := range Cells(piece) {
		if c.X < 0 || c.X >= Width || c.Y < -4 || c.Y >= Height {
			return false
		}
		if c.Y >= 0 && board[c.Y][c.X] != 0 {
			return false
		}
	}
	return true
}

func Ghost(board Board, piece Piece) Piece {
	if !Fits(board, piece) {
		return piece
	}
	for {
		next := piece
		next.Y++
		if !Fits(board, next) {
			return piece
		}
		piece = next
	}
}

func NewEngine(players []table.Player) (*Engine, error) {
	if len(players) != 2 && len(players) != 4 {
		return nil, errors.New("俄罗斯方块只支持双人或四人对战")
	}
	e := &Engine{startAt: time.Now().Add(3 * time.Second)}
	seed := time.Now().UnixNano()
	seen := map[string]bool{}
	for _, p := range players {
		if p.ID == "" || seen[p.ID] {
			return nil, errors.New("玩家编号无效或重复")
		}
		seen[p.ID] = true
		state := playerState{Player: Player{ID: p.ID, Name: p.Name, Alive: true}, rng: rand.New(rand.NewSource(seed)), bagIndex: 7, lastFall: e.startAt, lastBot: e.startAt}
		state.Active = Piece{Kind: state.nextKind(), X: 3, Y: -1}
		state.Next = state.nextKind()
		e.players = append(e.players, state)
	}
	return e, nil
}
func (p *playerState) nextKind() int {
	if p.bagIndex == 7 {
		for i := range p.bag {
			p.bag[i] = i + 1
		}
		p.rng.Shuffle(7, func(i, j int) { p.bag[i], p.bag[j] = p.bag[j], p.bag[i] })
		p.bagIndex = 0
	}
	kind := p.bag[p.bagIndex]
	p.bagIndex++
	return kind
}
func (e *Engine) Mode() table.Mode  { return table.TetrisMode }
func (e *Engine) Finished() bool    { e.mu.RLock(); defer e.mu.RUnlock(); return e.finished }
func (e *Engine) View(_ string) any { e.mu.RLock(); defer e.mu.RUnlock(); return e.snapshot() }
func (e *Engine) snapshot() Snapshot {
	v := Snapshot{Started: e.started, Winner: e.winner, Draw: e.draw, Finished: e.finished, StartAt: e.startAt.UnixMilli(), Players: make([]Player, len(e.players))}
	for i := range e.players {
		v.Players[i] = e.players[i].Player
	}
	return v
}

func (e *Engine) Apply(id string, raw json.RawMessage) (any, error) {
	var action struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &action); err != nil {
		return nil, errors.New("无效的方块操作")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.finished {
		return nil, errors.New("比赛已结束，按 F2 再来一局")
	}
	var player *playerState
	for i := range e.players {
		if e.players[i].ID == id {
			player = &e.players[i]
			break
		}
	}
	if player == nil {
		return nil, errors.New("玩家不在比赛中")
	}
	if !player.Alive {
		return nil, errors.New("你已淘汰，可以观看剩余玩家")
	}
	now := time.Now()
	if now.Before(e.startAt) {
		return nil, errors.New("比赛准备中，请等待倒计时")
	}
	e.started = true
	piece := player.Active
	switch action.Type {
	case "left", "right":
		if action.Type == "left" {
			piece.X--
		} else {
			piece.X++
		}
		if Fits(player.Board, piece) {
			player.Active = piece
		}
	case "rotate_cw", "rotate_ccw":
		dir := 1
		if action.Type == "rotate_ccw" {
			dir = -1
		}
		player.Active = rotate(player.Board, piece, dir)
	case "soft_drop":
		piece.Y++
		if Fits(player.Board, piece) {
			player.Active = piece
			player.Score++
		} else {
			e.lock(player, now)
		}
		player.lastFall = now
	case "hard_drop":
		piece = Ghost(player.Board, piece)
		player.Score += 2 * (piece.Y - player.Active.Y)
		player.Active = piece
		e.lock(player, now)
	default:
		return nil, errors.New("不支持的方块操作")
	}
	e.resolve()
	return e.snapshot(), nil
}

// Rotation uses bounded wall/floor kicks; it never moves through settled cells.
func rotate(board Board, piece Piece, direction int) Piece {
	next := piece
	next.Rotation = (next.Rotation + direction + 4) % 4
	for _, offset := range [...]Point{{0, 0}, {-1, 0}, {1, 0}, {-2, 0}, {2, 0}, {0, -1}, {0, -2}} {
		candidate := next
		candidate.X += offset.X
		candidate.Y += offset.Y
		if Fits(board, candidate) {
			return candidate
		}
	}
	return piece
}

func clearRows(board *Board) int {
	write := Height - 1
	cleared := 0
	for read := Height - 1; read >= 0; read-- {
		full := true
		for _, value := range board[read] {
			if value == 0 {
				full = false
				break
			}
		}
		if full {
			cleared++
			continue
		}
		board[write] = board[read]
		write--
	}
	for write >= 0 {
		board[write] = [Width]uint8{}
		write--
	}
	return cleared
}
func (e *Engine) lock(p *playerState, now time.Time) {
	p.Locked++
	for _, c := range Cells(p.Active) {
		if c.Y < 0 {
			p.Alive = false
			return
		}
		p.Board[c.Y][c.X] = uint8(p.Active.Kind)
	}
	lines := clearRows(&p.Board)
	p.Score += [...]int{0, 100, 300, 500, 800}[lines] * (1 + p.Lines/10)
	p.Lines += lines
	// A settled stack touching the top edge loses, after any completed rows clear.
	for _, value := range p.Board[0] {
		if value != 0 {
			p.Alive = false
			return
		}
	}
	p.Active = Piece{Kind: p.Next, X: 3, Y: -1}
	p.Next = p.nextKind()
	p.lastFall = now
	if !Fits(p.Board, p.Active) {
		p.Alive = false
	}
}
func (e *Engine) resolve() {
	alive := 0
	last := -1
	for i := range e.players {
		if e.players[i].Alive {
			alive++
			last = i
		}
	}
	// Players eliminated in the same tick share the same placing.
	for i := range e.players {
		if !e.players[i].Alive && e.players[i].Place == 0 {
			e.players[i].Place = alive + 1
		}
	}
	if alive > 1 {
		return
	}
	e.finished = true
	if alive == 0 {
		e.draw = true
		return
	}
	e.winner = e.players[last].ID
	e.players[last].Place = 1
}

// Tick is called by the server, never by a client. Every board advances before
// resolving simultaneous eliminations, and client keyboard traffic cannot delay gravity.
func (e *Engine) Tick(now time.Time, automated []string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.finished || now.Before(e.startAt) {
		return false
	}
	changed := false
	if !e.started {
		e.started = true
		changed = true
	}
	for i := range e.players {
		p := &e.players[i]
		if !p.Alive {
			continue
		}
		bot := false
		for _, id := range automated {
			if p.ID == id {
				bot = true
				break
			}
		}
		if bot && now.Sub(p.lastBot) >= 1500*time.Millisecond {
			e.botDrop(p, now)
			p.lastBot = now
			changed = true
			continue
		}
		interval := time.Second - time.Duration(p.Lines/10)*100*time.Millisecond
		if interval < 500*time.Millisecond {
			interval = 500 * time.Millisecond
		}
		if now.Sub(p.lastFall) >= interval {
			piece := p.Active
			piece.Y++
			if Fits(p.Board, piece) {
				p.Active = piece
				p.lastFall = now
			} else {
				e.lock(p, now)
			}
			changed = true
		}
	}
	e.resolve()
	return changed
}

// Sunjiajia chooses a reachable landing by preferring clears and avoiding holes,
// tall columns and an uneven skyline. This is a deterministic heuristic, not ML.
func (e *Engine) botDrop(p *playerState, now time.Time) {
	best := Ghost(p.Board, p.Active)
	bestCost := 1 << 30
	rotated := p.Active
	for r := 0; r < 4; r++ {
		if r > 0 {
			rotated = rotate(p.Board, rotated, 1)
		}
		for target := -3; target < Width; target++ {
			candidate := rotated
			for candidate.X != target {
				next := candidate
				if next.X < target {
					next.X++
				} else {
					next.X--
				}
				if !Fits(p.Board, next) {
					break
				}
				candidate = next
			}
			if candidate.X != target {
				continue
			}
			candidate = Ghost(p.Board, candidate)
			board := p.Board
			above := false
			for _, c := range Cells(candidate) {
				if c.Y < 0 {
					above = true
					break
				}
				board[c.Y][c.X] = uint8(candidate.Kind)
			}
			if above {
				continue
			}
			cleared := clearRows(&board)
			heights := [Width]int{}
			holes, total := 0, 0
			for x := 0; x < Width; x++ {
				for y := 0; y < Height; y++ {
					if board[y][x] != 0 {
						heights[x] = Height - y
						break
					}
				}
				total += heights[x]
				for y := Height - heights[x]; y < Height; y++ {
					if board[y][x] == 0 {
						holes++
					}
				}
			}
			bump := 0
			for x := 1; x < Width; x++ {
				d := heights[x] - heights[x-1]
				if d < 0 {
					d = -d
				}
				bump += d
			}
			cost := total*5 + holes*45 + bump*4 - cleared*70
			if cost < bestCost {
				bestCost = cost
				best = candidate
			}
		}
	}
	p.Score += 2 * (best.Y - p.Active.Y)
	p.Active = best
	e.lock(p, now)
}
