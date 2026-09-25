package gomoku

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/17hwliao/table-card-independent/internal/table"
)

const BoardSize = 15

const (
	Empty uint8 = iota
	Black
	White
)

var (
	ErrPlayers = errors.New("五子棋需要 2 名玩家")
	ErrTurn    = errors.New("当前不是该玩家的回合")
	ErrMove    = errors.New("棋子位置无效或已经被占用")
)

type Snapshot struct {
	Board      [BoardSize][BoardSize]uint8 `json:"board"`
	Turn       uint8                       `json:"turn"`
	TurnPlayer string                      `json:"turnPlayer"`
	LastMove   *Move                       `json:"lastMove,omitempty"`
	Winner     string                      `json:"winner,omitempty"`
	Draw       bool                        `json:"draw"`
	Moves      int                         `json:"moves"`
}

type Move struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Game struct {
	mu      sync.RWMutex
	players []table.Player
	board   [BoardSize][BoardSize]uint8
	turn    uint8
	last    *Move
	winner  string
	draw    bool
	moves   int
}

func New(players []table.Player) (*Game, error) {
	if len(players) != 2 || players[0].ID == "" || players[1].ID == "" || players[0].ID == players[1].ID {
		return nil, ErrPlayers
	}
	return &Game{players: append([]table.Player(nil), players...), turn: Black}, nil
}

func (g *Game) Snapshot() Snapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()
	view := Snapshot{Board: g.board, Turn: g.turn, LastMove: g.last, Winner: g.winner, Draw: g.draw, Moves: g.moves}
	if g.winner == "" && !g.draw {
		view.TurnPlayer = g.players[int(g.turn)-1].ID
	}
	if g.last != nil {
		move := *g.last
		view.LastMove = &move
	}
	return view
}

func (g *Game) Place(playerID string, x, y int) (Snapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.winner != "" || g.draw {
		return Snapshot{}, errors.New("对局已经结束")
	}
	seat := int(g.turn) - 1
	if g.players[seat].ID != playerID {
		return Snapshot{}, ErrTurn
	}
	if x < 0 || x >= BoardSize || y < 0 || y >= BoardSize || g.board[y][x] != Empty {
		return Snapshot{}, ErrMove
	}
	g.board[y][x] = g.turn
	g.last = &Move{X: x, Y: y}
	g.moves++
	if hasFive(g.board, x, y, g.turn) {
		g.winner = playerID
	} else if g.moves == BoardSize*BoardSize {
		g.draw = true
	} else if g.turn == Black {
		g.turn = White
	} else {
		g.turn = Black
	}
	return g.snapshotLocked(), nil
}

func (g *Game) snapshotLocked() Snapshot {
	view := Snapshot{Board: g.board, Turn: g.turn, Winner: g.winner, Draw: g.draw, Moves: g.moves}
	if g.winner == "" && !g.draw {
		view.TurnPlayer = g.players[int(g.turn)-1].ID
	}
	if g.last != nil {
		move := *g.last
		view.LastMove = &move
	}
	return view
}

func hasFive(board [BoardSize][BoardSize]uint8, x, y int, color uint8) bool {
	for _, direction := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
		count := 1
		for _, sign := range []int{-1, 1} {
			for step := 1; ; step++ {
				nx, ny := x+direction[0]*step*sign, y+direction[1]*step*sign
				if nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize || board[ny][nx] != color {
					break
				}
				count++
			}
		}
		if count >= 5 {
			return true
		}
	}
	return false
}

type Engine struct{ game *Game }

func NewEngine(players []table.Player) (*Engine, error) {
	game, err := New(players)
	if err != nil {
		return nil, err
	}
	return &Engine{game: game}, nil
}

func (e *Engine) Mode() table.Mode { return table.GomokuMode }
func (e *Engine) View(string) any  { return e.game.Snapshot() }
func (e *Engine) Apply(playerID string, payload json.RawMessage) (any, error) {
	var action struct {
		Type string `json:"type"`
		X    int    `json:"x"`
		Y    int    `json:"y"`
	}
	if err := json.Unmarshal(payload, &action); err != nil {
		return nil, err
	}
	if action.Type != "place" {
		return nil, errors.New("五子棋操作类型必须是 place")
	}
	return e.game.Place(playerID, action.X, action.Y)
}
