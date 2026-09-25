package internationalchess

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/17hwliao/table-card-independent/internal/table"
)

const BoardSize = 8

type Side uint8

const (
	NoSide Side = iota
	White
	Black
)

type Kind uint8

const (
	Empty Kind = iota
	Pawn
	Knight
	Bishop
	Rook
	Queen
	King
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
	From      Point `json:"from"`
	To        Point `json:"to"`
	Promotion Kind  `json:"promotion,omitempty"`
}

type Snapshot struct {
	Board      [BoardSize][BoardSize]Piece `json:"board"`
	Turn       Side                        `json:"turn"`
	TurnPlayer string                      `json:"turnPlayer"`
	LastMove   *Move                       `json:"lastMove,omitempty"`
	InCheck    bool                        `json:"inCheck"`
	Winner     string                      `json:"winner,omitempty"`
	Draw       bool                        `json:"draw"`
	Moves      int                         `json:"moves"`
	Halfmove   int                         `json:"halfmove"`
	DrawReason string                      `json:"drawReason,omitempty"`
}

var (
	ErrPlayers = errors.New("国际象棋需要 2 名玩家")
	ErrTurn    = errors.New("当前不是该玩家的回合")
	ErrMove    = errors.New("国际象棋走法不合法")
)

const (
	CastleKing  uint8 = 1
	CastleQueen uint8 = 2
)

type position struct {
	board     [BoardSize][BoardSize]Piece
	castling  [2]uint8
	enPassant *Point
}

type Game struct {
	mu         sync.RWMutex
	players    [2]table.Player
	position   position
	turn       Side
	last       *Move
	winner     string
	draw       bool
	moves      int
	halfmove   int
	positions  map[string]int
	drawReason string
}

func New(players []table.Player) (*Game, error) {
	if len(players) != 2 || players[0].ID == "" || players[1].ID == "" || players[0].ID == players[1].ID {
		return nil, ErrPlayers
	}
	g := &Game{players: [2]table.Player{players[0], players[1]}, turn: White, positions: make(map[string]int)}
	g.position.castling = [2]uint8{CastleKing | CastleQueen, CastleKing | CastleQueen}
	back := [BoardSize]Kind{Rook, Knight, Bishop, Queen, King, Bishop, Knight, Rook}
	for x, kind := range back {
		g.position.board[0][x] = Piece{Side: Black, Kind: kind}
		g.position.board[7][x] = Piece{Side: White, Kind: kind}
		g.position.board[1][x] = Piece{Side: Black, Kind: Pawn}
		g.position.board[6][x] = Piece{Side: White, Kind: Pawn}
	}
	g.positions[positionKey(g.position, g.turn)] = 1
	return g, nil
}

func (g *Game) Snapshot() Snapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.snapshotLocked()
}

func (g *Game) Move(playerID string, from, to Point, promotion Kind) (Snapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.winner != "" || g.draw {
		return Snapshot{}, errors.New("对局已经结束")
	}
	seat := int(g.turn) - 1
	if g.players[seat].ID != playerID {
		return Snapshot{}, ErrTurn
	}
	if !legalMove(g.position, g.turn, from, to, promotion) {
		return Snapshot{}, ErrMove
	}
	piece := g.position.board[from.Y][from.X]
	captured := g.position.board[to.Y][to.X]
	if piece.Kind == Pawn && from.X != to.X && captured.Kind == Empty {
		captured = Piece{Kind: Pawn}
	}
	if piece.Kind == Pawn || captured.Kind != Empty {
		g.halfmove = 0
	} else {
		g.halfmove++
	}
	g.position = applyMove(g.position, from, to, promotion)
	g.last = &Move{From: from, To: to}
	if piece.Kind == Pawn && (to.Y == 0 || to.Y == 7) {
		g.last.Promotion = g.position.board[to.Y][to.X].Kind
	}
	g.moves++
	if g.turn == White {
		g.turn = Black
	} else {
		g.turn = White
	}
	if !hasLegalMove(g.position, g.turn) {
		if inCheck(g.position.board, g.turn) {
			winner := White
			if g.turn == White {
				winner = Black
			}
			g.winner = g.players[int(winner)-1].ID
		} else {
			g.draw = true
			g.drawReason = "逼和：当前玩家没有合法走法且未被将军"
		}
	}
	if g.winner == "" && !g.draw {
		key := positionKey(g.position, g.turn)
		g.positions[key]++
		if g.positions[key] >= 3 {
			g.draw = true
			g.drawReason = "三次重复局面"
		} else if g.halfmove >= 100 {
			g.draw = true
			g.drawReason = "连续 50 回合未吃子且未走兵"
		}
	}
	return g.snapshotLocked(), nil
}

