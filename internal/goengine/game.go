package goengine

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/17hwliao/table-card-independent/internal/table"
)

const BoardSize = 19

const (
	Empty uint8 = iota
	Black
	White
)

var ErrPlayers = errors.New("围棋需要 2 名玩家")

type Move struct {
	X    int  `json:"x"`
	Y    int  `json:"y"`
	Pass bool `json:"pass,omitempty"`
}
type Snapshot struct {
	Board             [BoardSize][BoardSize]uint8 `json:"board"`
	Turn              uint8                       `json:"turn"`
	TurnPlayer        string                      `json:"turnPlayer"`
	LastMove          *Move                       `json:"lastMove,omitempty"`
	Winner            string                      `json:"winner,omitempty"`
	Draw              bool                        `json:"draw"`
	Finished          bool                        `json:"finished"`
	Moves             int                         `json:"moves"`
	ConsecutivePasses int                         `json:"consecutivePasses"`
	Captures          [3]int                      `json:"captures"`
	BlackScore        float64                     `json:"blackScore"`
	WhiteScore        float64                     `json:"whiteScore"`
}

type Game struct {
	mu       sync.RWMutex
	players  [2]table.Player
	board    [BoardSize][BoardSize]uint8
	history  map[[BoardSize][BoardSize]uint8]bool
	turn     uint8
	last     *Move
	captures [3]int
	moves    int
	passes   int
	finished bool
	winner   string
	draw     bool
}

func New(players []table.Player) (*Game, error) {
	if len(players) != 2 || players[0].ID == "" || players[1].ID == "" || players[0].ID == players[1].ID {
		return nil, ErrPlayers
	}
	return &Game{players: [2]table.Player{players[0], players[1]}, turn: Black, history: map[[BoardSize][BoardSize]uint8]bool{{}: true}}, nil
}

func (g *Game) Snapshot() Snapshot { g.mu.RLock(); defer g.mu.RUnlock(); return g.snapshotLocked() }

func (g *Game) Play(playerID string, x, y int, pass bool) (Snapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.finished {
		return Snapshot{}, errors.New("对局已经结束")
	}
	if g.players[int(g.turn)-1].ID != playerID {
		return Snapshot{}, errors.New("当前不是该玩家的回合")
	}
	move := Move{X: x, Y: y, Pass: pass}
	if pass {
		g.passes++
		g.last = &move
		if g.passes >= 2 {
			g.finishLocked()
		}
	} else {
		next, captured, err := g.candidate(x, y)
		if err != nil {
			return Snapshot{}, err
		}
		g.history[next] = true
		g.board = next
		g.captures[g.turn] += captured
		g.last = &move
		g.moves++
		g.passes = 0
	}
	g.turn = opponent(g.turn)
	return g.snapshotLocked(), nil
}

func (g *Game) finishLocked() { g.finished = true }

// score fields are derived from the final position and exposed through snapshotLocked.
func (g *Game) snapshotLocked() Snapshot {
	black, white := score(g.board)
	white += 7.5
	view := Snapshot{Board: g.board, Turn: g.turn, Captures: g.captures, Moves: g.moves, ConsecutivePasses: g.passes, Finished: g.finished, Winner: g.winner, Draw: g.draw, BlackScore: black, WhiteScore: white}
	if !g.finished {
		view.TurnPlayer = g.players[int(g.turn)-1].ID
	} else if g.winner != "" {
		view.Winner = g.winner
	} else if black > white {
		view.Winner = g.players[0].ID
	} else if white > black {
		view.Winner = g.players[1].ID
	} else {
		view.Draw = true
	}
	if g.last != nil {
		m := *g.last
		view.LastMove = &m
	}
	return view
}

func score(board [BoardSize][BoardSize]uint8) (float64, float64) {
	var black, white float64
	visited := [BoardSize][BoardSize]bool{}
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			switch board[y][x] {
			case Black:
				black++
			case White:
				white++
			default:
				if visited[y][x] {
					continue
				}
				region := [][2]int{}
				queue := [][2]int{{x, y}}
				visited[y][x] = true
				borders := uint8(0)
				for len(queue) > 0 {
					p := queue[0]
					queue = queue[1:]
					region = append(region, p)
					for _, d := range neighbors(p[0], p[1]) {
						nx, ny := p[0]+d[0], p[1]+d[1]
						if !inBoard(nx, ny) {
							continue
						}
						c := board[ny][nx]
						if c == Empty && !visited[ny][nx] {
							visited[ny][nx] = true
							queue = append(queue, [2]int{nx, ny})
						} else if c != Empty {
							borders |= c
						}
					}
				}
				if borders == Black {
					black += float64(len(region))
				} else if borders == White {
					white += float64(len(region))
				}
			}
		}
	}
	return black, white
}

func collect(board [BoardSize][BoardSize]uint8, x, y int) ([][2]int, int) {
	color := board[y][x]
	if color == Empty {
		return nil, 0
	}
	seen := [BoardSize][BoardSize]bool{}
	liberty := [BoardSize][BoardSize]bool{}
	group := [][2]int{}
	queue := [][2]int{{x, y}}
	seen[y][x] = true
	count := 0
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		group = append(group, p)
		for _, d := range neighbors(p[0], p[1]) {
			nx, ny := p[0]+d[0], p[1]+d[1]
			if !inBoard(nx, ny) {
				continue
			}
			if board[ny][nx] == Empty && !liberty[ny][nx] {
				liberty[ny][nx] = true
				count++
			} else if board[ny][nx] == color && !seen[ny][nx] {
				seen[ny][nx] = true
				queue = append(queue, [2]int{nx, ny})
			}
		}
	}
	return group, count
}
func neighbors(x, y int) [][2]int { return [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} }
func inBoard(x, y int) bool       { return x >= 0 && x < BoardSize && y >= 0 && y < BoardSize }
func opponent(c uint8) uint8 {
	if c == Black {
		return White
	}
	return Black
}

type Engine struct{ game *Game }

func NewEngine(players []table.Player) (*Engine, error) {
	g, e := New(players)
	if e != nil {
		return nil, e
	}
	return &Engine{game: g}, nil
}
func (e *Engine) Mode() table.Mode { return table.GoMode }
func (e *Engine) View(string) any  { return e.game.Snapshot() }
func (e *Engine) Apply(playerID string, payload json.RawMessage) (any, error) {
	var a struct {
		Type string `json:"type"`
		X    int    `json:"x"`
		Y    int    `json:"y"`
	}
	if err := json.Unmarshal(payload, &a); err != nil {
		return nil, err
	}
	if a.Type == "resign" {
		return e.resign(playerID)
	}
	if a.Type != "play" && a.Type != "pass" {
		return nil, errors.New("围棋操作类型必须是 play 或 pass")
	}
	return e.game.Play(playerID, a.X, a.Y, a.Type == "pass")
}
