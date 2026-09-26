package mahjong

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	game "github.com/17hwliao/table-card-independent/internal/mahjong"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
)

// Controller renders Sichuan Mahjong as a compact, table-oriented terminal UI.
// Selection indices always address the server-provided hand order.
type Controller struct {
	selected map[int]bool
	cursor   int
}

func New() *Controller { return &Controller{selected: make(map[int]bool)} }

func (*Controller) Mode() table.Mode { return table.MahjongMode }

func (c *Controller) Reset() { c.selected = make(map[int]bool); c.cursor = 0 }

func (c *Controller) View(s ui.Snapshot) string {
	state, ok := decode(s.Game)
	if !ok {
		return "🀄 四川麻将\n正在等待牌局状态……"
	}
	if c.selected == nil {
		c.selected = make(map[int]bool)
	}
	width := min(max(s.Width-4, 58), 116)
	compact := (s.Height > 0 && s.Height < 36) || s.Width < 90
	var b strings.Builder
	fmt.Fprintf(&b, "🀄 四川麻将 · 血战到底  第 %d 局  牌墙 %d 张  底分 %d / 封顶 %s\n", state.Round, state.Wall, state.Options.BaseScore, capText(state.Options.FanCap))
	self := seatOf(state, s.PlayerID)
	if self < 0 {
		self = 0
	}
	side := max(16, (width-32)/2)
	middle := width - 2*side
	top := c.seatPanel(state, (self+2)%4, width-2, compact)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, top))
	b.WriteByte('\n')
	center := "等待出牌"
	if state.LastDiscard != nil {
		who := "玩家"
		if state.Discarder >= 0 && state.Discarder < len(state.Players) {
			who = state.Players[state.Discarder].Name
		}
		center = who + " 打出\n" + tileCard(*state.LastDiscard, false, false)
	}
	centerHeight := 8
	if compact {
		centerHeight = 5
	}
	tableCenter := lipgloss.NewStyle().Width(middle).Height(centerHeight).Align(lipgloss.Center, lipgloss.Center).Render(center)
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, c.seatPanel(state, (self+3)%4, side, compact), tableCenter, c.seatPanel(state, (self+1)%4, side, compact)))
	b.WriteByte('\n')
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, c.seatPanel(state, self, width-2, compact)))
	b.WriteByte('\n')
	if compact {
		b.WriteString(c.compactHand(state, width))
	} else {
		b.WriteString(c.renderHand(state, width))
	}
	b.WriteByte('\n')
	fmt.Fprintf(&b, "%s · %s\n", phaseLabel(state.Phase), state.Message)
	if state.LastWinFan > 0 {
		fmt.Fprintf(&b, "最近胡牌结算：%d 番\n", state.LastWinFan)
	}
	if state.Phase == "turn" && state.TurnPlayer != s.PlayerID {
		b.WriteString("等待当前玩家出牌")
	} else {
		b.WriteString(c.controls(state))
	}
	b.WriteString(" · Esc 取消选择 · Del 离桌")
	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (c *Controller) Key(s ui.Snapshot, key string) ui.Result {
	state, ok := decode(s.Game)
	if !ok {
		return ui.Result{Handled: false}
	}
	if c.selected == nil {
		c.selected = make(map[int]bool)
	}
	k := strings.ToLower(key)
	if k == "esc" || k == "escape" {
		c.Reset()
		return ui.Result{Handled: true, Status: "已取消选牌"}
	}
	if c.cursor >= len(state.Hand) {
		c.cursor = max(0, len(state.Hand)-1)
	}
	self := seatOf(state, s.PlayerID)
	if self < 0 {
		return ui.Result{Handled: false}
	}
	switch state.Phase {
	case "exchange":
		if state.ExchangeReady {
			return ui.Result{Handled: true, Status: "已提交换牌，等待其他玩家"}
		}
		if k == "right" || k == "l" {
			c.moveCursor(len(state.Hand), 1)
			return ui.Result{Handled: true, Status: fmt.Sprintf("光标：第 %d 张", c.cursor+1)}
		}
		if k == "left" || k == "h" {
			c.moveCursor(len(state.Hand), -1)
			return ui.Result{Handled: true, Status: fmt.Sprintf("光标：第 %d 张", c.cursor+1)}
		}
		if k == "space" || k == " " {
			if c.cursor < len(state.Hand) {
				c.toggle(c.cursor)
			}
			return ui.Result{Handled: true, Status: fmt.Sprintf("已选 %d 张换牌；需选同花色 3 张", len(c.selected))}
		}
		if n, yes := keyIndex(k); yes && n < len(state.Hand) {
			c.toggle(n)
			c.cursor = n
			return ui.Result{Handled: true, Status: fmt.Sprintf("已选 %d 张换牌；需选同花色 3 张", len(c.selected))}
		}
		if isEnter(k) {
			tiles, valid := c.selectedTiles(state.Hand)
			if !valid || len(tiles) != 3 {
				return ui.Result{Handled: true, Status: "换三张需要选择同花色的 3 张牌"}
			}
			c.selected = make(map[int]bool)
			return action(map[string]any{"type": "exchange", "tiles": tiles}, "已提交换三张")
		}
	case "missing":
		if k == "1" || k == "2" || k == "3" {
			suit := uint8(k[0] - '1')
			return action(map[string]any{"type": "missing", "suit": suit}, "已选择缺门")
		}
	case "turn":
		if state.TurnPlayer != s.PlayerID {
			return ui.Result{Handled: false}
		}
		if k == "d" && !state.HasDrawn {
			return action(map[string]string{"type": "draw"}, "摸牌")
		}
		if k == "right" || k == "l" {
			c.moveCursor(len(state.Hand), 1)
			return ui.Result{Handled: true, Status: fmt.Sprintf("光标：第 %d 张", c.cursor+1)}
		}
		if k == "left" || k == "h" {
			c.moveCursor(len(state.Hand), -1)
			return ui.Result{Handled: true, Status: fmt.Sprintf("光标：第 %d 张", c.cursor+1)}
		}
		if isEnter(k) && state.HasDrawn && c.cursor < len(state.Hand) {
			t := state.Hand[c.cursor]
			return action(map[string]any{"type": "discard", "suit": t.Suit, "rank": t.Rank}, "打出 "+tileName(t))
		}
		if n, yes := keyIndex(k); yes && n < len(state.Hand) && state.HasDrawn {
			c.cursor = n
			return ui.Result{Handled: true, Status: "已选 " + tileName(state.Hand[n]) + "，按 Enter 出牌"}
		}
		if k == "z" && state.CanHu {
			return action(map[string]string{"type": "hu"}, "申報自摸")
		}
		if k == "g" && state.CanAddGang {
			for offset := 0; offset < len(state.Hand); offset++ {
				t := state.Hand[(c.cursor+offset)%len(state.Hand)]
				if int(t.Suit) == state.Players[self].MissingSuit {
					continue
				}
				if _, yes := matchingAddGang([]game.Tile{t}, state.Players[self].Melds); yes {
					return action(map[string]any{"type": "add_gang", "suit": t.Suit, "rank": t.Rank}, "申报补杠")
				}
			}
			return ui.Result{Handled: true, Status: "当前没有可补杠的牌"}
		}
		if k == "b" && state.HasDrawn && state.Wall > 0 {
			for offset := 0; offset < len(state.Hand); offset++ {
				t := state.Hand[(c.cursor+offset)%len(state.Hand)]
				if int(t.Suit) == state.Players[self].MissingSuit {
					continue
				}
				count := 0
				for _, other := range state.Hand {
					if t == other {
						count++
					}
				}
				if count == 4 {
					return action(map[string]any{"type": "gang", "suit": t.Suit, "rank": t.Rank}, "申报暗杠")
				}
			}
			return ui.Result{Handled: true, Status: "当前没有可暗杠的四张牌"}
		}
	case "claim", "rob_gang":
		if k == "z" && has(state.ClaimOptions, "hu") {
			return action(map[string]string{"type": "claim", "claim": "hu"}, "选择胡牌")
		}
		if k == "p" && has(state.ClaimOptions, "peng") {
			return action(map[string]string{"type": "claim", "claim": "peng"}, "选择碰牌")
		}
		if k == "g" && has(state.ClaimOptions, "gang") {
			return action(map[string]string{"type": "claim", "claim": "gang"}, "选择杠牌")
		}
		if (k == "n" || k == " " || k == "space") && len(state.ClaimOptions) > 0 {
			return action(map[string]string{"type": "claim", "claim": "pass"}, "选择过牌")
		}
	case "finished":
		if isEnter(k) {
			return action(map[string]string{"type": "next_round"}, "开始下一局")
		}
	}
	if k == "esc" || k == "escape" {
		c.Reset()
		return ui.Result{Handled: true, Status: "已取消选牌"}
	}
	return ui.Result{Handled: false}
}