func (g *Game) snapshotLocked() Snapshot {
	view := Snapshot{
		Board: g.position.board, Turn: g.turn, InCheck: inCheck(g.position.board, g.turn),
		Winner: g.winner, Draw: g.draw, Moves: g.moves, Halfmove: g.halfmove, DrawReason: g.drawReason,
	}
	if g.winner == "" && !g.draw {
		view.TurnPlayer = g.players[int(g.turn)-1].ID
	}
	if g.last != nil {
		move := *g.last
		view.LastMove = &move
	}
	return view
}

func positionKey(pos position, turn Side) string {
	var key strings.Builder
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			piece := pos.board[y][x]
			key.WriteByte(uint8(piece.Side)*8 + uint8(piece.Kind))
		}
	}
	key.WriteByte(byte(pos.castling[0]))
	key.WriteByte(byte(pos.castling[1]))
	key.WriteByte(byte(turn))
	if pos.enPassant == nil {
		key.WriteByte(0)
	} else {
		key.WriteByte(byte(pos.enPassant.X + 1))
		key.WriteByte(byte(pos.enPassant.Y + 1))
	}
	return key.String()
}

func legalMove(pos position, side Side, from, to Point, promotion Kind) bool {
	if !inside(from) || !inside(to) || from == to {
		return false
	}
	piece := pos.board[from.Y][from.X]
	if piece.Side != side || pos.board[to.Y][to.X].Side == side || pos.board[to.Y][to.X].Kind == King {
		return false
	}
	if !canMove(pos, from, to) {
		return false
	}
	if piece.Kind == Pawn && (to.Y == 0 || to.Y == 7) && !validPromotion(promotion) {
		return false
	}
	if piece.Kind == King && abs(to.X-from.X) == 2 {
		if inCheck(pos.board, side) {
			return false
		}
		step := sign(to.X - from.X)
		for x := from.X + step; x != to.X+step; x += step {
			if isAttacked(pos.board, Point{x, from.Y}, opposite(side)) {
				return false
			}
		}
	}
	next := applyMove(pos, from, to, promotion)
	return !inCheck(next.board, side)
}

func canMove(pos position, from, to Point) bool {
	piece, target := pos.board[from.Y][from.X], pos.board[to.Y][to.X]
	dx, dy := to.X-from.X, to.Y-from.Y
	absX, absY := abs(dx), abs(dy)
	switch piece.Kind {
	case Pawn:
		direction, startRank := -1, 6
		if piece.Side == Black {
			direction, startRank = 1, 1
		}
		if dx == 0 && target.Kind == Empty && dy == direction {
			return true
		}
		if dx == 0 && target.Kind == Empty && from.Y == startRank && dy == 2*direction {
			return pos.board[from.Y+direction][from.X].Kind == Empty
		}
		if absX == 1 && dy == direction && target.Side == opposite(piece.Side) {
			return true
		}
		return absX == 1 && dy == direction && target.Kind == Empty && pos.enPassant != nil && *pos.enPassant == to
	case Knight:
		return absX == 1 && absY == 2 || absX == 2 && absY == 1
	case Bishop:
		return absX == absY && clearPath(pos.board, from, to)
	case Rook:
		return (dx == 0 || dy == 0) && clearPath(pos.board, from, to)
	case Queen:
		return (dx == 0 || dy == 0 || absX == absY) && clearPath(pos.board, from, to)
	case King:
		if absX <= 1 && absY <= 1 {
			return true
		}
		return dy == 0 && absX == 2 && canCastle(pos, piece.Side, from, to)
	default:
		return false
	}
}

func canCastle(pos position, side Side, from, to Point) bool {
	homeRank, rightsIndex := 7, int(White)-1
	if side == Black {
		homeRank, rightsIndex = 0, int(Black)-1
	}
	if from != (Point{4, homeRank}) || to.Y != homeRank {
		return false
	}
	kingSide := to.X == 6
	if !kingSide && to.X != 2 {
		return false
	}
	right := CastleQueen
	rookX := 0
	if kingSide {
		right, rookX = CastleKing, 7
	}
	if pos.castling[rightsIndex]&right == 0 || pos.board[homeRank][rookX] != (Piece{Side: side, Kind: Rook}) {
		return false
	}
	start, end := 1, 3
	if kingSide {
		start, end = 5, 6
	}
	for x := start; x <= end; x++ {
		if pos.board[homeRank][x].Kind != Empty {
			return false
		}
	}
	return true
}

