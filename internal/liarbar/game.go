package liarbar

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"
)

type Rank uint8

const (
	Queen Rank = iota + 1
	King
	Ace
	Joker
)

func (r Rank) String() string {
	switch r {
	case Queen:
		return "Q"
	case King:
		return "K"
	case Ace:
		return "A"
	case Joker:
		return "Joker"
	default:
		return "?"
	}
}

type Card struct {
	ID   uint8 `json:"id"`
	Rank Rank  `json:"rank"`
}

type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Phase uint8

const (
	RoundActive Phase = iota
	GameOver
)

var (
	ErrBadPlayers = errors.New("骗子酒馆需要 4 名身份有效的玩家")
	ErrWrongTurn  = errors.New("当前不是该玩家的回合")
	ErrEliminated = errors.New("被淘汰的玩家不能操作")
	ErrBadPlay    = errors.New("每次必须打出 1 到 3 张当前手牌")
	ErrNoPending  = errors.New("当前没有可以质疑的暗牌")
	ErrNoCards    = errors.New("手牌已用完，只能质疑上一位玩家")
)

type PublicPlayer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Alive     bool   `json:"alive"`
	Shots     int    `json:"shots"`
	HandCount int    `json:"handCount"`
}

type PendingPlay struct {
	Seat    int  `json:"seat"`
	Count   int  `json:"count"`
	CanCall bool `json:"canCall"`
}

type Snapshot struct {
	Round   int            `json:"round"`
	Target  Rank           `json:"target"`
	Turn    int            `json:"turn"`
	Phase   Phase          `json:"phase"`
	Players []PublicPlayer `json:"players"`
	Pending *PendingPlay   `json:"pending,omitempty"`
	Hand    []Card         `json:"hand,omitempty"`
	Winner  string         `json:"winner,omitempty"`
}

type ChallengeResult struct {
	BlufferSeat int          `json:"blufferSeat"`
	Challenger  int          `json:"challenger"`
	Bluff       bool         `json:"bluff"`
	Shooter     int          `json:"shooter"`
	ShotNumber  int          `json:"shotNumber"`
	Eliminated  bool         `json:"eliminated"`
	Revealed    []Card       `json:"revealed"`
	Volley      []ShotResult `json:"volley,omitempty"`
	Winner      string       `json:"winner,omitempty"`
}

type ShotResult struct {
	Seat       int  `json:"seat"`
	ShotNumber int  `json:"shotNumber"`
	Eliminated bool `json:"eliminated"`
}

type play struct {
	seat  int
	cards []Card
}

type playerState struct {
	Player
	alive    bool
	shots    int
	bulletAt int
	hand     []Card
}

type Game struct {
	mu      sync.RWMutex
	players [4]playerState
	round   int
	target  Rank
	turn    int
	phase   Phase
	pending *play
	winner  string
}

func NewGame(players []Player) (*Game, error) {
	if len(players) != 4 {
		return nil, ErrBadPlayers
	}
	game := &Game{phase: RoundActive}
	seen := make(map[string]bool, 4)
	for i, player := range players {
		if player.ID == "" || player.Name == "" || seen[player.ID] {
			return nil, ErrBadPlayers
		}
		seen[player.ID] = true
		game.players[i] = playerState{Player: player, alive: true}
		bullet, err := randomInt(6)
		if err != nil {
			return nil, err
		}
		game.players[i].bulletAt = bullet + 1
	}
	if err := game.dealRound(0); err != nil {
		return nil, err
	}
	return game, nil
}

func (g *Game) Snapshot(viewerID string) Snapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()
	view := Snapshot{Round: g.round, Target: g.target, Turn: g.turn, Phase: g.phase, Winner: g.winner}
	view.Players = make([]PublicPlayer, len(g.players))
	for i, player := range g.players {
		view.Players[i] = PublicPlayer{ID: player.ID, Name: player.Name, Alive: player.alive, Shots: player.shots, HandCount: len(player.hand)}
		if player.ID == viewerID {
			view.Hand = append([]Card(nil), player.hand...)
		}
	}
	if g.pending != nil {
		view.Pending = &PendingPlay{
			Seat: g.pending.seat, Count: len(g.pending.cards),
			CanCall: g.phase == RoundActive && g.players[g.turn].ID == viewerID,
		}
	}
	return view
}

// Play hides cards from the public state until a challenge resolves them.
func (g *Game) Play(playerID string, cardIDs []uint8) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	seat, err := g.checkTurn(playerID)
	if err != nil {
		return err
	}
	if len(g.players[seat].hand) == 0 {
		return ErrNoCards
	}
	if len(cardIDs) < 1 || len(cardIDs) > 3 {
		return ErrBadPlay
	}
	selected := make([]Card, 0, len(cardIDs))
	remaining := append([]Card(nil), g.players[seat].hand...)
	for _, id := range cardIDs {
		found := -1
		for index, card := range remaining {
			if card.ID == id {
				found = index
				break
			}
		}
		if found < 0 {
			return ErrBadPlay
		}
		selected = append(selected, remaining[found])
		remaining = append(remaining[:found], remaining[found+1:]...)
	}
	g.players[seat].hand = remaining
	g.pending = &play{seat: seat, cards: selected}
	g.turn = g.nextAlive(seat)
	return nil
}

