package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/audio"
	"github.com/17hwliao/table-card-independent/internal/terminal/modes"
	"github.com/17hwliao/table-card-independent/internal/terminal/netclient"
	"github.com/17hwliao/table-card-independent/internal/terminal/profile"
	modeui "github.com/17hwliao/table-card-independent/internal/terminal/ui"
)

type screen uint8

const (
	homeScreen screen = iota
	roomScreen
	gameScreen
)

type modeInfo struct {
	ID       table.Mode `json:"id"`
	Name     string     `json:"name"`
	MinSeats int        `json:"minSeats"`
	MaxSeats int        `json:"maxSeats"`
	Progress string     `json:"progress"`
}

type modesLoaded struct {
	items []modeInfo
	err   error
}
type connected struct {
	client *netclient.Client
	room   table.Snapshot
	err    error
}
type roomUpdated struct {
	client *netclient.Client
	room   table.Snapshot
	err    error
}
type socketReceived struct {
	client   *netclient.Client
	envelope netclient.Envelope
	err      error
}
type actionSent struct{ err error }
type chatSent struct{ err error }

type Model struct {
	sound        *audio.Player
	menuOn       bool
	overlay      string
	optionsOn    bool
	optionIndex  int
	settings     map[table.Mode]map[string]any
	pending      bool
	reconnecting bool
	pauseUntil   int64
	clockActive  bool
	address      string
	api          *netclient.Client
	page         screen
	modes        []modeInfo
	mode         int
	seats        int
	bots         bool
	name         textinput.Model
	code         textinput.Model
	editing      string
	room         table.Snapshot
	player       table.Player
	game         json.RawMessage
	control      modeui.Controller
	width        int
	height       int
	status       string
	chat         textinput.Model
	chatOn       bool
	chatLog      []string
}

func New(address, playerName string) *Model {
	if address == "" {
		address = "localhost:1781"
	}
	name := textinput.New()
	name.Prompt = "姓名："
	name.SetValue(playerName)
	name.CharLimit = 24
	name.SetWidth(26)
	code := textinput.New()
	code.Prompt = "房间号："
	code.Placeholder = "输入 6 位房间号"
	code.CharLimit = 6
	code.SetWidth(20)
	chat := textinput.New()
	chat.Prompt = "聊天 › "
	chat.CharLimit = 300
	chat.SetWidth(50)
	return &Model{sound: audio.New(), settings: defaultSettings(), address: address, page: homeScreen, name: name, code: code, chat: chat, seats: 2, status: "正在连接牌桌服务…", width: 90, height: 30}
}

