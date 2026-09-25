package chinesechess

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/17hwliao/table-card-independent/internal/table"
)

const (
	Files = 9
	Ranks = 10
)

type Side uint8

const (
	NoSide Side = iota
	Red
	Black
)

type Kind uint8

const (
	Empty Kind = iota
	General
	Advisor
	Elephant
	Horse
	Chariot
	Cannon
	Soldier
)

type Piece struct {
	Side Side `json:"side"`
	Kind Kind `json:"kind"`
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Move struct {
	From Point `json:"from"`
	To   Point `json:"to"`
}

type Snapshot struct {
	Board    [Ranks][Files]Piece `json:"board"`
	Turn     Side                `json:"turn"`
	TurnID   string              `json:"turnPlayer"`
	LastMove *Move               `json:"lastMove,omitempty"`
	InCheck  bool                `json:"inCheck"`
	Winner   string              `json:"winner,omitempty"`
	Draw     bool                `json:"draw"`
	Moves    int                 `json:"moves"`
}

var (
	ErrPlayers = errors.New("中国象棋需要 2 名玩家")
	ErrTurn    = errors.New("当前不是该玩家的回合")
	ErrMove    = errors.New("棋子走法不合法")
)

type Game struct {
	mu      sync.RWMutex
	players [2]table.Player
	board   [Ranks][Files]Piece
	turn    Side
	last    *Move
	winner  string
	draw    bool
	moves   int
}

func New(players []table.Player) (*Game, error) {
	if len(players) != 2 || players[0].ID == "" || players[1].ID == "" || players[0].ID == players[1].ID {
		return nil, ErrPlayers
	}
	g := &Game{turn: Red, players: [2]table.Player{players[0], players[1]}}
	setupBoard(&g.board)
	return g, nil
}

func (g *Game) Snapshot() Snapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.snapshotLocked()
}

func (g *Game) Move(playerID string, from, to Point) (Snapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.winner != "" || g.draw {
		return Snapshot{}, errors.New("对局已经结束")
	}
	seat := int(g.turn) - 1
	if g.players[seat].ID != playerID {
		return Snapshot{}, ErrTurn
	}
	if !inside(from.X, from.Y) || !inside(to.X, to.Y) {
		return Snapshot{}, ErrMove
	}
	piece := g.board[from.Y][from.X]
	if piece.Side != g.turn || !pieceCanMove(g.board, from, to) || g.board[to.Y][to.X].Kind == General {
		return Snapshot{}, ErrMove
	}
	previous := g.board[to.Y][to.X]
	g.board[to.Y][to.X] = piece
	g.board[from.Y][from.X] = Piece{}
	if inCheck(g.board, g.turn) {
		g.board[from.Y][from.X] = piece
		g.board[to.Y][to.X] = previous
		return Snapshot{}, ErrMove
	}
	g.last = &Move{From: from, To: to}
	g.moves++
	if g.turn == Red {
		g.turn = Black
	} else {
		g.turn = Red
	}
	if !hasLegalMove(g.board, g.turn) {
		if inCheck(g.board, g.turn) {
			winnerSide := Red
			if g.turn == Red {
				winnerSide = Black
			}
			g.winner = g.players[int(winnerSide)-1].ID
		} else {
			g.draw = true
		}
	}
	return g.snapshotLocked(), nil
}

func (g *Game) snapshotLocked() Snapshot {
	view := Snapshot{Board: g.board, Turn: g.turn, InCheck: inCheck(g.board, g.turn), Winner: g.winner, Draw: g.draw, Moves: g.moves}
	if g.winner == "" && !g.draw {
		view.TurnID = g.players[int(g.turn)-1].ID
	}
	if g.last != nil {
		move := *g.last
		view.LastMove = &move
	}
	return view
}

func setupBoard(board *[Ranks][Files]Piece) {
	back := [Files]Kind{Chariot, Horse, Elephant, Advisor, General, Advisor, Elephant, Horse, Chariot}
	for x, kind := range back {
		board[0][x], board[9][x] = Piece{Side: Black, Kind: kind}, Piece{Side: Red, Kind: kind}
	}
	for _, x := range []int{1, 7} {
		board[2][x] = Piece{Side: Black, Kind: Cannon}
		board[7][x] = Piece{Side: Red, Kind: Cannon}
	}
	for _, x := range []int{0, 2, 4, 6, 8} {
		board[3][x] = Piece{Side: Black, Kind: Soldier}
		board[6][x] = Piece{Side: Red, Kind: Soldier}
	}
}

func inside(x, y int) bool { return x >= 0 && x < Files && y >= 0 && y < Ranks }