func (c *Controller) renderSeat(s game.Snapshot, index, offset, width int) string {
	if index < 0 || index >= len(s.Players) {
		return ""
	}
	p := s.Players[index]
	wind := []string{"東", "南", "西", "北"}[(index-s.Dealer+4)%4]
	marker := "  "
	if s.Phase == "turn" && index == s.Turn {
		marker = "▶ "
	}
	if p.Winner {
		marker = "✓ "
	}
	state := "行牌中"
	if !p.Active && p.Winner {
		state = "已胡离场"
	}
	line := fmt.Sprintf("%s%s%s%s  %+d分  缺%s  弃牌%d  副露:%s  最近:%s",
		marker, wind, dealerMark(index, s.Dealer), p.Name, p.Score, suitName(p.MissingSuit),
		len(p.Discards), meldText(p.Melds), recentText(p.Discards))
	if !p.Active {
		line += "  " + state
	}
	return "  " + fit(line, width)
}

func (c *Controller) renderSelf(s game.Snapshot, self int) string {
	if self >= len(s.Players) {
		return "玩家"
	}
	p := s.Players[self]
	wind := []string{"東", "南", "西", "北"}[(self-s.Dealer+4)%4]
	return fmt.Sprintf("%s%s%s  %+d分  缺%s  副露:%s  弃牌:%s", wind, dealerMark(self, s.Dealer), p.Name, p.Score, suitName(p.MissingSuit), meldText(p.Melds), recentText(p.Discards))
}