func (m *Model) Init() tea.Cmd { return loadModes(m.address) }

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case clockMsg:
		if m.page == gameScreen && time.Now().Unix() < m.pauseUntil {
			return m, clockTick()
		}
		m.clockActive = false
		m.pauseUntil = 0
		return m, nil
	case panelMsg:
		m.pending = false
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.overlay = msg.text
		}
		return m, nil
	case reconnectMsg:
		if msg.client != m.api {
			return m, nil
		}
		if msg.err != nil {
			m.status = fmt.Sprintf("重连第 %d 次失败；Del 离桌", msg.attempt)
			return m, reconnect(m.api, m.room.Code, m.player.ID, msg.attempt+1)
		}
		m.reconnecting = false
		m.status = "已恢复房间连接"
		return m, readSocket(m.api)
	case tea.MouseClickMsg:
		if m.page == gameScreen && !m.chatOn && m.overlay == "" && msg.Button == tea.MouseLeft {
			if control, ok := m.control.(interface {
				Mouse(modeui.Snapshot, int, int) modeui.Result
			}); ok {
				return m, m.submit(control.Mouse(m.controllerSnapshot(), msg.X, msg.Y-2))
			}
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case modesLoaded:
		if msg.err != nil {
			m.status = "连接失败：" + msg.err.Error()
			return m, nil
		}
		m.modes = msg.items
		if len(m.modes) > 0 {
			m.mode = min(m.mode, len(m.modes)-1)
			m.seats = m.modes[m.mode].MaxSeats
			m.status = "已连接 · 可创建房间或输入房间号加入"
		}
		return m, nil
	case connected:
		m.pending = false
		if msg.err != nil {
			m.status = "房间连接失败：" + msg.err.Error()
			return m, nil
		}
		m.api, m.room = msg.client, msg.room
		m.page = roomScreen
		m.game = nil
		m.control = nil
		m.chatLog = nil
		m.status = "实时连接成功"
		m.reconnecting = false
		if err := m.sound.SetMode(m.room.Mode); err != nil {
			m.status = "声音加载失败：" + err.Error()
		}
		return m, readSocket(msg.client)
	case roomUpdated:
		if msg.client != m.api {
			return m, nil
		}
		if msg.err != nil {
			m.status = "房间操作失败：" + msg.err.Error()
		} else {
			m.room = msg.room
			m.status = "房间状态已更新"
		}
		return m, nil
	case socketReceived:
		if msg.client != m.api {
			return m, nil
		}
		if msg.err != nil {
			m.status = "实时连接断开：" + msg.err.Error()
			m.reconnecting = true
			return m, reconnect(m.api, m.room.Code, m.player.ID, 1)
		}
		m.applyEnvelope(msg.envelope)
		if m.page == gameScreen && time.Now().Unix() < m.pauseUntil && !m.clockActive {
			m.clockActive = true
			return m, tea.Batch(readSocket(msg.client), clockTick())
		}
		return m, readSocket(msg.client)
	case actionSent:
		if msg.err != nil {
			m.status = "操作未送达：" + msg.err.Error()
		}
		return m, nil
	case chatSent:
		if msg.err != nil {
			m.status = "聊天发送失败：" + msg.err.Error()
		}
		return m, nil
	case tea.KeyPressMsg:
		return m, m.key(msg)
	}
	return m, nil
}

func (m *Model) applyEnvelope(envelope netclient.Envelope) {
	switch envelope.Type {
	case "room":
		_ = json.Unmarshal(envelope.Data, &m.room)
		m.page = roomScreen
		m.game = nil
		m.status = "等待玩家准备"
	case "state":
		_ = json.Unmarshal(envelope.Room, &m.room)
		if len(m.game) > 0 && !bytes.Equal(m.game, envelope.Game) {
			m.sound.Effect(effectSeed(envelope.Game))
		}
		m.game = append(m.game[:0], envelope.Game...)
		m.pauseUntil = envelope.PauseUntil
		if m.room.Phase == table.InProgress {
			if m.control == nil || m.control.Mode() != m.room.Mode {
				m.control = modes.New(m.room.Mode)
			}
			m.page = gameScreen
		}
	case "chat":
		prefix := envelope.Name
		if envelope.PlayerID == m.player.ID {
			prefix = "我"
		}
		m.chatLog = append(m.chatLog, prefix+"："+envelope.Text)
		if len(m.chatLog) > 5 {
			m.chatLog = m.chatLog[len(m.chatLog)-5:]
		}
	case "error":
		m.status = envelope.Error
	}
}

func (m *Model) key(message tea.KeyMsg) tea.Cmd {
	key := message.String()
	if key == "ctrl+c" {
		return tea.Sequence(m.leave(), tea.Quit)
	}
	if m.editing != "" {
		if key == "esc" {
			m.name.Blur()
			m.code.Blur()
			m.editing = ""
			return nil
		}
		if key == "enter" {
			joinAfter := m.editing == "join"
			m.name.Blur()
			m.code.Blur()
			m.editing = ""
			if joinAfter {
				return m.join()
			}
			return nil
		}
		var cmd tea.Cmd
		if m.editing == "name" {
			m.name, cmd = m.name.Update(message)
		} else {
			m.code, cmd = m.code.Update(message)
		}
		return cmd
	}
	if m.chatOn {
		if key == "esc" {
			m.chatOn = false
			m.chat.Blur()
			m.status = "聊天已关闭"
			return nil
		}
		if key == "enter" {
			text := strings.TrimSpace(m.chat.Value())
			m.chat.SetValue("")
			if text != "" && m.api != nil {
				client := m.api
				return func() tea.Msg {
					return chatSent{err: client.Send(map[string]any{"type": "chat", "text": text})}
				}
			}
			return nil
		}
		var cmd tea.Cmd
		m.chat, cmd = m.chat.Update(message)
		return cmd
	}
	if key == "m" {
		muted, err := m.sound.Toggle()
		if err != nil {
			m.status = "无法开启声音：" + err.Error()
		} else if muted {
			m.status = "声音已关闭"
		} else {
			m.status = "声音已开启 · M 关闭"
		}
		return nil
	}
	if m.optionsOn {
		return m.optionKey(key)
	}
	if m.overlay != "" {
		if key == "esc" || key == "enter" {
			m.overlay = ""
		}
		return nil
	}
	if key == "f1" || key == "?" {
		m.overlay = rulesFor(m.currentMode())
		return nil
	}
	if m.pending {
		return nil
	}

	switch m.page {
	case homeScreen:
		return m.homeKey(key)
	case roomScreen:
		return m.roomKey(key)
	case gameScreen:
		return m.gameKey(key)
	}
	return nil
}

