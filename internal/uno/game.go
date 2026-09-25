package uno

import (
	"encoding/json"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/17hwliao/table-card-independent/internal/table"
)

type Color uint8

const (
	NoColor Color = iota
	Red
	Yellow
	Blue
	Green
)

type Kind uint8

const (
	Number Kind = iota
	DrawTwo
	Skip
	Reverse
	Wild
	WildDrawFour
)

type Card struct {
	ID     int   `json:"id"`
	Color  Color `json:"color"`
	Kind   Kind  `json:"kind"`
	Number uint8 `json:"number"`
}
type PublicPlayer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Cards   int    `json:"cards"`
	Score   int    `json:"score"`
	Bot     bool   `json:"bot,omitempty"`
	SaidUNO bool   `json:"saidUNO"`
}
type Snapshot struct {
	Players          []PublicPlayer `json:"players"`
	Hand             []Card         `json:"hand"`
	Turn             int            `json:"turn"`
	TurnPlayer       string         `json:"turnPlayer"`
	Discard          Card           `json:"discard"`
	Color            Color          `json:"color"`
	Direction        int            `json:"direction"`
	DrawPenalty      int            `json:"drawPenalty"`
	PenaltyKind      Kind           `json:"penaltyKind"`
	Winner           string         `json:"winner,omitempty"`
	SeriesWinner     string         `json:"seriesWinner,omitempty"`
	Round            int            `json:"round"`
	Finished         bool           `json:"finished"`
	PendingChallenge bool           `json:"pendingChallenge"`
	ChallengeTarget  int            `json:"challengeTarget"`
	MissedUNO        int            `json:"missedUNO"`
	CanCatchUNO      bool           `json:"canCatchUNO"`
	DrewPlayable     bool           `json:"drewPlayable"`
}
type Game struct {
	mu                   sync.RWMutex
	players              []table.Player
	hands                [][]Card
	scores               []int
	draw                 []Card
	discard              []Card
	turn                 int
	direction            int
	color                Color
	penalty              int
	penaltyKind          Kind
	winner, seriesWinner string
	finished             bool
	round                int
	pendingChallenge     bool
	challengeTarget      int
	challengeColor       Color
	missedUNO            int
	saidUNO              []bool
	drewPlayable         bool
}