func (c *Controller) renderHand(s game.Snapshot, width int) string {
	if len(s.Hand) == 0 {
		return "（手牌已收起）"
	}
	perRow := max(1, width/7)
	var rows []string
	for start := 0; start < len(s.Hand); start += perRow {
		var tiles []string
		for i := start; i < min(start+perRow, len(s.Hand)); i++ {
			tiles = append(tiles, tileCard(s.Hand[i], c.selected[i], i == c.cursor)+fmt.Sprintf("\n  %02d", i+1))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, tiles...))
	}
	return strings.Join(rows, "\n")
}

func (c *Controller) toggle(i int) {
	if c.selected[i] {
		delete(c.selected, i)
	} else {
		c.selected[i] = true
	}
}
func capText(fan int) string {
	if fan == 0 {
		return "不限"
	}
	return fmt.Sprint(fan)
}
func tileCard(t game.Tile, selected, cursor bool) string {
	color := lipgloss.Color("252")
	if t.Suit < 3 {
		color = lipgloss.Color([]string{"203", "81", "114"}[t.Suit])
	}
	border := lipgloss.Color("240")
	if cursor {
		border = lipgloss.Color("220")
	}
	if selected {
		border = lipgloss.Color("117")
	}
	glyph := "□"
	if t.Suit < 3 && t.Rank >= 1 && t.Rank <= 9 {
		glyph = string(rune([]int{0x1F007, 0x1F019, 0x1F010}[t.Suit] + int(t.Rank) - 1))
	}
	label := glyph + "\n" + tileName(t)
	return lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Foreground(color).Border(lipgloss.RoundedBorder()).BorderForeground(border).Render(label)
}
func (c *Controller) seatPanel(s game.Snapshot, seat, width int, compact bool) string {
	p := s.Players[seat]
	wind := []string{"东", "南", "西", "北"}[(seat-s.Dealer+4)%4]
	marker := ""
	if s.Phase == "turn" && s.Turn == seat {
		marker = "▶ "
	}
	if p.Winner {
		marker = "✓ 已胡 "
	}
	title := fmt.Sprintf("%s%s%s %s %+d分", marker, wind, dealerMark(seat, s.Dealer), p.Name, p.Score)
	details := fmt.Sprintf("缺%s · 手牌%d · %s", suitName(p.MissingSuit), p.HandCount, meldText(p.Melds))
	content := title + "\n" + details + "\n弃牌 " + recentText(p.Discards)
	if compact {
		lines := []string{title, details, "弃牌 " + recentText(p.Discards)}
		if width >= 50 {
			lines = []string{title + " · " + details, "弃牌 " + recentText(p.Discards)}
		}
		for i := range lines {
			lines[i] = fit(lines[i], width)
		}
		return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
	}
	return lipgloss.NewStyle().Width(max(12, width-2)).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Render(content)
}