func (m *Model) homeKey(key string) tea.Cmd {
	if key == "f5" {
		return loadModes(m.address)
	}
	if key == "q" {
		return tea.Quit
	}
	if len(m.modes) == 0 {
		return nil
	}
	if m.menuOn {
		return m.menuKey(key)
	}
	switch key {
	case "up", "k":
		m.mode = (m.mode + len(m.modes) - 1) % len(m.modes)
		m.fixSeats()
	case "down":
		m.mode = (m.mode + 1) % len(m.modes)
		m.fixSeats()
	case "left", "h":
		m.setSeats(m.seats - 1)
	case "right", "l":
		m.setSeats(m.seats + 1)
	case "e":
		m.editing = "name"
		return m.name.Focus()
	case "r":
		m.editing = "code"
		return m.code.Focus()
	case "b":
		m.bots = true
		return m.create()
	case "c":
		return m.create()
	case "enter":
		m.menuOn = true
	case "j":
		if strings.TrimSpace(m.code.Value()) != "" {
			return m.join()
		}
		return m.create()
	case "q":
		return tea.Quit
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '8' {
			index := int(key[0] - '1')
			if index < len(m.modes) {
				m.mode = index
				m.fixSeats()
				m.menuOn = true
			}
		}
	}
	return nil
}

func (m *Model) roomKey(key string) tea.Cmd {
	switch key {
	case "r":
		ready := true
		for _, player := range m.room.Players {
			if player.ID == m.player.ID {
				ready = !player.Ready
			}
		}
		return m.updateRoom(func(ctx context.Context, client *netclient.Client) (table.Snapshot, error) {
			return client.SetReady(ctx, m.room.Code, m.player.ID, ready)
		})
	case "s", "enter":
		return m.updateRoom(func(ctx context.Context, client *netclient.Client) (table.Snapshot, error) {
			return client.StartRoom(ctx, m.room.Code)
		})
	case "/":
		m.chatOn = true
		return m.chat.Focus()
	case "esc", "q", "delete":
		return m.leave()
	}
	return nil
}

func (m *Model) gameKey(key string) tea.Cmd {
	if key == "/" {
		m.chatOn = true
		return m.chat.Focus()
	}
	if key == "delete" {
		return m.leave()
	}
	if key == "f2" {
		return m.rematch()
	}
	if m.reconnecting {
		m.status = "正在恢复连接，请稍候"
		return nil
	}
	if time.Now().Unix() < m.pauseUntil {
		m.status = "质疑结果公示中，请稍候"
		return nil
	}
	if m.control == nil || m.api == nil {
		return nil
	}
	return m.submit(m.control.Key(m.controllerSnapshot(), key))
}

func (m *Model) controllerSnapshot() modeui.Snapshot {
	return modeui.Snapshot{Mode: m.room.Mode, Room: m.room, PlayerID: m.player.ID, Game: m.game, Width: m.width, Height: m.height}
}

