package mahjong

import (
	"encoding/json"
	"errors"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/17hwliao/table-card-independent/internal/table"
)

const Players = 4

type Tile struct {
	Suit uint8 `json:"suit"`
	Rank uint8 `json:"rank"`
}
type Meld struct {
	Kind  string `json:"kind"`
	Tiles []Tile `json:"tiles"`
	From  int    `json:"from"`
}
type PublicPlayer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Ready       bool   `json:"ready"`
	Active      bool   `json:"active"`
	MissingSuit int    `json:"missingSuit"`
	Discards    []Tile `json:"discards"`
	Melds       []Meld `json:"melds"`
	Score       int    `json:"score"`
	Winner      bool   `json:"winner"`
	HandCount   int    `json:"handCount"`
}
type Snapshot struct {
	Players       []PublicPlayer `json:"players"`
	Hand          []Tile         `json:"hand"`
	Turn          int            `json:"turn"`
	TurnPlayer    string         `json:"turnPlayer"`
	Phase         string         `json:"phase"`
	Wall          int            `json:"wall"`
	HasDrawn      bool           `json:"hasDrawn"`
	CanHu         bool           `json:"canHu"`
	ExchangeReady bool           `json:"exchangeReady"`
	CanAddGang    bool           `json:"canAddGang"`
	CanGang       bool           `json:"canGang"`
	Round         int            `json:"round"`
	Dealer        int            `json:"dealer"`
	NextDealer    int            `json:"nextDealer"`
	LastDiscard   *Tile          `json:"lastDiscard,omitempty"`
	Discarder     int            `json:"discarder"`
	ClaimOptions  []string       `json:"claimOptions"`
	Winners       []int          `json:"winners"`
	Finished      bool           `json:"finished"`
	Message       string         `json:"message"`
	Options       Options        `json:"options"`
	LastWinFan    int            `json:"lastWinFan"`
}
type gangPayment struct {
	Payer    int
	Receiver int
	Amount   int
}
type gangUpgrade struct {
	Seat      int
	MeldIndex int
	Tile      Tile
}
type Game struct {
	mu             sync.RWMutex
	players        [Players]table.Player
	hands          [Players][]Tile
	discards       [Players][]Tile
	melds          [Players][]Meld
	scores         [Players]int
	missing        [Players]int
	ready          [Players]bool
	active         [Players]bool
	winners        []int
	wall           []Tile
	dealer         int
	round          int
	nextDealer     int
	turn           int
	hasDrawn       bool
	phase          string
	discarder      int
	pending        *Tile
	options        [Players][]string
	responses      [Players]string
	message        string
	finished       bool
	gangPayments   []gangPayment
	exchange       [Players][]Tile
	exchangeChosen [Players]bool
	pendingGang    *gangUpgrade
	optionsConfig  Options
	gangDraw       bool
	gangDiscard    bool
	discardCount   int
	lastWinFan     int
	lastDiscard    *Tile
	lastDiscarder  int
}

func New(players []table.Player) (*Game, error) {
	return NewWithOptions(players, DefaultOptions())
}

func NewWithOptions(players []table.Player, options Options) (*Game, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if len(players) != Players {
		return nil, errors.New("四川麻将需要 4 名玩家")
	}
	g := &Game{dealer: 0, round: 1, nextDealer: 0, turn: 0, phase: "exchange", discarder: -1, optionsConfig: options}
	g.dealer = rand.Intn(Players)
	g.nextDealer = g.dealer
	identities := make(map[string]bool, Players)
	for i, p := range players {
		if p.ID == "" || identities[p.ID] {
			return nil, errors.New("玩家身份无效")
		}
		identities[p.ID] = true
		g.players[i] = p
		g.missing[i] = -1
		g.active[i] = true
	}
	g.resetHand()
	return g, nil
}