func (c *Controller) controls(s game.Snapshot) string {
	switch s.Phase {
	case "exchange":
		if s.ExchangeReady {
			return "换牌已提交，等待其他玩家"
		}
		return "数字选择牌，或 ←/→ 移动、Space 选牌 · Enter 确认三张同花色换三张"
	case "missing":
		return "1 缺万 · 2 缺筒 · 3 缺条"
	case "turn":
		if s.TurnPlayer == "" {
			return "等待其他玩家行动"
		}
		if !s.HasDrawn {
			return "D 摸牌"
		}
		choices := []string{"数字键或 ←/→ 选牌", "Enter 确认出牌"}
		if s.CanHu {
			choices = append(choices, "Z 自摸")
		}
		if s.CanAddGang {
			choices = append(choices, "G 补杠")
		}
		if s.CanGang {
			choices = append(choices, "B 暗杠")
		}
		return strings.Join(choices, " · ")
	case "claim":
		return claimControls(s.ClaimOptions, false)
	case "rob_gang":
		return claimControls(s.ClaimOptions, true)
	case "finished":
		return "Enter 开始下一局"
	}
	return "等待牌局状态"
}

func claimControls(options []string, rob bool) string {
	choices := []string{}
	if has(options, "hu") {
		choices = append(choices, "Z 胡")
	}
	if has(options, "peng") {
		choices = append(choices, "P 碰")
	}
	if has(options, "gang") {
		choices = append(choices, "G 杠")
	}
	if len(options) == 0 {
		return "等待其他玩家响应"
	}
	choices = append(choices, "N 过")
	if rob {
		return "补杠响应：" + strings.Join(choices, " · ")
	}
	return "弃牌响应：" + strings.Join(choices, " · ")
}

func action(value any, status string) ui.Result {
	return ui.Result{Handled: true, Action: ui.Action(value), Status: status}
}

func decode(raw json.RawMessage) (game.Snapshot, bool) {
	var state game.Snapshot
	if len(raw) == 0 || json.Unmarshal(raw, &state) != nil || len(state.Players) != game.Players {
		return game.Snapshot{}, false
	}
	return state, true
}

func seatOf(s game.Snapshot, id string) int {
	for i, player := range s.Players {
		if player.ID == id {
			return i
		}
	}
	return -1
}

func (c *Controller) selectedTiles(hand []game.Tile) ([]game.Tile, bool) {
	indices := make([]int, 0, len(c.selected))
	for i := range c.selected {
		indices = append(indices, i)
	}
	if len(indices) != 3 {
		return nil, false
	}
	// Stable order keeps submitted selections predictable.
	for i := 0; i < len(indices); i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[j] < indices[i] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}
	for _, index := range indices {
		if index < 0 || index >= len(hand) {
			return nil, false
		}
	}
	suit := hand[indices[0]].Suit
	tiles := make([]game.Tile, 0, 3)
	for _, index := range indices {
		if hand[index].Suit != suit {
			return nil, false
		}
		tiles = append(tiles, hand[index])
	}
	return tiles, true
}