func pieceCanMove(board [Ranks][Files]Piece, from, to Point) bool {
	piece := board[from.Y][from.X]
	target := board[to.Y][to.X]
	if piece.Kind == Empty || target.Side == piece.Side {
		return false
	}
	dx, dy := to.X-from.X, to.Y-from.Y
	absX, absY := abs(dx), abs(dy)
	switch piece.Kind {
	case General:
		if target.Kind == General && dx == 0 && clearPath(board, from, to) == 0 {
			return true
		}
		return absX+absY == 1 && inPalace(piece.Side, to)
	case Advisor:
		return absX == 1 && absY == 1 && inPalace(piece.Side, to)
	case Elephant:
		if absX != 2 || absY != 2 || (piece.Side == Red && to.Y < 5) || (piece.Side == Black && to.Y > 4) {
			return false
		}
		return board[from.Y+dy/2][from.X+dx/2].Kind == Empty
	case Horse:
		if !((absX == 1 && absY == 2) || (absX == 2 && absY == 1)) {
			return false
		}
		leg := from
		if absX == 2 {
			leg.X += dx / 2
		} else {
			leg.Y += dy / 2
		}
		return board[leg.Y][leg.X].Kind == Empty
	case Chariot:
		return (dx == 0 || dy == 0) && clearPath(board, from, to) == 0
	case Cannon:
		if dx != 0 && dy != 0 {
			return false
		}
		screens := clearPath(board, from, to)
		if target.Kind == Empty {
			return screens == 0
		}
		return screens == 1
	case Soldier:
		forward := -1
		crossed := from.Y <= 4
		if piece.Side == Black {
			forward, crossed = 1, from.Y >= 5
		}
		return (dx == 0 && dy == forward) || (crossed && absX == 1 && dy == 0)
	default:
		return false
	}
}

func clearPath(board [Ranks][Files]Piece, from, to Point) int {
	dx, dy := sign(to.X-from.X), sign(to.Y-from.Y)
	count := 0
	for x, y := from.X+dx, from.Y+dy; x != to.X || y != to.Y; x, y = x+dx, y+dy {
		if board[y][x].Kind != Empty {
			count++
		}
	}
	return count
}

func inPalace(side Side, point Point) bool {
	if point.X < 3 || point.X > 5 {
		return false
	}
	if side == Red {
		return point.Y >= 7 && point.Y <= 9
	}
	return point.Y >= 0 && point.Y <= 2
}

func inCheck(board [Ranks][Files]Piece, side Side) bool {
	king := Point{-1, -1}
	for y := 0; y < Ranks; y++ {
		for x := 0; x < Files; x++ {
			piece := board[y][x]
			if piece.Side == side && piece.Kind == General {
				king = Point{x, y}
				break
			}
		}
	}
	if king.X < 0 {
		return true
	}
	opponent := Red
	if side == Red {
		opponent = Black
	}
	for y := 0; y < Ranks; y++ {
		for x := 0; x < Files; x++ {
			piece := board[y][x]
			if piece.Side == opponent && pieceCanMove(board, Point{x, y}, king) {
				return true
			}
		}
	}
	return false
}

func hasLegalMove(board [Ranks][Files]Piece, side Side) bool {
	for y := 0; y < Ranks; y++ {
		for x := 0; x < Files; x++ {
			if board[y][x].Side != side {
				continue
			}
			from := Point{x, y}
			for ty := 0; ty < Ranks; ty++ {
				for tx := 0; tx < Files; tx++ {
					to := Point{tx, ty}
					if !pieceCanMove(board, from, to) || board[ty][tx].Kind == General {
						continue
					}
					next := board
					next[ty][tx], next[y][x] = next[y][x], Piece{}
					if !inCheck(next, side) {
						return true
					}
				}
			}
		}
	}
	return false
}

func sign(value int) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

type Engine struct{ game *Game }

func NewEngine(players []table.Player) (*Engine, error) {
	game, err := New(players)
	if err != nil {
		return nil, err
	}
	return &Engine{game: game}, nil
}

func (e *Engine) Mode() table.Mode { return table.ChessMode }
func (e *Engine) View(string) any  { return e.game.Snapshot() }
func (e *Engine) Apply(playerID string, payload json.RawMessage) (any, error) {
	var action struct {
		Type  string `json:"type"`
		FromX int    `json:"fromX"`
		FromY int    `json:"fromY"`
		ToX   int    `json:"toX"`
		ToY   int    `json:"toY"`
	}
	if err := json.Unmarshal(payload, &action); err != nil {
		return nil, err
	}
	if action.Type != "move" {
		return nil, errors.New("中国象棋操作类型必须是 move")
	}
	return e.game.Move(playerID, Point{action.FromX, action.FromY}, Point{action.ToX, action.ToY})
}
