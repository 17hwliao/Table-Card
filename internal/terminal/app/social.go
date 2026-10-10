package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/netclient"
	"github.com/17hwliao/table-card-independent/internal/terminal/profile"
	"github.com/charmbracelet/x/ansi"
)

type socialConnected struct {
	client        *netclient.Client
	name, address string
	err           error
}
type socialReceived struct {
	client   *netclient.Client
	envelope netclient.Envelope
	err      error
}
type socialRetry struct{ client *netclient.Client }

func (m *Model) connectSocial() tea.Cmd {
	name := strings.TrimSpace(m.name.Value())
	if name == "" || m.socialConnecting {
		return nil
	}
	if m.socialReady && m.socialName == name {
		return nil
	}
	if m.social != nil {
		m.social.SocialClose()
	}
	id, err := profile.Load(m.address, name)
	if err != nil {
		m.status = "玩家身份读取失败：" + err.Error()
		return nil
	}
	m.player = table.Player{ID: id.ID, Name: name}
	c := netclient.New(m.address)
	c.Identity(id.ID, id.Token)
	m.social = c
	m.socialName = name
	m.socialConnecting = true
	m.socialReady = false
	m.socialMode = m.presenceMode()
	mode := m.socialMode
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		_, err := c.Modes(ctx)
		if err == nil {
			err = c.SocialConnect(ctx, id.ID, name, mode)
		}
		address := ""
		if err == nil {
			if info, e := c.ConnectionInfo(ctx); e == nil && len(info.Addresses) > 0 {
				address = info.Addresses[0].Address
			}
		}
		return socialConnected{client: c, name: name, address: address, err: err}
	}
}
func (m *Model) socialConnected(msg socialConnected) tea.Cmd {
	if msg.client != m.social {
		msg.client.SocialClose()
		return nil
	}
	m.socialConnecting = false
	if msg.err != nil {
		m.socialReady = false
		m.status = "消息服务未连接：" + msg.err.Error() + " · F4 重试（同昵称仅开一个窗口）"
		return nil
	}
	m.socialReady = true
	m.socialAddress = msg.address
	var recoverCmd tea.Cmd
	if m.pokemonRecover && m.page == pokemonScreen {
		recoverCmd = m.socialSend(map[string]any{"type": "pokemon_fetch"})
	}
	return tea.Batch(readSocial(m.social), m.publishSocialMode(), recoverCmd)
}
func readSocial(c *netclient.Client) tea.Cmd {
	return func() tea.Msg { envelope, err := c.SocialRead(); return socialReceived{c, envelope, err} }
}
func (m *Model) socialReceived(msg socialReceived) tea.Cmd {
	if msg.client != m.social {
		return nil
	}
	if msg.err != nil {
		m.socialReady = false
		m.pokemonSocialReady = false
		m.socialUsers = nil
		m.pokemonRecover = m.pokemonDuel != nil || m.pokemonOffer != nil || m.pokemonSocialPending
		if m.pokemonRecover {
			m.pokemonDuel = nil
			m.pokemonDuelExit = false
			m.pokemonOffer = nil
			m.pokemonSocialPending = false
			m.pokemonPanel = "online"
		}
		m.status = "消息连接中断，正在重连；正在进行的玩家对战按断线认输结算"
		c := m.social
		return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return socialRetry{c} })
	}
	var cmd tea.Cmd
	switch msg.envelope.Type {
	case "presence":
		selectedID := ""
		if m.socialCursor < len(m.socialUsers) {
			selectedID = m.socialUsers[m.socialCursor].PlayerID
		}
		_ = json.Unmarshal(msg.envelope.Data, &m.socialUsers)
		m.socialCursor = min(m.socialCursor, max(0, len(m.socialUsers)-1))
		for i, u := range m.socialUsers {
			if u.PlayerID == selectedID {
				m.socialCursor = i
			}
		}
		if m.page == pokemonScreen && m.pokemonPanel == "online" && m.pokemonSocialReady && !m.pokemonSocialPending {
			cmd = m.socialSend(map[string]any{"type": "pokemon_roster"})
		}
	case "chat":
		m.appendChat(msg.envelope)
	case "error":
		m.status = msg.envelope.Error
	default:
		cmd = m.pokemonSocialEvent(msg.envelope)
	}
	return tea.Batch(readSocial(msg.client), cmd)
}
func (m *Model) publishSocialMode() tea.Cmd {
	mode := m.presenceMode()
	if !m.socialReady || m.social == nil || mode == m.socialMode {
		return nil
	}
	m.socialMode = mode
	return m.socialSend(map[string]any{"type": "status", "mode": mode})
}
func (m *Model) presenceMode() table.Mode {
	if m.page == homeScreen {
		return ""
	}
	return m.currentMode()
}
func (m *Model) socialSend(v any) tea.Cmd {
	if m.social == nil || !m.socialReady {
		m.status = "先连接房主服务（F4重试）；单机冒险仍可继续"
		return nil
	}
	c := m.social
	return func() tea.Msg { return chatSent{c.SocialSend(v)} }
}
func (m *Model) appendChat(e netclient.Envelope) {
	name := e.Name
	if e.PlayerID == m.player.ID {
		name = "我"
	}
	scope := "房间 " + e.RoomCode
	if e.Scope == "server" {
		scope = "全服"
	}
	if e.Scope == "direct" {
		scope = "私聊"
		if e.PlayerID == m.player.ID {
			for _, u := range m.socialUsers {
				if u.PlayerID == e.TargetID {
					scope += "→" + u.Name
				}
			}
		}
	}
	if e.Scope == "" {
		scope = "房间"
	}
	m.chatLog = append(m.chatLog, fmt.Sprintf("[%s] %s：%s", scope, name, e.Text))
	if len(m.chatLog) > 60 {
		m.chatLog = m.chatLog[len(m.chatLog)-60:]
	}
}
func (m *Model) openChat(scope string) tea.Cmd {
	if !m.socialReady {
		m.status = "消息服务未连接，请 F4 重试"
		return m.connectSocial()
	}
	if scope == "room" && m.room.Code == "" {
		m.status = "先加入房间才能发送房间广播；F6全服 / F8私聊"
		return nil
	}
	m.chatScope = scope
	m.chatOn = true
	m.socialPanel = false
	m.quickChat = false
	label := map[string]string{"server": "全服广播", "room": "房间广播", "direct": "私聊→" + m.chatTargetName}[scope]
	m.chat.Prompt = label + " › "
	m.chat.SetWidth(max(8, m.width-lipgloss.Width(m.chat.Prompt)-4))
	return m.chat.Focus()
}
func (m *Model) sendChatText(text string) tea.Cmd {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	scope := m.chatScope
	if scope == "" {
		scope = "server"
		if m.room.Code != "" {
			scope = "room"
		}
	}
	return m.socialSend(map[string]any{"type": "chat", "scope": scope, "targetId": m.chatTarget, "text": text})
}
func (m *Model) socialKey(message tea.KeyMsg) (bool, tea.Cmd) {
	key := message.String()
	if m.editing != "" {
		return false, nil
	}
	if m.chatOn {
		switch key {
		case "esc":
			m.chatOn = false
			m.chat.Blur()
			return true, nil
		case "enter":
			if p, ok := message.(tea.KeyPressMsg); ok && p.IsRepeat {
				return true, nil
			}
			text := m.chat.Value()
			m.chat.SetValue("")
			return true, m.sendChatText(text)
		case "f3":
			m.quickChat = true
			m.chatOn = false
			return true, nil
		}
		var cmd tea.Cmd
		m.chat, cmd = m.chat.Update(message)
		return true, cmd
	}
	if m.quickChat {
		return true, m.quickChatKey(key)
	}
	if p, ok := message.(tea.KeyPressMsg); ok && p.IsRepeat && strings.HasPrefix(key, "f") {
		return true, nil
	}
	switch key {
	case "f3":
		if m.socialReady && m.overlay == "" && !m.optionsOn {
			m.quickChat = true
			return true, nil
		}
	case "f4":
		m.socialPanel = !m.socialPanel
		if !m.socialReady {
			return true, m.connectSocial()
		}
		return true, nil
	case "f6":
		return true, m.openChat("server")
	case "f7":
		return true, m.openChat("room")
	case "f8":
		m.socialPanel = true
		m.status = "↑↓选择在线玩家，Enter开始私聊"
		return true, nil
	case "/":
		if m.overlay == "" && !m.optionsOn {
			scope := "server"
			if m.room.Code != "" {
				scope = "room"
			}
			return true, m.openChat(scope)
		}
	}
	if m.socialPanel {
		switch key {
		case "esc":
			m.socialPanel = false
		case "up":
			m.socialCursor = max(0, m.socialCursor-1)
		case "down":
			m.socialCursor = min(max(0, len(m.socialUsers)-1), m.socialCursor+1)
		case "enter":
			if p, ok := message.(tea.KeyPressMsg); ok && p.IsRepeat {
				return true, nil
			}
			if m.socialCursor < len(m.socialUsers) {
				u := m.socialUsers[m.socialCursor]
				if u.PlayerID == m.player.ID {
					m.status = "请选择另一位玩家"
					return true, nil
				}
				m.chatTarget, m.chatTargetName = u.PlayerID, u.Name
				return true, m.openChat("direct")
			}
		}
		return true, nil
	}
	return false, nil
}
func (m *Model) socialChatRows() int {
	if m.chatOn {
		return 6
	}
	return 4
}
func (m *Model) socialChatView(width int) string {
	rows := []string{}
	for _, entry := range m.chatLog {
		rows = append(rows, strings.Split(ansi.Wrap(entry, max(12, width-2), ""), "\n")...)
	}
	if len(rows) > 2 {
		rows = rows[len(rows)-2:]
	}
	for len(rows) < 2 {
		rows = append([]string{""}, rows...)
	}
	label := "消息未连接 · F4重试"
	if m.socialReady {
		label = fmt.Sprintf("在线 %d · F4玩家列表 · F6全服 / F7房间 / F8私聊", len(m.socialUsers))
	}
	view := muted.Render(ansi.Truncate(label, width, "…")) + "\n" + strings.Join(rows, "\n")
	if m.chatOn {
		view += "\n" + m.chat.View() + "\n" + muted.Render("Enter发送 · F3数字短语 · Esc退出聊天开始选择")
	}
	return view
}
func userStatus(u netclient.OnlineUser) string {
	status := map[string]string{"lobby": "大厅", "waiting": "等待准备", "playing": "游戏中", "finished": "已结束", "offlineRoom": "掉线", "adventuring": "冒险中", "center": "联机中心", "invited": "对战邀请中", "trading": "交换确认中", "battling": "玩家对战中", "settling": "金币结算中"}[u.Status]
	if u.Status == "waiting" && u.Ready {
		status = "已准备"
	}
	if u.Host {
		status += " · 房主"
	}
	return status
}
func (m *Model) modeName(mode table.Mode) string {
	if mode == table.PokemonMode {
		return "宝可梦"
	}
	if mode == table.SnakeMode {
		return "贪吃蛇"
	}
	for _, item := range m.modes {
		if item.ID == mode {
			return item.Name
		}
	}
	return "选模式"
}
func (m *Model) socialSidebar(width, height int) string {
	rows := []string{titleStyle.Render(fmt.Sprintf("在线玩家 · %d", len(m.socialUsers))), muted.Render("F4查看 / F8选择私聊"), ""}
	capacity := max(1, (height-6)/3)
	start := max(0, min(m.socialCursor-capacity+1, max(0, len(m.socialUsers)-capacity)))
	for i := start; i < min(len(m.socialUsers), start+capacity); i++ {
		u := m.socialUsers[i]
		name := u.Name
		if u.PlayerID == m.player.ID {
			name += "（你）"
		}
		marker := "  "
		if i == m.socialCursor {
			marker = "▶ "
		}
		rows = append(rows, selected.Render(ansi.Truncate(marker+name, width, "…")), ansi.Truncate(m.modeName(u.Mode)+" · "+userStatus(u), width, "…"), muted.Render("房间 "+display(u.RoomCode, "—")))
	}
	if len(m.socialUsers) == 0 {
		rows = append(rows, "暂无在线玩家")
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(rows, "\n"))
}
func (m *Model) socialPanelView() string {
	return lipgloss.Place(m.width, max(10, m.height-m.socialChatRows()), lipgloss.Center, lipgloss.Center, m.socialSidebar(min(64, max(20, m.width-8)), max(8, m.height-9))+"\n\n"+muted.Render("↑↓选择 · Enter私聊 · F6全服 · F7房间 · Esc返回"))
}
