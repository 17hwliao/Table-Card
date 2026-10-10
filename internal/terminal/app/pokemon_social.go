package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/17hwliao/table-card-independent/internal/pokemon"
	"github.com/17hwliao/table-card-independent/internal/terminal/netclient"
	"github.com/charmbracelet/x/ansi"
)

type pokemonOnlinePlayer struct {
	PlayerID string            `json:"playerId"`
	Name     string            `json:"name"`
	Party    []pokemon.Monster `json:"party"`
	Busy     bool              `json:"busy"`
	Ready    bool              `json:"ready"`
	Money    int               `json:"money"`
}
type pokemonInvite struct {
	ID, Kind, From, To, FromName, ToName string
	Slots                                [2]int
	Monsters                             [2]pokemon.Monster
	Expires                              int64
}

func netOption(label, cmd, panel string) pokemon.Choice {
	return pokemon.Choice{Label: label, Command: cmd, Panel: panel, Enabled: true}
}
func (m *Model) pokemonSocialOptions() []pokemon.Choice {
	g := m.pokemonGame
	if g == nil {
		return nil
	}
	switch m.pokemonPanel {
	case "online":
		out := []pokemon.Choice{netOption("同步队伍 / 刷新在线训练家", "net sync", "online")}
		for _, p := range m.pokemonRoster {
			label := p.Name + fmt.Sprintf(" · %d只伙伴", len(p.Party))
			if p.Busy {
				label += " · 忙碌"
			}
			c := netOption(label, "net player "+p.PlayerID, "net-player")
			c.Enabled = !p.Busy && p.Ready && m.pokemonSocialReady
			out = append(out, c)
		}
		return append(out, netOption("返回城市中心", "net pause", "center"))
	case "net-player":
		return []pokemon.Choice{netOption("邀请对战 · 胜+300 / 败−300", "net duel", "net-offer"), netOption("交换队伍伙伴 · 选择我的伙伴", "", "net-own"), netOption("返回在线列表", "net roster", "online")}
	case "net-own":
		out := []pokemon.Choice{}
		for i, p := range g.Party {
			out = append(out, netOption(fmt.Sprintf("送出 %s Lv%d", p.Name(), p.Level), fmt.Sprintf("net own %d", i), "net-foe"))
		}
		return out
	case "net-foe":
		out := []pokemon.Choice{}
		for _, p := range m.pokemonRoster {
			if p.PlayerID == m.pokemonSocialTarget {
				for i, mon := range p.Party {
					out = append(out, netOption(fmt.Sprintf("换取 %s Lv%d", mon.Name(), mon.Level), fmt.Sprintf("net foe %d", i), "net-confirm"))
				}
			}
		}
		return out
	case "net-confirm":
		return []pokemon.Choice{netOption("确认交换提案 · 发给对方", "net trade", "net-offer"), netOption("取消，重新选择", "", "net-player")}
	case "net-offer":
		if m.pokemonOffer == nil {
			return []pokemon.Choice{netOption("刷新在线训练家", "net roster", "online")}
		}
		out := []pokemon.Choice{}
		if m.pokemonOffer.To == m.player.ID {
			out = append(out, netOption("接受邀请（确认双方伙伴 / 300金币规则）", "net accept", "net-offer"))
		}
		return append(out, netOption("拒绝 / 取消邀请", "net cancel", "online"))
	}
	return nil
}
func (m *Model) pokemonSocialInfo() string {
	switch m.pokemonPanel {
	case "online":
		return "训练家联机中心\nL + Enter 可随时打开\n先同步，再选择在线玩家\n双方确认后才开始对战或交换\n本地战斗期间不可邀请\n对战使用满HP副本，不损耗冒险队伍\n断线 / Del退出按认输结算\n90秒未接受则邀请取消"
	case "net-player":
		for _, p := range m.pokemonRoster {
			if p.PlayerID == m.pokemonSocialTarget {
				return fmt.Sprintf("训练家 %s\n队伍%d只 · 金币%d\n胜者净获得300金币\n败者净失去300金币\n双方至少300金币才可开战", p.Name, len(p.Party), p.Money)
			}
		}
	case "net-own", "net-foe":
		return "交换伙伴\n先选择自己的队伍伙伴\n再选择对方的队伍伙伴\n最后确认提案，由对方接受\n等级、经验、技能、PP均保留\n通信进化会在交换时触发"
	case "net-confirm":
		if m.pokemonTradeSlots[0] < len(m.pokemonGame.Party) {
			for _, p := range m.pokemonRoster {
				if p.PlayerID == m.pokemonSocialTarget && m.pokemonTradeSlots[1] < len(p.Party) {
					a, b := m.pokemonGame.Party[m.pokemonTradeSlots[0]], p.Party[m.pokemonTradeSlots[1]]
					return fmt.Sprintf("交换确认\n你送出：%s Lv%d\n收到：%s Lv%d\n对方：%s\n选择1发出，双方档案会暂时锁定", a.Name(), a.Level, b.Name(), b.Level, p.Name)
				}
			}
		}
	case "net-offer":
		if o := m.pokemonOffer; o != nil {
			if o.Kind == "trade" {
				return fmt.Sprintf("交换邀请\n%s送出%s Lv%d\n%s送出%s Lv%d\n双方确认后一次性交换\n90秒未接受自动取消", o.FromName, o.Monsters[0].Name(), o.Monsters[0].Level, o.ToName, o.Monsters[1].Name(), o.Monsters[1].Level)
			}
			return fmt.Sprintf("对战邀请\n%s → %s\n胜+300金币 / 败−300金币\n对战期间不能进行冒险\n断线按认输处理\n90秒未接受自动取消", o.FromName, o.ToName)
		}
	}
	return "正在等待消息服务响应…"
}
func (m *Model) handlePokemonSocialInput(input string) (bool, tea.Cmd) {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "l" || input == "link" {
		if m.pokemonOffer != nil {
			m.pokemonOpen("net-offer", 0)
			return true, nil
		}
		if m.pokemonGame.Battle != nil || m.pokemonGame.Busy() {
			m.status = "先结束本地战斗再进入联机中心"
			return true, nil
		}
		m.pokemonOpen("online", 0)
		return true, m.pokemonSync()
	}
	if !strings.HasPrefix(input, "net ") {
		return false, nil
	}
	w := strings.Fields(input)
	if len(w) < 2 {
		return true, nil
	}
	switch w[1] {
	case "pause":
		m.pokemonSocialReady = false
		m.pokemonPanel = "center"
		return true, m.socialSend(map[string]any{"type": "pokemon_pause"})
	case "sync":
		return true, m.pokemonSync()
	case "roster":
		m.pokemonPanel = "online"
		return true, m.socialSend(map[string]any{"type": "pokemon_roster"})
	case "player":
		if len(w) > 2 {
			m.pokemonSocialTarget = w[2]
			m.pokemonOpen("net-player", 0)
		}
	case "own":
		if len(w) > 2 {
			n, err := strconv.Atoi(w[2])
			if err != nil || n < 0 || n >= len(m.pokemonGame.Party) {
				m.status = "自己的伙伴编号无效"
				return true, nil
			}
			m.pokemonTradeSlots[0] = n
			m.pokemonOpen("net-foe", 0)
		}
	case "foe":
		if len(w) > 2 {
			n, err := strconv.Atoi(w[2])
			valid := false
			for _, p := range m.pokemonRoster {
				if p.PlayerID == m.pokemonSocialTarget && n >= 0 && n < len(p.Party) {
					valid = true
				}
			}
			if err != nil || !valid {
				m.status = "对方伙伴编号无效"
				return true, nil
			}
			m.pokemonTradeSlots[1] = n
			m.pokemonOpen("net-confirm", 0)
		}
	case "duel", "trade":
		if !m.pokemonSocialReady {
			m.status = "请先L进入联机中心并同步"
			return true, nil
		}
		m.pokemonSocialPending = true
		m.status = "正在发出邀请，队伍和金币暂时锁定"
		return true, m.socialSend(map[string]any{"type": "pokemon_invite", "targetId": m.pokemonSocialTarget, "data": map[string]any{"kind": w[1], "slots": m.pokemonTradeSlots}})
	case "accept":
		if m.pokemonOffer != nil {
			m.pokemonSocialPending = true
			return true, m.socialSend(map[string]any{"type": "pokemon_reply", "data": map[string]any{"id": m.pokemonOffer.ID}})
		}
	case "cancel":
		m.pokemonSocialPending = true
		return true, m.socialSend(map[string]any{"type": "pokemon_cancel"})
	}
	return true, nil
}
func (m *Model) pokemonSync() tea.Cmd {
	if m.pokemonSocialPending {
		m.status = "正在同步或发出邀请，请等待回执"
		return nil
	}
	if !m.socialReady {
		return m.connectSocial()
	}
	if m.pokemonGame == nil || m.pokemonLoadError || m.pokemonGame.Battle != nil || m.pokemonGame.Busy() || len(m.pokemonGame.Party) == 0 {
		m.status = "领取伙伴并结束本地战斗后才能同步"
		return nil
	}
	if m.pokemonOffer != nil || m.pokemonDuel != nil && !m.pokemonDuel.Finished {
		m.status = "先完成当前邀请或对战"
		return nil
	}
	s := pokemon.CloneState(m.pokemonGame.State)
	if s.MultiplayerServer != m.address {
		s.MultiplayerRevision = 0
	}
	s.MultiplayerServer = m.address
	m.pokemonSocialPending = true
	m.pokemonSocialReady = false
	return m.socialSend(map[string]any{"type": "pokemon_sync", "data": map[string]any{"state": s, "revision": s.MultiplayerRevision}})
}
func (m *Model) pokemonSocialEvent(e netclient.Envelope) tea.Cmd {
	if m.pokemonGame == nil {
		if e.Type == "pokemon_offer" {
			return m.socialSend(map[string]any{"type": "pokemon_cancel"})
		}
		return nil
	}
	switch e.Type {
	case "pokemon_error":
		m.pokemonSocialPending = false
		m.status = e.Error
		m.pokemonGame.Notice(e.Error)
	case "pokemon_profile":
		var data struct {
			State    pokemon.State
			Revision uint64
			Note     string
		}
		if json.Unmarshal(e.Data, &data) != nil || pokemon.ValidateState(data.State) != nil {
			m.status = "联机档案校验失败，保留本地存档"
			return nil
		}
		m.pokemonGame.State = data.State
		m.pokemonRecover = false
		m.pokemonGame.MultiplayerRevision = data.Revision
		m.pokemonGame.MultiplayerServer = m.address
		m.pokemonGame.Notice(data.Note)
		m.pokemonSocialReady = true
		m.pokemonSocialPending = false
		m.pokemonCache = pokemonRenderCache{}
		m.pokemonNav = pokemonNavigationCache{}
		m.savePokemon()
		return m.socialSend(map[string]any{"type": "pokemon_roster"})
	case "pokemon_roster":
		_ = json.Unmarshal(e.Data, &m.pokemonRoster)
		m.pokemonNav = pokemonNavigationCache{}
	case "pokemon_offer":
		var o pokemonInvite
		if json.Unmarshal(e.Data, &o) != nil {
			return nil
		}
		if m.pokemonGame.Battle != nil || m.pokemonGame.Busy() {
			return m.socialSend(map[string]any{"type": "pokemon_cancel"})
		}
		if o.To == m.player.ID && !m.pokemonSocialReady {
			m.status = "邀请已拒绝：请先L进入联机中心同步最新队伍"
			return m.socialSend(map[string]any{"type": "pokemon_cancel"})
		}
		m.pokemonOffer = &o
		m.pokemonSocialPending = false
		m.pokemonOpen("net-offer", 0)
		m.pokemonGame.Notice("收到联机邀请：" + o.FromName + " → " + o.ToName)
		m.status = "L联机邀请 · 1接受 / 取消选项拒绝"
	case "pokemon_offer_done":
		m.pokemonOffer = nil
		m.pokemonSocialPending = false
		m.pokemonPanel = "online"
		m.pokemonGame.Notice(e.Text)
		return m.socialSend(map[string]any{"type": "pokemon_roster"})
	case "pokemon_duel":
		var d pokemon.DuelView
		if json.Unmarshal(e.Data, &d) != nil {
			return nil
		}
		m.pokemonOffer = nil
		m.pokemonSocialPending = false
		m.pokemonDuel = &d
		m.pokemonInput.SetValue("")
		m.status = "1–4技能 + Enter · S 编号换人 · F6/F8消息 · Del认输"
		if d.Finished {
			m.pokemonGame.Notice("玩家对战已结束，请查看金币结算；Enter返回联机中心")
			if m.pokemonDuelExit {
				m.pokemonDuelExit = false
				m.pokemonDuel = nil
				return m.pokemonKey(tea.KeyPressMsg{Code: tea.KeyDelete})
			}
		}
	}
	return nil
}
func (m *Model) pokemonDuelKey(message tea.KeyMsg) tea.Cmd {
	key := message.String()
	if p, ok := message.(tea.KeyPressMsg); ok && p.IsRepeat && key == "enter" {
		return nil
	}
	if key == "delete" {
		if m.pokemonDuel.Finished {
			m.pokemonDuel = nil
			return m.pokemonKey(message)
		}
		m.pokemonDuelExit = true
		return m.socialSend(map[string]any{"type": "pokemon_cancel"})
	}
	if key == "f9" {
		_, err := m.sound.Toggle()
		if err != nil {
			m.status = err.Error()
		}
		return nil
	}
	if key == "enter" {
		if m.pokemonDuel.Finished {
			m.pokemonDuel = nil
			m.pokemonPanel = "online"
			m.pokemonInput.SetValue("")
			return m.socialSend(map[string]any{"type": "pokemon_roster"})
		}
		input := strings.Fields(strings.ToLower(strings.TrimSpace(m.pokemonInput.Value())))
		m.pokemonInput.SetValue("")
		if len(input) == 0 {
			return nil
		}
		kind := "move"
		slotText := input[0]
		if input[0] == "s" || input[0] == "switch" {
			if len(input) < 2 {
				m.status = "输入 S 2 + Enter，换上第二只伙伴"
				return nil
			}
			kind = "switch"
			slotText = input[1]
		}
		n, err := strconv.Atoi(slotText)
		if err != nil {
			m.status = "技能1–4；换人 S 1–6"
			return nil
		}
		return m.socialSend(map[string]any{"type": "pokemon_action", "data": map[string]any{"action": pokemon.DuelAction{Kind: kind, Slot: n - 1}}})
	}
	var cmd tea.Cmd
	m.pokemonInput, cmd = m.pokemonInput.Update(message)
	return cmd
}
func (m *Model) pokemonDuelView() string {
	d := m.pokemonDuel
	if d == nil {
		return "等待对战…"
	}
	rows := []string{titleStyle.Render(fmt.Sprintf("宝可梦 / 玩家对战 · 回合 %d", d.Turn)), fmt.Sprintf("%s VS %s · 胜+300 / 败−300金币", d.Names[d.Seat], d.Names[1-d.Seat]), "", selected.Render(fmt.Sprintf("对手 %s Lv%d · 剩余%d/%d只", d.Foe.Name(), d.Foe.Level, d.FoeRemaining, d.FoeTotal)), hpBar(d.Foe.HP, d.Foe.MaxHP()) + " " + d.Foe.Status, ""}
	if d.Active < len(d.Team) {
		p := d.Team[d.Active]
		rows = append(rows, selected.Render(fmt.Sprintf("你 %s Lv%d", p.Name(), p.Level)), hpBar(p.HP, p.MaxHP())+" "+p.Status)
		for i, id := range p.Moves {
			if id >= 0 && id < len(pokemon.Moves) {
				mv := pokemon.Moves[id]
				rows = append(rows, fmt.Sprintf("%d. %s PP %d/%d · %s", i+1, mv.Name, p.PP[i], mv.PP, pokemon.MoveDescription(id)))
			}
		}
	}
	party := []string{}
	for i, p := range d.Team {
		party = append(party, fmt.Sprintf("%d:%s %d/%d", i+1, p.Name(), p.HP, p.MaxHP()))
	}
	rows = append(rows, "队伍 "+strings.Join(party, " · "), "")
	if m.height < 24 {
		compact := []string{}
		for _, r := range rows {
			if r != "" {
				compact = append(compact, r)
			}
		}
		rows = compact
	}
	logs := d.Log
	logCount := min(4, max(0, m.height-len(rows)-4))
	if len(logs) > logCount {
		logs = logs[len(logs)-logCount:]
	}
	rows = append(rows, logs...)
	if d.Finished {
		rows = append(rows, selected.Render("对战结束，Enter返回联机中心"))
	} else if d.Submitted {
		rows = append(rows, muted.Render("选择已锁定，等待对手提交；双方技能同时揭晓"))
	}
	rows = append(rows, statusStyle.Render(m.status), m.pokemonInput.View(), muted.Render("1–4技能 / S 编号换人 + Enter · Del认输 · F6全服/F8私聊"))
	for i, row := range rows {
		rows[i] = ansi.Truncate(row, max(16, m.width-2), "…")
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(strings.Join(rows, "\n"))
}