func (g *Game) Challenge(playerID string) (ChallengeResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pending == nil {
		return ChallengeResult{}, ErrNoPending
	}
	challenger, err := g.checkTurn(playerID)
	if err != nil {
		return ChallengeResult{}, err
	}
	bluff := false
	for _, card := range g.pending.cards {
		if card.Rank != g.target && card.Rank != Joker {
			bluff = true
			break
		}
	}
	result := ChallengeResult{
		BlufferSeat: g.pending.seat,
		Challenger:  challenger,
		Bluff:       bluff,
		Shooter:     -1,
		Revealed:    append([]Card(nil), g.pending.cards...),
	}
	if bluff && containsJoker(g.pending.cards) {
		for seat := range g.players {
			if seat == challenger || !g.players[seat].alive {
				continue
			}
			result.Volley = append(result.Volley, g.shoot(seat))
		}
	} else {
		shooter := challenger
		if bluff {
			shooter = g.pending.seat
		}
		shot := g.shoot(shooter)
		result.Shooter = shooter
		result.ShotNumber = shot.ShotNumber
		result.Eliminated = shot.Eliminated
		result.Volley = []ShotResult{shot}
	}
	g.pending = nil
	if g.aliveCount() <= 1 {
		g.phase = GameOver
		for _, player := range g.players {
			if player.alive {
				g.winner = player.ID
				break
			}
		}
		result.Winner = g.winner
		return result, nil
	}
	lead := (challenger + 1) % len(g.players)
	for !g.players[lead].alive {
		lead = (lead + 1) % len(g.players)
	}
	if err := g.dealRound(lead); err != nil {
		return result, err
	}
	return result, nil
}

func (g *Game) checkTurn(playerID string) (int, error) {
	if g.phase != RoundActive {
		return -1, errors.New("游戏已经结束")
	}
	if !g.players[g.turn].alive {
		return -1, ErrEliminated
	}
	for seat, player := range g.players {
		if player.ID == playerID {
			if !player.alive {
				return -1, ErrEliminated
			}
			if seat != g.turn {
				return -1, ErrWrongTurn
			}
			return seat, nil
		}
	}
	return -1, errors.New("玩家不在本局中")
}

func (g *Game) shoot(seat int) ShotResult {
	g.players[seat].shots++
	shot := ShotResult{Seat: seat, ShotNumber: g.players[seat].shots}
	if shot.ShotNumber >= g.players[seat].bulletAt {
		shot.Eliminated = true
		g.players[seat].alive = false
		g.players[seat].hand = nil
	}
	return shot
}

func containsJoker(cards []Card) bool {
	for _, card := range cards {
		if card.Rank == Joker {
			return true
		}
	}
	return false
}

func (g *Game) dealRound(lead int) error {
	deck := make([]Card, 0, 20)
	for rank := Queen; rank <= Ace; rank++ {
		for copy := 0; copy < 6; copy++ {
			deck = append(deck, Card{ID: uint8(len(deck)), Rank: rank})
		}
	}
	for copy := 0; copy < 2; copy++ {
		deck = append(deck, Card{ID: uint8(len(deck)), Rank: Joker})
	}
	if err := shuffle(deck); err != nil {
		return err
	}
	target, err := randomInt(3)
	if err != nil {
		return err
	}
	g.target = Rank(target + 1)
	g.round++
	position := 0
	for seat := range g.players {
		if g.players[seat].alive {
			g.players[seat].hand = append([]Card(nil), deck[position:position+5]...)
			position += 5
		} else {
			g.players[seat].hand = nil
		}
	}
	g.pending = nil
	g.turn = lead
	for !g.players[g.turn].alive {
		g.turn = (g.turn + 1) % len(g.players)
	}
	return nil
}

func (g *Game) nextAlive(seat int) int {
	for offset := 1; offset <= len(g.players); offset++ {
		next := (seat + offset) % len(g.players)
		if g.players[next].alive {
			return next
		}
	}
	return seat
}

func (g *Game) aliveCount() int {
	count := 0
	for _, player := range g.players {
		if player.alive {
			count++
		}
	}
	return count
}

func shuffle(deck []Card) error {
	for i := len(deck) - 1; i > 0; i-- {
		j, err := randomInt(i + 1)
		if err != nil {
			return err
		}
		deck[i], deck[j] = deck[j], deck[i]
	}
	return nil
}

func randomInt(max int) (int, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, fmt.Errorf("生成随机游戏状态失败: %w", err)
	}
	return int(value.Int64()), nil
}
