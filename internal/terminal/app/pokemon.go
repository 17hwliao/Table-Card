package app

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/17hwliao/table-card-independent/internal/pokemon"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/charmbracelet/x/ansi"
)

type pokemonTickMsg struct {
	generation uint64
	step       int
}

func NewPokemon(name string) *Model { m := New("", name); m.startPokemon(); return m }
func (m *Model) startPokemon() tea.Cmd {
	if !m.nameReady() {
		return nil
	}
	m.pokemonGeneration++
	m.pokemonLogOffset = 0
	game, err := pokemon.Load(strings.TrimSpace(m.name.Value()))
	m.pokemonGame = game
	m.pokemonLoadError = err != nil
	m.page = pokemonScreen
	m.overlay = ""
	m.chatOn = false
	m.status = "输入文字指令，按 Enter 执行；输入帮助查看命令"
	m.pokemonInput = textinput.New()
	m.pokemonInput.Prompt = "指令 › "
	m.pokemonInput.Placeholder = "选择 妙蛙种子 / 进草丛 / 挑战"
	m.pokemonInput.CharLimit = 100
	m.pokemonInput.SetWidth(max(10, m.width-14))
	if err != nil {
		m.pokemonGame.Notice("存档加载失败：" + err.Error())
		m.pokemonGame.Notice("原存档保留。备份后可输入 新冒险 确认 替换存档；Del 返回。")
	} else {
		m.pokemonGame.Notice("冒险已就绪。输入帮助查看命令；F5存档 / F9音乐 / Del保存返回。")
		m.savePokemon()
	}
	if err := m.sound.SetMode(table.PokemonMode); err != nil {
		m.status = "声音加载失败：" + err.Error()
	}
	return tea.Batch(m.pokemonInput.Focus(), m.pokemonTimer())
}
func (m *Model) savePokemon() bool {
	if m.pokemonGame == nil || m.pokemonLoadError {
		return true
	}
	if err := m.pokemonGame.Save(); err != nil {
		m.status = "存档失败：" + err.Error() + " · F5重试"
		return false
	}
	return true
}
func (m *Model) pokemonTimer() tea.Cmd {
	if m.pokemonGame == nil || !m.pokemonGame.Busy() || m.pokemonLoadError {
		return nil
	}
	generation, step := m.pokemonGeneration, m.pokemonGame.CaptureStep()
	return tea.Tick(750*time.Millisecond, func(time.Time) tea.Msg { return pokemonTickMsg{generation, step} })
}
func (m *Model) pokemonTick(msg pokemonTickMsg) tea.Cmd {
	if m.page != pokemonScreen || m.pokemonGame == nil || !m.pokemonGame.Busy() || msg.generation != m.pokemonGeneration || msg.step != m.pokemonGame.CaptureStep() {
		return nil
	}
	passed := m.pokemonGame.Capture.Passed[msg.step]
	m.pokemonGame.AdvanceCapture()
	m.pokemonLogOffset = 0
	if !passed {
		m.sound.Effect(3)
	} else if m.pokemonGame.Busy() {
		m.sound.Effect(8)
	} else {
		m.sound.Effect(20)
	}
	m.savePokemon()
	return m.pokemonTimer()
}
func (m *Model) pokemonKey(message tea.KeyMsg) tea.Cmd {
	key := message.String()
	if m.overlay != "" {
		if key == "esc" || key == "enter" || key == "f1" {
			m.overlay = ""
		}
		return nil
	}
	switch key {
	case "delete":
		if !m.savePokemon() {
			return nil
		}
		m.pokemonGeneration++
		m.pokemonGame = nil
		m.pokemonLoadError = false
		m.page = homeScreen
		m.menuOn = true
		m.pokemonInput.Blur()
		m.status = "冒险已保存，返回宝可梦菜单"
		_ = m.sound.SetMode("")
		for i, item := range m.modes {
			if item.ID == table.PokemonMode {
				m.mode = i
				m.seats = 1
				break
			}
		}
		return nil
	case "f1":
		m.overlay = pokemonRules
		return nil
	case "f5", "ctrl+s":
		if m.savePokemon() {
			if m.pokemonLoadError {
				m.status = "损坏的原存档仍保留，未覆盖"
			} else {
				m.status = "存档已保存"
			}
		}
		return nil
	case "f9":
		silent, err := m.sound.Toggle()
		if err != nil {
			m.status = "声音开启失败：" + err.Error()
		} else if silent {
			m.status = "声音已关闭"
		} else {
			m.status = "声音已开启 · F9关闭"
		}
		return nil
	case "pgup":
		m.pokemonLogOffset += 5
		return nil
	case "pgdown":
		m.pokemonLogOffset = max(0, m.pokemonLogOffset-5)
		return nil
	case "esc":
		m.pokemonInput.SetValue("")
		return nil
	case "enter":
		input := strings.TrimSpace(m.pokemonInput.Value())
		if input == "" {
			return nil
		}
		if m.pokemonGame.Busy() {
			m.status = "精灵球摇动中，请稍候再执行指令"
			return nil
		}
		if m.pokemonLoadError && input != "新冒险 确认" {
			m.status = "原存档加载失败；备份后输入 新冒险 确认，或Del返回"
			return nil
		}
		m.pokemonInput.SetValue("")
		m.pokemonLogOffset = 0
		if input == "新冒险 确认" {
			m.pokemonGeneration++
			m.pokemonGame = pokemon.New(strings.TrimSpace(m.name.Value()))
			m.pokemonLoadError = false
		} else if err := m.pokemonGame.Command(input); err != nil {
			m.pokemonGame.Notice(err.Error())
			m.status = err.Error()
			m.savePokemon()
			return nil
		}
		m.status = "指令已执行 · 输入帮助查看命令"
		saved := m.savePokemon()
		if m.pokemonGame.Busy() {
			if !saved {
				m.status += "；投球结果仍保留在当前会话"
			}
			return m.pokemonTimer()
		}
		return nil
	}
	var cmd tea.Cmd
	m.pokemonInput, cmd = m.pokemonInput.Update(message)
	return cmd
}
func (m *Model) pokemonMenuKey(key string) tea.Cmd {
	switch key {
	case "1", "enter":
		return m.startPokemon()
	case "2", "f1", "?":
		m.overlay = pokemonRules
	case "3", "8", "esc":
		m.menuOn = false
	case "e":
		m.editing = "name"
		return m.name.Focus()
	}
	return nil
}
func (m *Model) pokemonLobbyView() string {
	body := titleStyle.Render("宝可梦 / 关都文字冒险") + "\n\n" + muted.Render("本地单机 · 151种图鉴 · 八徽章与联盟 · 按昵称存档") + "\n\n1. 继续冒险 / 首次开始\n2. 指令与规则\n3. 返回游戏选择\n\n玩家：" + m.name.Value()
	if m.editing == "name" {
		body += "\n" + m.name.View()
	}
	body += "\n\n" + statusStyle.Render(m.status) + "\n" + muted.Render("E 改昵称选择存档 · Enter 继续 · Esc 返回 · M 音乐")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}