func (c *Controller) firstSelected() int {
	first := -1
	for i := range c.selected {
		if first < 0 || i < first {
			first = i
		}
	}
	return first
}

func (c *Controller) moveCursor(handSize, delta int) {
	if handSize <= 0 {
		c.cursor = 0
		return
	}
	c.cursor = (c.cursor + delta + handSize) % handSize
}

func keyIndex(k string) (int, bool) {
	if len(k) != 1 {
		return 0, false
	}
	if k[0] >= '1' && k[0] <= '9' {
		return int(k[0] - '1'), true
	}
	if k == "0" {
		return 9, true
	}
	return 0, false
}

func isEnter(k string) bool { return k == "enter" || k == "return" || k == "\r" || k == "\n" }
func matchingGang(hand []game.Tile) (int, bool) {
	for i, t := range hand {
		count := 0
		for _, other := range hand {
			if other == t {
				count++
			}
		}
		if count == 4 {
			return i, true
		}
	}
	return 0, false
}

func matchingAddGang(hand []game.Tile, melds []game.Meld) (int, bool) {
	for i, tile := range hand {
		for _, meld := range melds {
			if meld.Kind == "peng" && len(meld.Tiles) == 3 && meld.Tiles[0] == tile {
				return i, true
			}
		}
	}
	return 0, false
}

func has(options []string, option string) bool {
	for _, value := range options {
		if value == option {
			return true
		}
	}
	return false
}

func suitName(suit int) string {
	if suit < 0 || suit > 2 {
		return "未定"
	}
	return []string{"万", "筒", "条"}[suit]
}

func tileName(t game.Tile) string {
	if t.Suit > 2 || t.Rank < 1 || t.Rank > 9 {
		return "□"
	}
	return fmt.Sprintf("%d%s", t.Rank, suitName(int(t.Suit)))
}

func meldText(melds []game.Meld) string {
	if len(melds) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(melds))
	for _, meld := range melds {
		label := map[string]string{"peng": "碰", "minggang": "明杠", "angang": "暗杠", "bugang": "补杠"}[meld.Kind]
		if label == "" {
			label = meld.Kind
		}
		if len(meld.Tiles) > 0 {
			parts = append(parts, label+tileName(meld.Tiles[0]))
		}
	}
	return strings.Join(parts, " ")
}

func recentText(discards []game.Tile) string {
	if len(discards) == 0 {
		return "—"
	}
	start := max(0, len(discards)-5)
	parts := make([]string, 0, len(discards)-start)
	for _, tile := range discards[start:] {
		parts = append(parts, tileName(tile))
	}
	return strings.Join(parts, " ")
}

func dealerMark(seat, dealer int) string {
	if seat == dealer {
		return "庄"
	}
	return ""
}

func phaseLabel(phase string) string {
	return map[string]string{
		"exchange": "换三张",
		"missing":  "定缺",
		"turn":     "摸打",
		"claim":    "弃牌响应",
		"rob_gang": "抢杠响应",
		"finished": "本局结算",
	}[phase]
}

func fit(text string, width int) string {
	if width <= 0 || lipgloss.Width(text) <= width {
		return text
	}
	out := ""
	for _, r := range text {
		if lipgloss.Width(out+string(r))+1 > width {
			break
		}
		out += string(r)
	}
	return out + "…"
}

func (c *Controller) compactHand(s game.Snapshot, width int) string {
	var rows []string
	var row []string
	perRow := max(1, width/8)
	for i, tile := range s.Hand {
		marker := " "
		if c.selected[i] {
			marker = "*"
		}
		if i == c.cursor {
			marker = ">"
		}
		label := fmt.Sprintf("%s%02d:%s", marker, i+1, tileName(tile))
		style := lipgloss.NewStyle().Width(8)
		if i == c.cursor {
			style = style.Foreground(lipgloss.Color("220"))
		} else if c.selected[i] {
			style = style.Foreground(lipgloss.Color("117"))
		}
		row = append(row, style.Render(label))
		if len(row) == perRow {
			rows = append(rows, strings.Join(row, ""))
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, strings.Join(row, ""))
	}
	return strings.Join(rows, "\n")
}