func applyMove(pos position, from, to Point, promotion Kind) position {
	next := pos
	piece := next.board[from.Y][from.X]
	captured := next.board[to.Y][to.X]
	if piece.Kind == Pawn && from.X != to.X && captured.Kind == Empty {
		next.board[from.Y][to.X] = Piece{}
		captured = Piece{Kind: Pawn, Side: opposite(piece.Side)}
	}
	if piece.Kind == King && abs(to.X-from.X) == 2 {
		rookFrom, rookTo := 0, 3
		if to.X == 6 {
			rookFrom, rookTo = 7, 5
		}
		next.board[from.Y][rookTo] = next.board[from.Y][rookFrom]
		next.board[from.Y][rookFrom] = Piece{}
	}
	if piece.Kind == King {
		next.castling[int(piece.Side)-1] = 0
	}
	if piece.Kind == Rook {
		removeRookRight(&next, piece.Side, from)
	}
	if captured.Kind == Rook {
		removeRookRight(&next, captured.Side, to)
	}
	next.board[to.Y][to.X] = piece
	next.board[from.Y][from.X] = Piece{}
	next.enPassant = nil
	if piece.Kind == Pawn && abs(to.Y-from.Y) == 2 {
		next.enPassant = &Point{X: from.X, Y: (from.Y + to.Y) / 2}
	}
	if piece.Kind == Pawn && (to.Y == 0 || to.Y == 7) {
		if !validPromotion(promotion) {
			promotion = Queen
		}
		next.board[to.Y][to.X].Kind = promotion
	}
	return next
}

func removeRookRight(pos *position, side Side, square Point) {
	homeRank, index := 7, int(White)-1
	if side == Black {
		homeRank, index = 0, int(Black)-1
	}
	if square.Y != homeRank {
		return
	}
	if square.X == 0 {
		pos.castling[index] &^= CastleQueen
	} else if square.X == 7 {
		pos.castling[index] &^= CastleKing
	}
}

func inCheck(board [BoardSize][BoardSize]Piece, side Side) bool {
	king := Point{-1, -1}
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			if board[y][x] == (Piece{Side: side, Kind: King}) {
				king = Point{x, y}
				break
			}
		}
	}
	if king.X < 0 {
		return true
	}
	return isAttacked(board, king, opposite(side))
}

func isAttacked(board [BoardSize][BoardSize]Piece, target Point, by Side) bool {
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			piece := board[y][x]
			if piece.Side != by {
				continue
			}
			from := Point{x, y}
			dx, dy := target.X-x, target.Y-y
			switch piece.Kind {
			case Pawn:
				direction := -1
				if by == Black {
					direction = 1
				}
				if abs(dx) == 1 && dy == direction {
					return true
				}
			case Knight:
				if abs(dx) == 1 && abs(dy) == 2 || abs(dx) == 2 && abs(dy) == 1 {
					return true
				}
			case Bishop:
				if abs(dx) == abs(dy) && clearPath(board, from, target) {
					return true
				}
			case Rook:
				if (dx == 0 || dy == 0) && clearPath(board, from, target) {
					return true
				}
			case Queen:
				if (dx == 0 || dy == 0 || abs(dx) == abs(dy)) && clearPath(board, from, target) {
					return true
				}
			case King:
				if abs(dx) <= 1 && abs(dy) <= 1 && (dx != 0 || dy != 0) {
					return true
				}
			}
		}
	}
	return false
}

func hasLegalMove(pos position, side Side) bool {
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			if pos.board[y][x].Side != side {
				continue
			}
			from := Point{x, y}
			for ty := 0; ty < BoardSize; ty++ {
				for tx := 0; tx < BoardSize; tx++ {
					to := Point{tx, ty}
					promotion := Kind(0)
					if pos.board[y][x].Kind == Pawn && (ty == 0 || ty == 7) {
						promotion = Queen
					}
					if legalMove(pos, side, from, to, promotion) {
						return true
					}
				}
			}
		}
	}
	return false
}

func clearPath(board [BoardSize][BoardSize]Piece, from, to Point) bool {
	dx, dy := sign(to.X-from.X), sign(to.Y-from.Y)
	for x, y := from.X+dx, from.Y+dy; x != to.X || y != to.Y; x, y = x+dx, y+dy {
		if board[y][x].Kind != Empty {
			return false
		}
	}
	return true
}

func validPromotion(kind Kind) bool {
	return kind == Queen || kind == Rook || kind == Bishop || kind == Knight
}
func opposite(side Side) Side {
	if side == White {
		return Black
	}
	return White
}
func inside(point Point) bool {
	return point.X >= 0 && point.X < BoardSize && point.Y >= 0 && point.Y < BoardSize
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

func (e *Engine) Mode() table.Mode { return table.WesternChessMode }
func (e *Engine) View(string) any  { return e.game.Snapshot() }
func (e *Engine) Apply(playerID string, payload json.RawMessage) (any, error) {
	var action struct {
		Type      string `json:"type"`
		FromX     int    `json:"fromX"`
		FromY     int    `json:"fromY"`
		ToX       int    `json:"toX"`
		ToY       int    `json:"toY"`
		Promotion Kind   `json:"promotion"`
	}
	if err := json.Unmarshal(payload, &action); err != nil {
		return nil, err
	}
	if action.Type != "move" {
		return nil, errors.New("国际象棋操作类型必须是 move")
	}
	return e.game.Move(playerID, Point{action.FromX, action.FromY}, Point{action.ToX, action.ToY}, action.Promotion)
}