func (m *Model) create() tea.Cmd {
	if !m.nameReady() {
		return nil
	}
	info := m.modes[m.mode]
	seats, bots := m.seats, 0
	if m.bots {
		bots = seats - 1
	}
	identity, err := profile.Load(m.address, strings.TrimSpace(m.name.Value()))
	if err != nil {
		m.status = "身份保存失败：" + err.Error()
		return nil
	}
	player := table.Player{ID: identity.ID, Name: strings.TrimSpace(m.name.Value())}
	client := netclient.New(m.address)
	client.Identity(identity.ID, identity.Token)
	m.player = player
	m.pending = true
	options, _ := json.Marshal(m.settings[info.ID])
	m.status = "正在创建房间…"
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		room, err := client.CreateRoom(ctx, info.ID, seats, bots, player, options)
		if err == nil && bots > 0 {
			room, err = client.SetReady(ctx, room.Code, player.ID, true)
			if err == nil {
				room, err = client.StartRoom(ctx, room.Code)
			}
		}
		if err == nil {
			err = client.Connect(context.Background(), room.Code, player.ID)
		}
		return connected{client: client, room: room, err: err}
	}
}

func (m *Model) join() tea.Cmd {
	if !m.nameReady() {
		return nil
	}
	identity, err := profile.Load(m.address, strings.TrimSpace(m.name.Value()))
	if err != nil {
		m.status = "身份保存失败：" + err.Error()
		return nil
	}
	player := table.Player{ID: identity.ID, Name: strings.TrimSpace(m.name.Value())}
	code := strings.ToUpper(strings.TrimSpace(m.code.Value()))
	client := netclient.New(m.address)
	client.Identity(identity.ID, identity.Token)
	m.player = player
	m.pending = true
	m.status = "正在加入房间…"
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		room, err := client.JoinRoom(ctx, code, player)
		if err == nil {
			err = client.Connect(context.Background(), room.Code, player.ID)
		}
		return connected{client: client, room: room, err: err}
	}
}

func (m *Model) updateRoom(action func(context.Context, *netclient.Client) (table.Snapshot, error)) tea.Cmd {
	if m.api == nil {
		return nil
	}
	client := m.api
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	return func() tea.Msg {
		defer cancel()
		room, err := action(ctx, client)
		return roomUpdated{client: client, room: room, err: err}
	}
}

func (m *Model) closeRoom() {
	if m.api != nil {
		m.api.Close()
		m.api = nil
	}
	m.room = table.Snapshot{}
	m.game = nil
	m.control = nil
	m.chatOn = false
}

func (m *Model) nameReady() bool {
	if strings.TrimSpace(m.name.Value()) == "" {
		m.status = "请先按 E 输入玩家名称"
		return false
	}
	return true
}

func (m *Model) fixSeats() {
	if len(m.modes) == 0 {
		return
	}
	item := m.modes[m.mode]
	if m.seats < item.MinSeats || m.seats > item.MaxSeats {
		m.seats = item.MaxSeats
	}
	m.bots = false
}

func (m *Model) setSeats(value int) {
	item := m.modes[m.mode]
	if value < item.MinSeats {
		value = item.MinSeats
	}
	if value > item.MaxSeats {
		value = item.MaxSeats
	}
	m.seats = value
}

