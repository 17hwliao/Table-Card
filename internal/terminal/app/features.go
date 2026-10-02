package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"context"
	"encoding/json"
	"fmt"
	"github.com/17hwliao/table-card-independent/internal/mahjong"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/netclient"
	"github.com/17hwliao/table-card-independent/internal/terminal/profile"
	modeui "github.com/17hwliao/table-card-independent/internal/terminal/ui"
	"github.com/17hwliao/table-card-independent/internal/uno"
	"log"
	"strings"
	"time"
)

type clockMsg time.Time
type panelMsg struct {
	text string
	err  error
}
type reconnectMsg struct {
	client  *netclient.Client
	err     error
	attempt int
}

func clockTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return clockMsg(t) })
}
func (m *Model) needsClock() bool {
	if m.page != gameScreen {
		return false
	}
	if time.Now().Unix() < m.pauseUntil {
		return true
	}
	if m.room.Mode == table.TetrisMode {
		var v struct {
			StartAt int64 `json:"startAt"`
			Started bool  `json:"started"`
		}
		_ = json.Unmarshal(m.game, &v)
		return !v.Started && time.Now().UnixMilli() < v.StartAt
	}
	return false
}

// Falling/rotating blocks are silent. Only our locks and clears produce an
// effect; opponents' realtime updates must not turn into continuous beeping.
func tetrisEffect(before, after json.RawMessage, id string) (int, bool) {
	type player struct {
		ID     string `json:"id"`
		Locked int    `json:"locked"`
		Lines  int    `json:"lines"`
	}
	var old, next struct {
		Players []player `json:"players"`
	}
	if json.Unmarshal(before, &old) != nil || json.Unmarshal(after, &next) != nil {
		return 0, false
	}
	for _, p := range next.Players {
		if p.ID == id {
			for _, previous := range old.Players {
				if previous.ID == id && p.Locked > previous.Locked {
					if p.Lines > previous.Lines {
						return 20, true
					}
					return 8, true
				}
			}
		}
	}
	return 0, false
}
func reconnect(client *netclient.Client, code, id string, attempt int) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Duration(min(attempt, 5)) * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return reconnectMsg{client, client.Connect(ctx, code, id), attempt}
	}
}
func (m *Model) currentMode() table.Mode {
	if m.page == pokemonScreen {
		return table.PokemonMode
	}
	if m.page == snakeScreen {
		return table.SnakeMode
	}
	if m.page != homeScreen {
		return m.room.Mode
	}
	if len(m.modes) > 0 {
		return m.modes[m.mode].ID
	}
	return ""
}
func (m *Model) Close() {
	if m.pokemonGame != nil && !m.pokemonLoadError {
		if err := m.pokemonGame.Save(); err != nil {
			log.Printf("宝可梦存档保存失败：%v", err)
		}
	}
	if m.api != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = m.api.Leave(ctx, m.room.Code, m.player.ID)
		cancel()
		m.api.Close()
	}
	if m.sound != nil {
		m.sound.Close()
	}
}
func (m *Model) leave() tea.Cmd {
	client, code, id := m.api, m.room.Code, m.player.ID
	m.api = nil
	m.room = table.Snapshot{}
	m.game = nil
	m.control = nil
	m.chatLog = nil
	m.chat.SetValue("")
	m.chatOn = false
	m.reconnecting = false
	m.page = homeScreen
	m.menuOn = true
	m.pauseUntil = 0
	m.status = "已离桌；进行中的座位由机器人接管"
	_ = m.sound.SetMode("")
	if client == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := client.Leave(ctx, code, id)
		client.Close()
		return actionSent{err}
	}
}
func (m *Model) rematch() tea.Cmd {
	if m.api == nil {
		return nil
	}
	client, code, id := m.api, m.room.Code, m.player.ID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return actionSent{client.Rematch(ctx, code, id)}
	}
}
func (m *Model) submit(result modeui.Result) tea.Cmd {
	if result.Status != "" {
		m.status = result.Status
	}
	if len(result.Action) == 0 || m.api == nil {
		return nil
	}
	if m.reconnecting || time.Now().Unix() < m.pauseUntil {
		m.status = "请等待连接恢复或结果公示结束"
		return nil
	}
	client := m.api
	payload := append(json.RawMessage(nil), result.Action...)
	return func() tea.Msg { return actionSent{client.Send(map[string]any{"type": "action", "action": payload})} }
}
func effectSeed(raw json.RawMessage) int {
	var v struct {
		LastDiscard *struct {
			Suit int `json:"suit"`
			Rank int `json:"rank"`
		} `json:"lastDiscard"`
	}
	_ = json.Unmarshal(raw, &v)
	if v.LastDiscard != nil {
		return v.LastDiscard.Suit*9 + v.LastDiscard.Rank - 1
	}
	return 12
}
func (m *Model) menuKey(key string) tea.Cmd {
	if m.currentMode() == table.PokemonMode {
		return m.pokemonMenuKey(key)
	}
	if m.currentMode() == table.SnakeMode {
		return m.snakeMenuKey(key)
	}
	switch key {
	case "esc", "8":
		m.menuOn = false
	case "1":
		return m.match()
	case "2", "c":
		m.bots = false
		return m.create()
	case "3", "j":
		m.editing = "join"
		return m.code.Focus()
	case "4", "b":
		m.bots = true
		return m.create()
	case "5":
		return m.statsPanel(false)
	case "6":
		return m.statsPanel(true)
	case "7", "f1", "?":
		m.overlay = rulesFor(m.currentMode())
	case "o":
		if len(m.settings[m.currentMode()]) == 0 {
			m.status = "本模式使用固定规则"
		} else {
			m.optionsOn = true
			m.optionIndex = 0
		}
	case "e":
		m.editing = "name"
		return m.name.Focus()
	case "l":
		return m.roomsPanel()
	case "left":
		m.setSeats(m.seats - 1)
	case "right":
		m.setSeats(m.seats + 1)
	case "enter":
		m.bots = true
		return m.create()
	}
	return nil
}
func (m *Model) modeLobbyView() string {
	if m.currentMode() == table.PokemonMode {
		return m.pokemonLobbyView()
	}
	if m.currentMode() == table.SnakeMode {
		return m.snakeLobbyView()
	}
	item := m.modes[m.mode]
	body := fmt.Sprintf("1  快速匹配（15秒后机器人补位）\n2  创建真人房间\n3  输入房间号加入\n4  人机练习（一键开局）\n5  排行榜\n6  我的战绩\n7  游戏规则\n8  返回游戏选择\n\n玩家：%s    座位：%d", m.name.Value(), m.seats)
	if m.editing == "name" {
		body += "\n" + m.name.View()
	} else if m.editing != "" {
		body += "\n" + m.code.View()
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 3).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, lipgloss.JoinVertical(lipgloss.Center, titleStyle.Render("牌桌 / "+item.Name), box, statusStyle.Render(m.status), muted.Render("E 改名 · ←/→ 人数 · O 房规 · L 房间列表 · M 声音")))
}
func (m *Model) makeClient() (*netclient.Client, table.Player, error) {
	name := strings.TrimSpace(m.name.Value())
	if name == "" {
		return nil, table.Player{}, fmt.Errorf("请先输入玩家名字")
	}
	identity, err := profile.Load(m.address, name)
	if err != nil {
		return nil, table.Player{}, err
	}
	client := netclient.New(m.address)
	client.Identity(identity.ID, identity.Token)
	return client, table.Player{ID: identity.ID, Name: name}, nil
}
func (m *Model) match() tea.Cmd {
	client, p, err := m.makeClient()
	if err != nil {
		m.status = err.Error()
		return nil
	}
	m.player = p
	m.pending = true
	mode := m.currentMode()
	m.status = "正在匹配，同模式玩家优先，15秒后补机器人"
	seats := m.seats
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		room, err := client.Match(ctx, mode, p, seats)
		if err == nil {
			err = client.Connect(context.Background(), room.Code, p.ID)
		}
		return connected{client, room, err}
	}
}
func (m *Model) statsPanel(personal bool) tea.Cmd {
	client, p, err := m.makeClient()
	if err != nil {
		m.status = err.Error()
		return nil
	}
	mode := m.currentMode()
	m.pending = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		data, err := client.Stats(ctx, mode, p.ID)
		if err != nil {
			return panelMsg{err: err}
		}
		type stat struct {
			Name                       string
			Wins, Losses, Draws, Score int
		}
		var result struct {
			Leaderboard []stat
			Mine        stat
		}
		if err = json.Unmarshal(data, &result); err != nil {
			return panelMsg{err: err}
		}
		if personal {
			v := result.Mine
			return panelMsg{text: fmt.Sprintf("我的战绩 · %s\n%s\n胜 %d  负 %d  和 %d\n累计积分 %d\n主动离开未结束对局记一次负场", mode, p.Name, v.Wins, v.Losses, v.Draws, v.Score)}
		}
		var b strings.Builder
		b.WriteString("排行榜 · " + string(mode) + "\n")
		for i, v := range result.Leaderboard {
			if i >= 15 {
				break
			}
			fmt.Fprintf(&b, "%2d  %-12s %5d 分  %d胜 %d负 %d和\n", i+1, v.Name, v.Score, v.Wins, v.Losses, v.Draws)
		}
		if len(result.Leaderboard) == 0 {
			b.WriteString("还没有已完成的真人对局记录")
		}
		return panelMsg{text: b.String()}
	}
}
func (m *Model) roomsPanel() tea.Cmd {
	client := netclient.New(m.address)
	mode := m.currentMode()
	m.pending = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rooms, err := client.Rooms(ctx, mode)
		if err != nil {
			return panelMsg{err: err}
		}
		var b strings.Builder
		b.WriteString("可加入的房间\n")
		for i, r := range rooms {
			if i >= 12 {
				break
			}
			fmt.Fprintf(&b, "%s  %d/%d人\n", r.Code, len(r.Players), r.Seats)
		}
		if len(rooms) == 0 {
			b.WriteString("当前没有等待中的房间")
		}
		b.WriteString("\n返回后按 3 输入房间号")
		return panelMsg{text: b.String()}
	}
}
func defaultSettings() map[table.Mode]map[string]any {
	result := map[table.Mode]map[string]any{}
	for mode, v := range map[table.Mode]any{table.MahjongMode: mahjong.DefaultOptions(), table.UNOMode: uno.DefaultOptions()} {
		data, _ := json.Marshal(v)
		var row map[string]any
		_ = json.Unmarshal(data, &row)
		result[mode] = row
	}
	return result
}