func hpBar(hp, total int) string {
	n := max(0, min(12, hp*12/max(1, total)))
	return strings.Repeat("█", n) + strings.Repeat("░", 12-n) + fmt.Sprintf(" %d/%d", hp, total)
}
func (m *Model) pokemonView() string {
	g := m.pokemonGame
	if g == nil {
		return "准备文字冒险…"
	}
	width := max(20, m.width-4)
	header := titleStyle.Render("牌桌 / 宝可梦文字冒险 · "+g.AreaName()) + "\n" + muted.Render(fmt.Sprintf("训练者 %s · 零钱 %d · 本章挑战 %d/6 · 徽章 %d/8 · 捕获 %d/151", g.Name, g.Money, g.TrainerWins[g.Area], len(g.Badges), len(g.Caught)))
	header += "\n" + ansi.Wrap("目标："+g.Objective(), width, "")
	if b := g.Battle; b != nil {
		p, f := g.Party[b.Active], b.Foes[b.Enemy]
		header += "\n" + selected.Render(fmt.Sprintf("[%s] 对手 %s Lv%d [%s] %s", b.Name, f.Name(), f.Level, f.Status, hpBar(f.HP, f.MaxHP())))
		header += "\n" + titleStyle.Render(fmt.Sprintf("你的 %s Lv%d [%s] %s", p.Name(), p.Level, p.Status, hpBar(p.HP, p.MaxHP())))
		var moves []string
		for i, id := range p.Moves {
			moves = append(moves, fmt.Sprintf("%d.%s(%dPP)", i+1, pokemon.Moves[id].Name, p.PP[i]))
		}
		header += "\n" + ansi.Wrap(strings.Join(moves, "  "), width, "")
		if c := g.Capture; c != nil {
			header += "\n" + statusStyle.Render("精灵球  "+strings.Repeat("● ", c.Step)+strings.Repeat("○ ", 3-c.Step)+" · 摇动中")
		}
	} else if len(g.Party) > 0 {
		var names []string
		for i, p := range g.Party {
			names = append(names, fmt.Sprintf("%d.%s Lv%d %d/%d", i+1, p.Name(), p.Level, p.HP, p.MaxHP()))
		}
		header += "\n" + ansi.Wrap(strings.Join(names, "  "), width, "")
	}
	var lines []string
	for _, entry := range g.Log {
		lines = append(lines, strings.Split(ansi.Wrap(entry, width, ""), "\n")...)
	}
	footer := ansi.Truncate(m.status, width, "…") + "\n" + m.pokemonInput.View() + "\n" + muted.Render("Enter执行 · 帮助 · PgUp/PgDn历史 · F5存档 · F9声音 · Del保存返回")
	rows := max(1, m.height-lipgloss.Height(header)-lipgloss.Height(footer)-3)
	m.pokemonLogOffset = min(m.pokemonLogOffset, max(0, len(lines)-rows))
	end := max(0, len(lines)-m.pokemonLogOffset)
	start := max(0, end-rows)
	log := strings.Join(lines[start:end], "\n")
	log = lipgloss.NewStyle().Height(rows).Render(log)
	return header + "\n" + muted.Render(strings.Repeat("─", width)) + "\n" + log + "\n" + footer
}

const pokemonRules = "宝可梦 · 关都文字冒险\n选择 妙蛙种子 / 小火龙 / 杰尼龟领取伙伴。\n进草丛遇敌；招式 1–4战斗；捕捉 精灵球投球。\n捕捉受球种、剩余HP、种族捕获率和异常状态影响。\n三次摇球都通过才成功；失败后野生精灵反击。\n挑战累计本章6名胜利，再用剧情、道馆、前进推动故事。\n商店 / 购买 精灵球 5 / 使用 伤药 1 / 治疗。\n队伍 / 背包 / 图鉴 1 / 电脑 / 存入 2 / 取出 1。\n进化 雷之石 1 / 交换 1；等级进化在升级时自动发生。\n八枚徽章后，联盟连续挑战四天王和冠军。\n治疗会重置联盟连战；药品不重置。\n自动存档；F5存档；F9音乐；Del保存返回。\n按昵称加载本地存档，离线可玩；新冒险 确认会覆盖存档。\n这是简化战斗与原创对话的关都改编，不是原版模拟器。"