func (g *Game) resetHand() {
	g.hands = [Players][]Tile{}
	g.discards = [Players][]Tile{}
	g.melds = [Players][]Meld{}
	g.missing = [Players]int{-1, -1, -1, -1}
	g.ready = [Players]bool{}
	g.active = [Players]bool{true, true, true, true}
	g.winners = nil
	g.phase = "exchange"
	if !g.optionsConfig.ExchangeThree {
		g.phase = "missing"
	}
	g.discarder = -1
	g.pending = nil
	g.options = [Players][]string{}
	g.responses = [Players]string{}
	g.message = ""
	g.finished = false
	g.gangPayments = nil
	g.exchange = [Players][]Tile{}
	g.exchangeChosen = [Players]bool{}
	g.pendingGang = nil
	g.gangDraw = false
	g.gangDiscard = false
	g.discardCount = 0
	g.lastWinFan = 0
	g.lastDiscard = nil
	g.lastDiscarder = -1
	g.turn = g.dealer
	g.hasDrawn = false
	deck := make([]Tile, 0, 108)
	for suit := uint8(0); suit < 3; suit++ {
		for rank := uint8(1); rank <= 9; rank++ {
			for c := 0; c < 4; c++ {
				deck = append(deck, Tile{suit, rank})
			}
		}
	}
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	g.wall = deck
	for round := 0; round < 13; round++ {
		for seat := 0; seat < 4; seat++ {
			g.hands[seat] = append(g.hands[seat], g.take())
		}
	}
	g.hands[g.dealer] = append(g.hands[g.dealer], g.take())
}
func (g *Game) take() Tile {
	if len(g.wall) == 0 {
		return Tile{255, 0}
	}
	t := g.wall[0]
	g.wall = g.wall[1:]
	return t
}
func (g *Game) View(viewer string) Snapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.snapshot(viewer)
}
func (g *Game) snapshot(viewer string) Snapshot {
	s := Snapshot{Turn: g.turn, Phase: g.phase, Round: g.round, Dealer: g.dealer, Wall: len(g.wall), HasDrawn: g.hasDrawn, Discarder: g.discarder, Winners: append([]int(nil), g.winners...), Finished: g.finished, Message: g.message, NextDealer: g.nextDealer, Options: g.optionsConfig, LastWinFan: g.lastWinFan}
	if !g.finished && g.phase == "turn" {
		s.TurnPlayer = g.players[g.turn].ID
	}
	if g.lastDiscard != nil {
		t := *g.lastDiscard
		s.LastDiscard = &t
		s.Discarder = g.lastDiscarder
	}
	if g.pending != nil {
		t := *g.pending
		s.LastDiscard = &t
		s.Discarder = g.discarder
	}
	for i, p := range g.players {
		missing := g.missing[i]
		if g.phase == "missing" && p.ID != viewer {
			missing = -1
		}
		melds := append([]Meld(nil), g.melds[i]...)
		for j := range melds {
			melds[j].Tiles = append([]Tile(nil), melds[j].Tiles...)
		}
		s.Players = append(s.Players, PublicPlayer{ID: p.ID, Name: p.Name, Ready: g.ready[i], Active: g.active[i], MissingSuit: missing, Discards: append([]Tile(nil), g.discards[i]...), Melds: melds, Score: g.scores[i], Winner: contains(g.winners, i), HandCount: len(g.hands[i])})
		if p.ID == viewer {
			s.Hand = append([]Tile(nil), g.hands[i]...)
			sort.SliceStable(s.Hand, func(a, b int) bool {
				if s.Hand[a].Suit != s.Hand[b].Suit {
					return s.Hand[a].Suit < s.Hand[b].Suit
				}
				return s.Hand[a].Rank < s.Hand[b].Rank
			})
			if g.responses[i] == "" {
				s.ClaimOptions = append([]string(nil), g.options[i]...)
			}
			s.CanHu = g.phase == "turn" && i == g.turn && g.hasDrawn && g.canSelfWin(i)
			s.ExchangeReady = g.exchangeChosen[i]
			s.CanAddGang = g.phase == "turn" && i == g.turn && g.hasDrawn && len(g.wall) > 0 && g.canAddGang(i)
			if g.phase == "turn" && i == g.turn && g.hasDrawn && len(g.wall) > 0 {
				for _, tile := range g.hands[i] {
					if int(tile.Suit) != g.missing[i] && countTile(g.hands[i], tile) == 4 {
						s.CanGang = true
						break
					}
				}
			}
		}
	}
	return s
}
func (g *Game) Apply(id string, payload json.RawMessage) (Snapshot, error) {
	var a struct {
		Type  string `json:"type"`
		Suit  uint8  `json:"suit"`
		Rank  uint8  `json:"rank"`
		Claim string `json:"claim"`
		Tiles []Tile `json:"tiles"`
	}
	if err := json.Unmarshal(payload, &a); err != nil {
		return Snapshot{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	seat := -1
	for i, p := range g.players {
		if p.ID == id {
			seat = i
			break
		}
	}
	if seat < 0 {
		return Snapshot{}, errors.New("玩家不在牌局中")
	}
	if g.finished {
		if a.Type != "next_round" {
			return Snapshot{}, errors.New("本局已经结束")
		}
		g.dealer = g.nextDealer
		g.round++
		g.resetHand()
		return g.snapshot(id), nil
	}
	switch a.Type {
	case "exchange":
		if g.phase != "exchange" || g.exchangeChosen[seat] || len(a.Tiles) != 3 {
			return Snapshot{}, errors.New("换三张阶段需要选择三张同花色牌")
		}
		suit := a.Tiles[0].Suit
		if suit > 2 {
			return Snapshot{}, errors.New("换牌花色无效")
		}
		seen := map[Tile]int{}
		for _, t := range a.Tiles {
			if t.Suit != suit || t.Rank < 1 || t.Rank > 9 {
				return Snapshot{}, errors.New("换三张必须选择同一花色的三张牌")
			}
			seen[t]++
		}
		for t, n := range seen {
			if countTile(g.hands[seat], t) < n {
				return Snapshot{}, errors.New("所选换牌不在手牌中")
			}
		}
		g.exchange[seat] = append([]Tile(nil), a.Tiles...)
		g.exchangeChosen[seat] = true
		if allTrue(g.exchangeChosen[:]) {
			var incoming [Players][]Tile
			for i := 0; i < Players; i++ {
				for _, t := range g.exchange[i] {
					removeTile(&g.hands[i], t)
				}
				from := (i + Players - 1) % Players
				incoming[i] = append([]Tile(nil), g.exchange[from]...)
			}
			for i := 0; i < Players; i++ {
				g.hands[i] = append(g.hands[i], incoming[i]...)
			}
			g.phase = "missing"
			g.message = "换三张完成，请同时选择缺门"
		}
	case "missing":
		if g.phase != "missing" || g.ready[seat] {
			return Snapshot{}, errors.New("当前不能选择缺门")
		}
		if a.Suit > 2 {
			return Snapshot{}, errors.New("缺门花色无效")
		}
		g.missing[seat] = int(a.Suit)
		g.ready[seat] = true
		if allTrue(g.ready[:]) {
			g.phase = "turn"
			g.turn = g.dealer
			g.hasDrawn = true
			g.message = "请庄家先打出一张牌"
		}
	case "draw":
		if g.phase != "turn" || seat != g.turn || g.hasDrawn {
			return Snapshot{}, errors.New("当前不能摸牌")
		}
		if len(g.wall) == 0 {
			g.endRound()
			break
		}
		g.hands[seat] = append(g.hands[seat], g.take())
		g.gangDraw = false
		g.hasDrawn = true
	case "discard":
		if g.phase != "turn" || seat != g.turn || !g.hasDrawn {
			return Snapshot{}, errors.New("请在自己的回合摸牌后出牌")
		}
		tile := Tile{a.Suit, a.Rank}
		if a.Suit > 2 || a.Rank < 1 || a.Rank > 9 || countTile(g.hands[seat], tile) == 0 {
			return Snapshot{}, errors.New("手牌中没有这张牌")
		}
		if hasSuit(g.hands[seat], g.missing[seat]) && int(tile.Suit) != g.missing[seat] {
			return Snapshot{}, errors.New("手里还有缺门牌，必须先打完缺门")
		}
		removeTile(&g.hands[seat], tile)
		g.discards[seat] = append(g.discards[seat], tile)
		g.discardCount++
		g.lastDiscard = &tile
		g.lastDiscarder = seat
		g.gangDiscard = g.gangDraw
		g.gangDraw = false
		g.pending = &tile
		g.discarder = seat
		g.hasDrawn = false
		g.startClaims(seat, tile)
	case "claim":
		if (g.phase != "claim" && g.phase != "rob_gang") || seat == g.discarder || !g.active[seat] || g.responses[seat] != "" {
			return Snapshot{}, errors.New("当前没有可响应的弃牌")
		}
		claim := a.Claim
		if claim == "" {
			claim = "pass"
		}
		if claim != "pass" && !containsString(g.options[seat], claim) {
			return Snapshot{}, errors.New("该玩家不能执行此操作")
		}
		g.responses[seat] = claim
		g.resolveClaims()
	case "gang":
		if g.phase != "turn" || seat != g.turn || !g.hasDrawn || len(g.wall) == 0 {
			return Snapshot{}, errors.New("当前不能杠牌")
		}
		tile := Tile{a.Suit, a.Rank}
		if a.Suit > 2 || a.Rank < 1 || a.Rank > 9 || int(tile.Suit) == g.missing[seat] || !removeN(&g.hands[seat], tile, 4) {
			return Snapshot{}, errors.New("暗杠需要手中有四张相同牌")
		}
		g.melds[seat] = append(g.melds[seat], Meld{Kind: "angang", Tiles: []Tile{tile, tile, tile, tile}, From: seat})
		for other := 0; other < 4; other++ {
			if other != seat && g.active[other] {
				g.applyGangPayment(other, seat, 2)
			}
		}
		if len(g.wall) == 0 {
			g.endRound()
			break
		}
		g.hands[seat] = append(g.hands[seat], g.take())
		g.gangDraw = true
		g.message = "暗杠完成，已补摸一张牌"
	case "add_gang":
		if g.phase != "turn" || seat != g.turn || !g.hasDrawn || len(g.wall) == 0 {
			return Snapshot{}, errors.New("当前不能补杠")
		}
		tile := Tile{a.Suit, a.Rank}
		if a.Suit > 2 || a.Rank < 1 || a.Rank > 9 || int(tile.Suit) == g.missing[seat] || countTile(g.hands[seat], tile) == 0 {
			return Snapshot{}, errors.New("补杠需要打出一张已碰牌的第四张")
		}
		meldIndex := -1
		for i, meld := range g.melds[seat] {
			if meld.Kind == "peng" && len(meld.Tiles) == 3 && meld.Tiles[0] == tile {
				meldIndex = i
				break
			}
		}
		if meldIndex < 0 {
			return Snapshot{}, errors.New("没有找到可以升级的碰牌")
		}
		removeTile(&g.hands[seat], tile)
		g.pending = &tile
		g.pendingGang = &gangUpgrade{Seat: seat, MeldIndex: meldIndex, Tile: tile}
		g.discarder = seat
		g.phase = "rob_gang"
		g.options = [Players][]string{}
		g.responses = [Players]string{}
		for other := 0; other < Players; other++ {
			if other == seat || !g.active[other] {
				continue
			}
			if g.canWin(other, tile) {
				g.options[other] = []string{"hu"}
			} else {
				g.responses[other] = "pass"
			}
		}
		g.message = "补杠声明中：其他玩家可以抢杠胡"
		g.resolveClaims()
	case "hu":
		if g.phase != "turn" || seat != g.turn || !g.hasDrawn || !g.canSelfWin(seat) {
			return Snapshot{}, errors.New("当前手牌不能自摸")
		}
		g.settleWin(seat, -1, true)
		if len(g.winners) == 0 {
			g.nextDealer = seat
		}
		g.winners = append(g.winners, seat)
		g.active[seat] = false
		g.discarder = seat
		g.message = "自摸胡牌，玩家已离场"
		g.afterWin()
	default:
		return Snapshot{}, errors.New("麻将操作必须是 missing、draw、discard、claim、gang、add_gang 或 hu")
	}
	return g.snapshot(id), nil
}
func (g *Game) startClaims(discarder int, tile Tile) {
	g.phase = "claim"
	g.options = [Players][]string{}
	g.responses = [Players]string{}
	for i := 0; i < 4; i++ {
		if i == discarder || !g.active[i] {
			continue
		}
		opts := []string{}
		if g.canWin(i, tile) {
			opts = append(opts, "hu")
		}
		if len(g.wall) > 0 && g.missing[i] != int(tile.Suit) && countTile(g.hands[i], tile) >= 2 {
			opts = append(opts, "peng")
		}
		if g.missing[i] != int(tile.Suit) && countTile(g.hands[i], tile) >= 3 && len(g.wall) > 0 {
			opts = append(opts, "gang")
		}
		g.options[i] = opts
		if len(opts) == 0 {
			g.responses[i] = "pass"
		}
	}
	g.resolveClaims()
}
func (g *Game) resolveClaims() {
	if g.phase != "claim" && g.phase != "rob_gang" {
		return
	}
	for i := 0; i < 4; i++ {
		if i != g.discarder && g.active[i] && g.responses[i] == "" {
			return
		}
	}
	if g.phase == "rob_gang" {
		winners := []int{}
		for distance := 1; distance < Players; distance++ {
			seat := (g.discarder + distance) % Players
			if g.responses[seat] == "hu" {
				winners = append(winners, seat)
			}
		}
		if len(winners) > 0 {
			if len(g.winners) == 0 {
				g.nextDealer = winners[0]
			}
			for _, winner := range winners {
				g.settleWin(winner, g.discarder, false)
				g.winners = append(g.winners, winner)
				g.active[winner] = false
			}
			g.message = "抢杠胡：补杠作废，胡牌玩家离场"
			g.pending = nil
			g.pendingGang = nil
			g.afterWin()
			return
		}
		upgrade := g.pendingGang
		if upgrade == nil {
			g.finishWithoutFlow()
			return
		}
		meld := &g.melds[upgrade.Seat][upgrade.MeldIndex]
		meld.Kind = "bugang"
		meld.Tiles = append(meld.Tiles, upgrade.Tile)
		for other := 0; other < Players; other++ {
			if other != upgrade.Seat && g.active[other] {
				g.applyGangPayment(other, upgrade.Seat, 1)
			}
		}
		g.turn = upgrade.Seat
		g.phase = "turn"
		g.pending = nil
		g.pendingGang = nil
		g.hands[g.turn] = append(g.hands[g.turn], g.take())
		g.hasDrawn = true
		g.gangDraw = true
		g.message = "补杠完成，已补摸一张牌"
		return
	}
	tile := *g.pending
	hu := []int{}
	for i := 0; i < 4; i++ {
		if g.responses[i] == "hu" {
			hu = append(hu, i)
		}
	}
	if len(hu) > 0 {
		for distance := 1; len(g.winners) == 0 && distance < Players; distance++ {
			seat := (g.discarder + distance) % Players
			if contains(hu, seat) {
				g.nextDealer = seat
				break
			}
		}
		for _, winner := range hu {
			g.settleWin(winner, g.discarder, false)
			g.winners = append(g.winners, winner)
			g.active[winner] = false
		}
		g.message = "点炮胡牌：胡牌玩家已离场"
		if len(hu) > 1 {
			g.message = "一炮多响：所有胡牌玩家已离场"
		}
		g.pending = nil
		g.afterWin()
		return
	}
	best := -1
	bestClaim := ""
	for distance := 1; distance < 4; distance++ {
		i := (g.discarder + distance) % 4
		if g.responses[i] == "peng" || g.responses[i] == "gang" {
			best = i
			bestClaim = g.responses[i]
			break
		}
	}
	g.pending = nil
	if best >= 0 {
		removeTile(&g.discards[g.discarder], tile)
		g.gangDraw = false
		if bestClaim == "peng" {
			removeN(&g.hands[best], tile, 2)
			g.melds[best] = append(g.melds[best], Meld{Kind: "peng", Tiles: []Tile{tile, tile, tile}, From: g.discarder})
			g.turn = best
			g.hasDrawn = true
			g.phase = "turn"
			g.message = "碰牌成功，请打出一张牌"
		} else {
			removeN(&g.hands[best], tile, 3)
			g.melds[best] = append(g.melds[best], Meld{Kind: "minggang", Tiles: []Tile{tile, tile, tile, tile}, From: g.discarder})
			g.applyGangPayment(g.discarder, best, 1)
			g.turn = best
			g.hasDrawn = false
			g.phase = "turn"
			g.hands[best] = append(g.hands[best], g.take())
			g.hasDrawn = true
			g.gangDraw = true
			g.message = "明杠完成，已补摸一张牌"
		}
		return
	}
	g.turn = g.nextActive(g.discarder)
	if g.turn < 0 {
		g.finishWithoutFlow()
		return
	}
	if len(g.wall) == 0 {
		g.endRound()
		return
	}
	g.phase = "turn"
	g.hasDrawn = false
	g.message = "轮到下一位玩家摸牌"
}
func (g *Game) afterWin() {
	active := 0
	for i := 0; i < 4; i++ {
		if g.active[i] {
			active++
		}
	}
	if active < 2 {
		g.finishWithoutFlow()
		return
	}
	if len(g.wall) == 0 {
		g.endRound()
		return
	}
	g.turn = g.nextActive(g.discarder)
	if g.turn < 0 {
		g.endRound()
		return
	}
	g.phase = "turn"
	g.hasDrawn = false
}
func (g *Game) nextActive(from int) int {
	for d := 1; d <= 4; d++ {
		i := (from + d) % 4
		if g.active[i] {
			return i
		}
	}
	return -1
}
func (g *Game) canWin(seat int, tile Tile) bool {
	if g.missing[seat] == int(tile.Suit) {
		return false
	}
	hand := append(append([]Tile(nil), g.hands[seat]...), tile)
	for _, t := range hand {
		if g.missing[seat] == int(t.Suit) {
			return false
		}
	}
	sets := 4 - len(g.melds[seat])
	return winning(hand, sets) || sets == 4 && sevenPairs(hand)
}
func (g *Game) canSelfWin(seat int) bool {
	hand := g.hands[seat]
	for _, t := range hand {
		if int(t.Suit) == g.missing[seat] {
			return false
		}
	}
	sets := 4 - len(g.melds[seat])
	return winning(hand, sets) || sets == 4 && sevenPairs(hand)
}
func (g *Game) settleWin(winner, source int, selfDraw bool) {
	hand := append([]Tile(nil), g.hands[winner]...)
	if !selfDraw && g.pending != nil {
		hand = append(hand, *g.pending)
	}
	fan := g.winFan(winner, hand, selfDraw)
	g.lastWinFan = fan
	amount := fan * g.optionsConfig.BaseScore
	if selfDraw {
		for i := 0; i < 4; i++ {
			if i != winner && g.active[i] {
				g.scores[winner] += amount
				g.scores[i] -= amount
			}
		}
	} else if source >= 0 {
		g.scores[winner] += amount
		g.scores[source] -= amount
	}
}
func allTripletShape(hand []Tile, sets int) bool {
	if len(hand) != sets*3+2 {
		return false
	}
	counts := [27]int{}
	for _, t := range hand {
		counts[int(t.Suit)*9+int(t.Rank)-1]++
	}
	var triplets func(*[27]int, int) bool
	triplets = func(c *[27]int, left int) bool {
		first := -1
		for i, n := range c {
			if n > 0 {
				first = i
				break
			}
		}
		if first < 0 {
			return left == 0
		}
		if left == 0 || c[first] < 3 {
			return false
		}
		c[first] -= 3
		ok := triplets(c, left-1)
		c[first] += 3
		return ok
	}
	for i, n := range counts {
		if n >= 2 {
			counts[i] -= 2
			if triplets(&counts, sets) {
				return true
			}
			counts[i] += 2
		}
	}
	return false
}
func (g *Game) endRound() {
	if g.finished {
		return
	}
	g.finished = true
	g.phase = "finished"
	if len(g.winners) == 0 {
		g.nextDealer = (g.dealer + 1) % Players
	}
	for _, payment := range g.gangPayments {
		if g.active[payment.Receiver] && g.readyFan(payment.Receiver) == 0 {
			g.scores[payment.Receiver] -= payment.Amount
			g.scores[payment.Payer] += payment.Amount
		}
	}
	pigs := [Players]bool{}
	for i := 0; i < Players; i++ {
		if g.active[i] && hasSuit(g.hands[i], g.missing[i]) {
			pigs[i] = true
		}
	}
	for pig := 0; pig < Players; pig++ {
		if !pigs[pig] || !g.active[pig] {
			continue
		}
		for receiver := 0; receiver < Players; receiver++ {
			if receiver != pig && !pigs[receiver] {
				amount := g.optionsConfig.FlowerPigPenalty * g.optionsConfig.BaseScore
				g.scores[pig] -= amount
				g.scores[receiver] += amount
			}
		}
	}
	for payer := 0; payer < Players; payer++ {
		if !g.active[payer] || pigs[payer] {
			continue
		}
		if g.readyFan(payer) > 0 {
			continue
		}
		for receiver := 0; receiver < Players; receiver++ {
			if receiver == payer || !g.active[receiver] || pigs[receiver] {
				continue
			}
			fan := g.readyFan(receiver)
			if fan > 0 {
				g.scores[payer] -= fan * g.optionsConfig.BaseScore
				g.scores[receiver] += fan * g.optionsConfig.BaseScore
			}
		}
	}
	g.message = "本局结束：已完成退税、查花猪、查叫"
}
func (g *Game) finishWithoutFlow() {
	if g.finished {
		return
	}
	g.finished = true
	g.phase = "finished"
	g.message = "血战完成：本局结束"
}
func (g *Game) canAddGang(seat int) bool {
	for _, tile := range g.hands[seat] {
		if int(tile.Suit) == g.missing[seat] {
			continue
		}
		if countTile(g.hands[seat], tile) < 1 {
			continue
		}
		for _, meld := range g.melds[seat] {
			if meld.Kind == "peng" && len(meld.Tiles) == 3 && meld.Tiles[0] == tile {
				return true
			}
		}
	}
	return false
}
func (g *Game) applyGangPayment(payer, receiver, amount int) {
	if payer < 0 || payer >= Players || receiver < 0 || receiver >= Players || payer == receiver {
		return
	}
	amount *= g.optionsConfig.BaseScore
	g.scores[payer] -= amount
	g.scores[receiver] += amount
	g.gangPayments = append(g.gangPayments, gangPayment{Payer: payer, Receiver: receiver, Amount: amount})
}
func (g *Game) readyFan(seat int) int {
	hand := g.hands[seat]
	for _, t := range hand {
		if int(t.Suit) == g.missing[seat] {
			return 0
		}
	}
	max := 0
	sets := 4 - len(g.melds[seat])
	for s := 0; s < 3; s++ {
		if s == g.missing[seat] {
			continue
		}
		for rank := 1; rank <= 9; rank++ {
			tile := Tile{uint8(s), uint8(rank)}
			if g.visibleCount(seat, tile) >= 4 {
				continue
			}
			candidate := append(append([]Tile(nil), hand...), tile)
			if winning(candidate, sets) || sets == 4 && sevenPairs(candidate) {
				fan := g.capFan(patternFan(candidate, g.melds[seat], g.optionsConfig.DragonPairBonus))
				if fan > max {
					max = fan
				}
			}
		}
	}
	return max
}
func winning(hand []Tile, sets int) bool {
	if len(hand) != sets*3+2 {
		return false
	}
	counts := [27]int{}
	for _, t := range hand {
		counts[int(t.Suit)*9+int(t.Rank)-1]++
	}
	for i, n := range counts {
		if n >= 2 {
			counts[i] -= 2
			if meldable(&counts, sets) {
				return true
			}
			counts[i] += 2
		}
	}
	return false
}
func sevenPairs(hand []Tile) bool {
	if len(hand) != 14 {
		return false
	}
	counts := [27]int{}
	for _, t := range hand {
		counts[int(t.Suit)*9+int(t.Rank)-1]++
	}
	pairs := 0
	for _, n := range counts {
		if n == 2 || n == 4 {
			pairs += n / 2
		} else if n != 0 {
			return false
		}
	}
	return pairs == 7
}
func meldable(c *[27]int, sets int) bool {
	first := -1
	for i, n := range c {
		if n > 0 {
			first = i
			break
		}
	}
	if first < 0 {
		return sets == 0
	}
	if sets <= 0 {
		return false
	}
	if c[first] >= 3 {
		c[first] -= 3
		if meldable(c, sets-1) {
			c[first] += 3
			return true
		}
		c[first] += 3
	}
	suit := first / 9
	rank := first % 9
	if rank <= 6 && suit == first/9 && first+2 < 27 && (first+2)/9 == suit && c[first+1] > 0 && c[first+2] > 0 {
		c[first]--
		c[first+1]--
		c[first+2]--
		if meldable(c, sets-1) {
			c[first]++
			c[first+1]++
			c[first+2]++
			return true
		}
		c[first]++
		c[first+1]++
		c[first+2]++
	}
	return false
}
func ready(hand []Tile, missing, sets int) bool {
	for _, t := range hand {
		if int(t.Suit) == missing {
			return false
		}
	}
	for s := 0; s < 3; s++ {
		if s == missing {
			continue
		}
		for r := 1; r <= 9; r++ {
			candidate := append(append([]Tile(nil), hand...), Tile{uint8(s), uint8(r)})
			if winning(candidate, sets) || sets == 4 && sevenPairs(candidate) {
				return true
			}
		}
	}
	return false
}
func removeTile(h *[]Tile, t Tile) bool { return removeN(h, t, 1) }
func removeN(h *[]Tile, t Tile, n int) bool {
	count := 0
	for _, v := range *h {
		if v == t {
			count++
		}
	}
	if count < n {
		return false
	}
	out := (*h)[:0]
	removed := 0
	for _, v := range *h {
		if v == t && removed < n {
			removed++
			continue
		}
		out = append(out, v)
	}
	*h = out
	return true
}
func countTile(h []Tile, t Tile) int {
	n := 0
	for _, v := range h {
		if v == t {
			n++
		}
	}
	return n
}
func allTrue(v []bool) bool {
	for _, x := range v {
		if !x {
			return false
		}
	}
	return true
}
func contains(v []int, x int) bool {
	for _, n := range v {
		if n == x {
			return true
		}
	}
	return false
}
func containsString(v []string, x string) bool {
	for _, n := range v {
		if n == x {
			return true
		}
	}
	return false
}

func hasSuit(hand []Tile, suit int) bool {
	for _, tile := range hand {
		if int(tile.Suit) == suit {
			return true
		}
	}
	return false
}

type Engine struct{ game *Game }

func NewEngine(players []table.Player) (*Engine, error) {
	g, e := New(players)
	if e != nil {
		return nil, e
	}
	return &Engine{game: g}, nil
}
func (e *Engine) Mode() table.Mode { return table.MahjongMode }
func (e *Engine) Finished() bool {
	e.game.mu.RLock()
	defer e.game.mu.RUnlock()
	return e.game.finished
}
func (e *Engine) View(id string) any { return e.game.View(id) }
func (e *Engine) Apply(id string, payload json.RawMessage) (any, error) {
	return e.game.Apply(id, payload)
}

func NewEngineWithOptions(players []table.Player, options Options) (*Engine, error) {
	g, err := NewWithOptions(players, options)
	if err != nil {
		return nil, err
	}
	return &Engine{game: g}, nil
}