func New(players []table.Player) (*Game, error) {
	if len(players) < 2 || len(players) > 4 {
		return nil, errors.New("UNO 需要 2 到 4 名玩家")
	}
	seen := map[string]bool{}
	for _, p := range players {
		if p.ID == "" || seen[p.ID] {
			return nil, errors.New("玩家身份无效")
		}
		seen[p.ID] = true
	}
	g := &Game{players: append([]table.Player(nil), players...), scores: make([]int, len(players)), round: 1}
	g.startRound()
	return g, nil
}
func (g *Game) startRound() {
	deck := makeDeck()
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	g.hands = make([][]Card, len(g.players))
	g.saidUNO = make([]bool, len(g.players))
	g.draw = deck
	g.discard = nil
	g.turn = 0
	g.direction = 1
	g.penalty = 0
	g.penaltyKind = Number
	g.pendingChallenge = false
	g.missedUNO = -1
	g.finished = false
	g.drewPlayable = false
	for i := 0; i < 7; i++ {
		for p := range g.players {
			g.hands[p] = append(g.hands[p], g.takeCard())
		}
	}
	first := g.takeCard()
	for first.Kind == Wild || first.Kind == WildDrawFour {
		g.draw = append(g.draw, first)
		first = g.takeCard()
	}
	g.discard = append(g.discard, first)
	g.color = first.Color
	switch first.Kind {
	case DrawTwo:
		g.penalty = 2
		g.penaltyKind = DrawTwo
		g.turn = g.next(0)
	case Skip:
		g.turn = g.next(0)
	case Reverse:
		g.direction = -1
		g.turn = g.next(0)
	case Number:
	}
}
func makeDeck() []Card {
	deck := make([]Card, 0, 108)
	id := 0
	for c := Red; c <= Green; c++ {
		deck = append(deck, Card{ID: id, Color: c, Kind: Number, Number: 0})
		id++
		for n := uint8(1); n <= 9; n++ {
			for k := 0; k < 2; k++ {
				deck = append(deck, Card{ID: id, Color: c, Kind: Number, Number: n})
				id++
			}
		}
		for k := 0; k < 2; k++ {
			for _, kind := range []Kind{DrawTwo, Skip, Reverse} {
				deck = append(deck, Card{ID: id, Color: c, Kind: kind})
				id++
			}
		}
	}
	for k := 0; k < 4; k++ {
		deck = append(deck, Card{ID: id, Kind: Wild})
		id++
		deck = append(deck, Card{ID: id, Kind: WildDrawFour})
		id++
	}
	return deck
}
func (g *Game) takeCard() Card {
	if len(g.draw) == 0 {
		if len(g.discard) <= 1 {
			return Card{ID: -1}
		}
		top := g.discard[len(g.discard)-1]
		g.draw = append([]Card(nil), g.discard[:len(g.discard)-1]...)
		g.discard = []Card{top}
		rand.Shuffle(len(g.draw), func(i, j int) { g.draw[i], g.draw[j] = g.draw[j], g.draw[i] })
	}
	if len(g.draw) == 0 {
		return Card{ID: -1}
	}
	card := g.draw[len(g.draw)-1]
	g.draw = g.draw[:len(g.draw)-1]
	return card
}
func (g *Game) next(from int) int { return (from + g.direction + len(g.players)) % len(g.players) }
func (g *Game) View(viewerID string) Snapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.snapshotLocked(viewerID)
}
func (g *Game) snapshotLocked(viewer string) Snapshot {
	s := Snapshot{Turn: g.turn, Direction: g.direction, Color: g.color, DrawPenalty: g.penalty, PenaltyKind: g.penaltyKind, Winner: g.winner, SeriesWinner: g.seriesWinner, Round: g.round, Finished: g.finished, PendingChallenge: g.pendingChallenge, ChallengeTarget: g.challengeTarget, MissedUNO: g.missedUNO, CanCatchUNO: g.missedUNO >= 0 && g.turn == g.next(g.missedUNO), DrewPlayable: g.drewPlayable}
	if len(g.discard) > 0 {
		s.Discard = g.discard[len(g.discard)-1]
	}
	for i, p := range g.players {
		s.Players = append(s.Players, PublicPlayer{ID: p.ID, Name: p.Name, Cards: len(g.hands[i]), Score: g.scores[i], Bot: p.Bot, SaidUNO: g.saidUNO[i]})
		if p.ID == viewer {
			s.Hand = append([]Card(nil), g.hands[i]...)
		}
	}
	if !g.finished {
		s.TurnPlayer = g.players[g.turn].ID
	}
	return s
}
func (g *Game) Apply(playerID string, payload json.RawMessage) (Snapshot, error) {
	var a struct {
		Type   string `json:"type"`
		CardID int    `json:"cardId"`
		Color  Color  `json:"color"`
		UNO    bool   `json:"uno"`
		Target int    `json:"target"`
	}
	if err := json.Unmarshal(payload, &a); err != nil {
		return Snapshot{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	seat := -1
	for i, p := range g.players {
		if p.ID == playerID {
			seat = i
			break
		}
	}
	if seat < 0 {
		return Snapshot{}, errors.New("玩家不在此局")
	}
	if g.finished {
		if a.Type != "next_round" {
			return Snapshot{}, errors.New("本局已经结束")
		}
		if g.seriesWinner != "" {
			return Snapshot{}, errors.New("已达到 500 分，整场比赛结束")
		}
		g.round++
		g.winner = ""
		g.startRound()
		return g.snapshotLocked(playerID), nil
	}
	if a.Type == "catch_uno" {
		if g.missedUNO < 0 || seat != g.turn || seat == g.missedUNO || len(g.hands[g.missedUNO]) != 1 {
			return Snapshot{}, errors.New("当前没有可抓漏喊 UNO 的玩家")
		}
		for i := 0; i < 2; i++ {
			c := g.takeCard()
			if c.ID >= 0 {
				g.hands[g.missedUNO] = append(g.hands[g.missedUNO], c)
			}
		}
		g.saidUNO[g.missedUNO] = false
		g.missedUNO = -1
		return g.snapshotLocked(playerID), nil
	}
	if g.turn != seat {
		return Snapshot{}, errors.New("当前不是该玩家的回合")
	}
	if g.missedUNO >= 0 {
		g.missedUNO = -1
	}
	if a.Type == "challenge" {
		if !g.pendingChallenge || seat != g.turn {
			return Snapshot{}, errors.New("当前没有可挑战的万能 +4")
		}
		target := g.challengeTarget
		hasColor := false
		for _, c := range g.hands[target] {
			if c.Color == g.challengeColor {
				hasColor = true
				break
			}
		}
		g.pendingChallenge = false
		if hasColor {
			for i := 0; i < 4; i++ {
				c := g.takeCard()
				if c.ID >= 0 {
					g.hands[target] = append(g.hands[target], c)
				}
			}
			g.turn = seat
			g.penalty = 0
			g.penaltyKind = Number
			g.color = g.challengeColor
		} else {
			for i := 0; i < g.penalty+2; i++ {
				c := g.takeCard()
				if c.ID >= 0 {
					g.hands[seat] = append(g.hands[seat], c)
				}
			}
			g.penalty = 0
			g.turn = g.next(seat)
		}
		return g.snapshotLocked(playerID), nil
	}
	if a.Type == "draw" {
		if g.penalty > 0 {
			for i := 0; i < g.penalty; i++ {
				c := g.takeCard()
				if c.ID >= 0 {
					g.hands[seat] = append(g.hands[seat], c)
				}
			}
			g.penalty = 0
			g.penaltyKind = Number
			g.pendingChallenge = false
			g.drewPlayable = false
			g.turn = g.next(seat)
			return g.snapshotLocked(playerID), nil
		}
		if g.drewPlayable {
			return Snapshot{}, errors.New("你已经摸到可出的牌，可以出牌或结束本回合")
		}
		g.drewPlayable = false
		for attempts := 0; attempts < 108; attempts++ {
			c := g.takeCard()
			if c.ID < 0 {
				break
			}
			g.hands[seat] = append(g.hands[seat], c)
			if g.playable(c) {
				g.drewPlayable = true
				break
			}
		}
		if !g.drewPlayable {
			g.drewPlayable = true
		}
		return g.snapshotLocked(playerID), nil
	}
	if a.Type == "keep" {
		if !g.drewPlayable {
			return Snapshot{}, errors.New("请先摸牌")
		}
		g.drewPlayable = false
		g.turn = g.next(seat)
		return g.snapshotLocked(playerID), nil
	}
	if a.Type != "play" {
		return Snapshot{}, errors.New("UNO 操作必须是 play、draw、challenge 或 catch_uno")
	}
	idx := -1
	for i, c := range g.hands[seat] {
		if c.ID == a.CardID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Snapshot{}, errors.New("手牌中找不到该牌")
	}
	card := g.hands[seat][idx]
	if card.Kind == Number && card.Number == 7 && len(g.hands[seat]) > 1 && (a.Target < 0 || a.Target >= len(g.players) || a.Target == seat) {
		return Snapshot{}, errors.New("数字 7 需要选择另一名玩家交换手牌")
	}
	if !g.canPlay(card) {
		return Snapshot{}, errors.New("这张牌不能匹配当前颜色或牌面")
	}
	if g.penalty > 0 && card.Kind != DrawTwo && card.Kind != WildDrawFour {
		return Snapshot{}, errors.New("罚牌叠加中只能叠加 +2 或 +4，或摸牌承受惩罚")
	}
	if (card.Kind == Wild || card.Kind == WildDrawFour) && (a.Color < Red || a.Color > Green) {
		return Snapshot{}, errors.New("万能牌需要选择一种有效颜色")
	}
	oldColor := g.color
	g.drewPlayable = false
	g.pendingChallenge = false
	g.hands[seat] = append(g.hands[seat][:idx], g.hands[seat][idx+1:]...)
	g.discard = append(g.discard, card)
	if card.Kind == Wild || card.Kind == WildDrawFour {
		g.color = a.Color
	} else {
		g.color = card.Color
	}
	if card.Kind == Number && card.Number == 7 && len(g.hands[seat]) > 0 {
		g.hands[seat], g.hands[a.Target] = g.hands[a.Target], g.hands[seat]
	}
	if card.Kind == Number && card.Number == 0 && len(g.hands[seat]) > 0 {
		oldHands := make([][]Card, len(g.hands))
		for i := range g.hands {
			oldHands[i] = g.hands[i]
		}
		for i := range g.hands {
			g.hands[i] = oldHands[g.next(i)]
		}
	}
	g.saidUNO[seat] = len(g.hands[seat]) == 1 && a.UNO
	g.missedUNO = -1
	if len(g.hands[seat]) == 1 && !a.UNO {
		g.missedUNO = seat
	}
	if len(g.hands[seat]) == 0 {
		if card.Kind == DrawTwo || card.Kind == WildDrawFour {
			count := 2
			if card.Kind == WildDrawFour {
				count = 4
			}
			target := g.next(seat)
			for i := 0; i < count; i++ {
				c := g.takeCard()
				if c.ID >= 0 {
					g.hands[target] = append(g.hands[target], c)
				}
			}
		}
		g.finishRound(seat)
		return g.snapshotLocked(playerID), nil
	}
	switch card.Kind {
	case Number:
		g.turn = g.next(seat)
	case Skip:
		g.turn = g.next(g.next(seat))
	case Reverse:
		if len(g.players) == 2 {
			g.turn = g.next(g.next(seat))
		} else {
			g.direction = -g.direction
			g.turn = g.next(seat)
		}
	case DrawTwo:
		g.penalty += 2
		g.penaltyKind = DrawTwo
		g.turn = g.next(seat)
	case Wild:
		g.turn = g.next(seat)
	case WildDrawFour:
		g.penalty += 4
		g.penaltyKind = WildDrawFour
		g.challengeTarget = seat
		g.challengeColor = oldColor
		g.pendingChallenge = true
		g.turn = g.next(seat)
	}
	return g.snapshotLocked(playerID), nil
}
func (g *Game) playable(c Card) bool {
	return c.Kind == Wild || c.Kind == WildDrawFour || c.Color == g.color || len(g.discard) > 0 && ((c.Kind == Number && g.discard[len(g.discard)-1].Kind == Number && c.Number == g.discard[len(g.discard)-1].Number) || c.Kind != Number && c.Kind == g.discard[len(g.discard)-1].Kind)
}
func (g *Game) canPlay(c Card) bool {
	if g.penalty > 0 {
		return c.Kind == DrawTwo || c.Kind == WildDrawFour
	}
	return g.playable(c)
}
func (g *Game) finishRound(winner int) {
	sum := 0
	for i, h := range g.hands {
		if i == winner {
			continue
		}
		for _, c := range h {
			switch c.Kind {
			case Number:
				sum += int(c.Number)
			case Wild, WildDrawFour:
				sum += 50
			default:
				sum += 20
			}
		}
	}
	g.scores[winner] += sum
	g.winner = g.players[winner].ID
	g.finished = true
	for i, p := range g.players {
		if g.scores[i] >= 500 {
			g.seriesWinner = p.ID
		}
	}
}

type Engine struct{ game *Game }

func NewEngine(players []table.Player) (*Engine, error) {
	g, e := New(players)
	if e != nil {
		return nil, e
	}
	return &Engine{game: g}, nil
}
func (e *Engine) Mode() table.Mode   { return table.UNOMode }
func (e *Engine) View(id string) any { return e.game.View(id) }
func (e *Engine) Apply(id string, payload json.RawMessage) (any, error) {
	return e.game.Apply(id, payload)
}