type optionRow struct {
	key, label string
	values     []any
}

func optionRows(mode table.Mode) []optionRow {
	if mode == table.MahjongMode {
		return []optionRow{
			{"exchangeThree", "换三张", []any{true, false}}, {"baseScore", "底分", []any{float64(1), float64(2), float64(5), float64(10)}}, {"fanCap", "封顶番（0不封顶）", []any{float64(8), float64(16), float64(0)}}, {"flowerPigPenalty", "花猪赔付倍数", []any{float64(2), float64(16)}}, {"dragonPairBonus", "龙七对每根额外番", []any{float64(0), float64(1), float64(2)}}, {"selfDrawBonus", "自摸加一番", []any{false, true}}, {"heavenlyEarthlyCap", "天地胡按封顶", []any{false, true}}}
	}
	if mode == table.UNOMode {
		return []optionRow{{"challenge", "+4 挑战", []any{true, false}}, {"blitz", "跳打", []any{true, false}}, {"sevenZero", "7换手 / 0传手", []any{true, false}}, {"openingEffects", "开局功能牌生效", []any{true, false}}, {"wrongUNOPenalty", "错喊UNO惩罚", []any{false, true}}, {"doublePlay", "同色数字双牌", []any{false, true}}, {"firstSeat", "先手座位（0庄家/1下家）", []any{float64(0), float64(1)}}}
	}
	return nil
}
func (m *Model) optionKey(key string) tea.Cmd {
	rows := optionRows(m.currentMode())
	if len(rows) == 0 {
		m.optionsOn = false
		return nil
	}
	switch key {
	case "esc":
		m.optionsOn = false
	case "up":
		m.optionIndex = (m.optionIndex + len(rows) - 1) % len(rows)
	case "down":
		m.optionIndex = (m.optionIndex + 1) % len(rows)
	case "enter", "right", "left", "space":
		row := rows[m.optionIndex]
		current := m.settings[m.currentMode()][row.key]
		next := 0
		for i, v := range row.values {
			if v == current {
				next = (i + 1) % len(row.values)
				break
			}
		}
		m.settings[m.currentMode()][row.key] = row.values[next]
	}
	return nil
}
func (m *Model) optionsView() string {
	var b strings.Builder
	b.WriteString("开局房规 · 创建后固定\n\n")
	for i, row := range optionRows(m.currentMode()) {
		mark := "  "
		if i == m.optionIndex {
			mark = "▶ "
		}
		value := fmt.Sprint(m.settings[m.currentMode()][row.key])
		if value == "true" {
			value = "开"
		} else if value == "false" {
			value = "关"
		}
		fmt.Fprintf(&b, "%s%-22s %s\n", mark, row.label, value)
	}
	b.WriteString("\n↑/↓ 选择 · Enter/←/→ 切换 · Esc 保存返回\n快速匹配使用统一默认房规")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}