func (m *Model) View() tea.View {
	var content string
	switch m.page {
	case homeScreen:
		content = m.homeView()
	case roomScreen:
		content = m.roomView()
	case gameScreen:
		content = m.gameView()
	}
	if m.optionsOn {
		content = m.optionsView()
	} else if m.overlay != "" {
		content = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.overlay+"\n\nEsc / Enter 返回")
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "牌桌 Card Table"
	if m.page == gameScreen && m.overlay == "" {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

func (m *Model) homeView() string {
	if m.menuOn {
		return m.modeLobbyView()
	}
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("#E9B44C")).Bold(true).Render("♟ 牌桌 / CARD TABLE")
	intro := muted.Render("独立规则引擎 · 终端实时牌桌")
	var rows []string
	for i, item := range m.modes {
		marker := "  "
		style := muted
		if i == m.mode {
			marker, style = "▶ ", selected
		}
		rows = append(rows, style.Render(fmt.Sprintf("%s%d. %-12s %s", marker, i+1, item.Name, item.Progress)))
	}
	menu := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#426A58")).Padding(1, 2).Width(minInt(78, maxInt(42, m.width-8))).Render(strings.Join(rows, "\n"))
	item := modeInfo{}
	if len(m.modes) > 0 {
		item = m.modes[m.mode]
	}
	settings := fmt.Sprintf("座位  %d / %d     玩家  %s     房间  %s", m.seats, item.MaxSeats, display(m.name.Value(), "玩家 1"), display(strings.ToUpper(m.code.Value()), "未填写"))
	if m.editing == "name" {
		settings = m.name.View()
	} else if m.editing != "" {
		settings = m.code.View()
	}
	if m.bots {
		settings += fmt.Sprintf("    Sunjiajia ×%d", m.seats-1)
	}
	keys := "↑/↓ 选模式 · 1–8 / Enter 进入 · E 改名\nM 声音 · F5 重连服务 · Q 退出"
	content := lipgloss.JoinVertical(lipgloss.Center, title, intro, "", menu, settings, "", statusStyle.Render(m.status), muted.Render(keys))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *Model) roomView() string {
	modeName := string(m.room.Mode)
	for _, item := range m.modes {
		if item.ID == m.room.Mode {
			modeName = item.Name
		}
	}
	title := titleStyle.Render("牌桌  /  " + modeName)
	meta := fmt.Sprintf("房间 %s   %d / %d 人", m.room.Code, len(m.room.Players), m.room.Seats)
	var players []string
	for i, p := range m.room.Players {
		state := "等待准备"
		if p.Ready {
			state = "已准备"
		}
		if p.ID == m.player.ID {
			state += " · 你"
		}
		if p.Bot {
			state += " · AI"
		}
		players = append(players, fmt.Sprintf("%d. %-24s %s", i+1, p.Name, state))
	}
	if len(players) == 0 {
		players = append(players, "等待房间状态…")
	}
	body := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#426A58")).Padding(1, 3).Width(58).Render(strings.Join(players, "\n"))
	controls := "R 准备/取消准备   S 开始（全员准备后）   / 聊天   Esc 返回模式选择"
	chat := ""
	if m.chatOn {
		chat = "\n" + m.chat.View() + "\n" + muted.Render("Enter 发送 · Esc 关闭聊天")
	} else if len(m.chatLog) > 0 {
		chat = "\n" + muted.Render(strings.Join(m.chatLog, "\n"))
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, lipgloss.JoinVertical(lipgloss.Center, title, muted.Render(meta), "", body, "", statusStyle.Render(m.status), muted.Render(controls), chat))
}

func (m *Model) gameView() string {
	if m.control == nil {
		return "正在加载终端牌桌…"
	}
	view := m.control.View(m.controllerSnapshot())
	header := titleStyle.Render(fmt.Sprintf("牌桌  /  %s  /  房间 %s", m.room.Mode, m.room.Code))
	help := muted.Render("/ 聊天 · Esc 取消选择 · Del 离桌 · F1 规则 · F2 再来一局 · M 声音")
	chat := ""
	if m.chatOn {
		chat = "\n" + m.chat.View() + "\n" + muted.Render("Enter 发送 · Esc 关闭聊天")
	} else if len(m.chatLog) > 0 {
		chat = "\n" + muted.Render(strings.Join(m.chatLog, "\n"))
	}
	status := m.status
	if remaining := m.pauseUntil - time.Now().Unix(); remaining > 0 {
		stage := "枪决结果"
		if remaining > 5 {
			stage = "亮牌公示"
		}
		status = fmt.Sprintf("%s · %d 秒后继续", stage, remaining)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, "", view, "", statusStyle.Render(status), help, chat)
}

func loadModes(address string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		modes, err := netclient.New(address).Modes(ctx)
		items := make([]modeInfo, len(modes))
		for i, item := range modes {
			items[i] = modeInfo(item)
		}
		return modesLoaded{items: items, err: err}
	}
}

func readSocket(client *netclient.Client) tea.Cmd {
	return func() tea.Msg {
		envelope, err := client.Read()
		return socketReceived{client: client, envelope: envelope, err: err}
	}
}

func newID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("player-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(data[:])
}

var (
	titleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E9B44C")).Bold(true)
	selected    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7BC99A")).Bold(true)
	muted       = lipgloss.NewStyle().Foreground(lipgloss.Color("#9BAFA5"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1D078"))
)

func display(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