func rulesFor(mode table.Mode) string {
	if mode == table.PokemonMode {
		return pokemonRules
	}
	common := "\n\n/ 开始聊天；聊天中 Enter 发送、Esc 退出聊天\nEsc 取消选牌/选子；Del 离桌并由机器人接管\nF2 结束后再开一局；M 声音；F1 规则"
	rules := map[table.Mode]string{
		table.SnakeMode:        "贪吃蛇 · 本地单机\n方向键 / WASD 控制方向，每个移动节拍只转向一次。\n不能直接反向；最多缓存两次转向，重复按键忽略。\n吃到食物长度+1、得分+10；每吃5个食物升一级。\n触碰边界或自身立即死亡，不穿墙；填满棋盘获胜。\nEnter / 空格开始；P / 空格暂停或继续，Esc暂停。\nF2 / R重开；Del / Q返回本模式菜单；M开关声音。\n查看规则会自动暂停，返回后按P / 空格继续。\n单机模式不需要服务器、房间或其他玩家。",
		table.TetrisMode:       "俄罗斯方块 · 生存对战\n双人 / 四人独立 10×20 棋盘，开局倒计时3秒。\n←/→ 移动；↑顺时针、↓逆时针旋转；S加速、空格落底。\n满一行自动消除，上方方块下移；一次可清除1–4行。\n每清10行加速，最低每0.5秒下落一格。\n堆积触顶或新方块无法生成即淘汰，最后存活者获胜。\n同一时钟周期全部触顶算平局；淘汰后可继续观战。\n各家方块序列相同；灰色虚影标出落点。\n大厅←/→切换2人/4人，选4进入Sunjiajia人机练习。",
		table.LandlordMode:     "斗地主\n三人叫分：1/2/3，P 不叫。地主先出，两农民合作。\n同型大牌可压，小王大王组成火箭；炸弹提高倍数。\n←/→ 或 Home/End 移动选牌光标，空格切换选牌。\nEnter 出已选牌，P 过牌；数字1–9快捷选牌。",
		table.LiarBarMode:      "骗子酒馆\n四人桌，每轮声明牌为 Q/K/A；Joker通用。\n数字选择1–3张，Enter盖牌；下一位可按 C 质疑。\n说谎被揭穿由出牌者开枪，否则质疑者开枪。\n亮牌公示5秒、枪决结果5秒后继续；最后存活者胜。",
		table.MahjongMode:      "四川麻将 · 血战到底\n108张万筒条，禁吃。先换三张同色牌，再同时定缺。\n←/→ + 空格选换牌，Enter提交；1/2/3选缺门。\nD摸牌，←/→选择后Enter出牌；Z胡、P碰、G补杠、B暗杠。\n回应弃牌：Z胡/P碰/G杠/N过。缺门优先打出。\n番数累加×底分，胡牌离场，剩余玩家继续。\n流局依次退杠分、查花猪、查叫；开局可在O设置房规。",
		table.ChessMode:        "中国象棋\n红先黑后，9×10棋盘。鼠标点击己方棋子，再点目标。\n也可方向键移动、Enter选子/落子，Esc取消。\n遵循蹩马腿、塞象眼、炮架、九宫和将帅照面规则。\n将死或无合法走法判负。",
		table.WesternChessMode: "国际象棋\n白先黑后，鼠标点棋子/目标；方向键+Enter也可操作。\n支持王车易位、吃过路兵、将死和无子可走和棋。\n升变前 Q后/R车/B象/N马；Esc取消选择。",
		table.GomokuMode:       "五子棋\n15×15棋盘，黑先白后。点击交叉点落子。\n横竖斜连续五子或以上获胜；本桌采用无禁手自由规则。",
		table.GoMode:           "围棋\n19路，黑先白后；点击交叉点落子，P停一手。\n支持提子、自杀禁入、全局同形禁入。\n双方连续停一手后数子，白贴7.5目。\n当前按终盘棋形直接数子，请先把死子处理完再停着。",
		table.UNOMode:          "UNO\n匹配颜色/数字/功能，万能牌可换色。←/→选牌，Enter出。\nR/Y/B/G选择万能颜色；D摸牌/承受叠罚，K摸牌后结束。\nU声明UNO；C抓漏喊；X挑战+4；T选择7的交换对象。\n0按方向传手，7交换手牌；非回合完全相同牌可跳打。\n+2/+4可叠加；V切换数字双牌（需开局启用）。\n先打空获胜，累计500分胜出；O设置可选房规。",
	}
	if mode == table.SnakeMode {
		return rules[mode]
	}
	return rules[mode] + common
}
