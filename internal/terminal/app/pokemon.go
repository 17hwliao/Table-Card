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
)

type pokemonTickMsg struct {
	generation uint64
	step       int
}
type pokemonRenderCache struct {
	game     *pokemon.Game
	revision uint64
	width    int
	header   string
	lines    []string
}

func NewPokemon(name string) *Model { m := New("", name); m.startPokemon(); return m }
func NewPokemonWithServer(address, name string) *Model {
	m := New(address, name)
	m.startPokemon()
	return m
}
func (m *Model) startPokemon() tea.Cmd {
	m.pokemonSocialReady, m.pokemonSocialPending, m.pokemonRecover, m.pokemonDuelExit = false, false, false, false
	m.pokemonOffer = nil
	m.pokemonDuel = nil
	m.pokemonRoster = nil
	m.pokemonCache = pokemonRenderCache{}
	m.pokemonNav = pokemonNavigationCache{}
	m.pokemonHistory = nil
	m.pokemonPanel, m.pokemonTarget, m.pokemonOptionPage, m.pokemonInfoOffset = "root", 0, 0, 0
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
	m.status = "数字 / 字母 + Enter 选择 · M 地图 · C 城市中心"
	m.pokemonInput = textinput.New()
	m.pokemonInput.Prompt = "选择 › "
	m.pokemonInput.Placeholder = "1 / M / C / H + Enter"
	m.pokemonInput.CharLimit = 100
	m.pokemonInput.SetWidth(max(10, m.width-14))
	if err != nil {
		m.pokemonGame.Notice("存档加载失败：" + err.Error())
		m.pokemonGame.Notice("原存档保留。先备份，再输入 new yes 重置；Del 返回。")
		m.pokemonPanel = "reset"
	} else {
		m.pokemonGame.Notice("数字菜单选择；H 查看指南。F5存档 / F9音乐 / Del保存返回。")
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
	if !m.pokemonGame.Busy() {
		m.pokemonPanel = "root"
		if m.pokemonGame.Battle != nil {
			m.pokemonPanel = "battle"
		}
		m.pokemonOptionPage, m.pokemonInfoOffset = 0, 0
		m.pokemonHistory = nil
	}
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
	if m.pokemonDuel != nil {
		return m.pokemonDuelKey(message)
	}
	if pressed, ok := message.(tea.KeyPressMsg); ok && pressed.IsRepeat && (key == "enter" || key == "delete" || key == "f9" || key == "f5" || key == "ctrl+s") {
		return nil
	}
	if m.overlay != "" && key != "delete" {
		m.overlayKey(key)
		return nil
	}
	switch key {
	case "delete":
		if m.pokemonOffer != nil || m.pokemonSocialPending {
			m.status = "正在取消联机邀请；收到取消回执后可Del返回"
			return m.socialSend(map[string]any{"type": "pokemon_cancel"})
		}
		if !m.savePokemon() {
			return nil
		}
		m.pokemonGeneration++
		m.pokemonGame = nil
		m.pokemonCache = pokemonRenderCache{}
		m.pokemonNav = pokemonNavigationCache{}
		m.pokemonHistory = nil
		m.pokemonLoadError = false
		m.overlay = ""
		m.page = homeScreen
		m.menuOn = true
		m.pokemonInput.Blur()
		m.status = "冒险已保存，返回宝可梦菜单"
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
			m.status = "声音已开启 · " + m.sound.Track().Title + " · F9关闭"
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
		m.pokemonBack()
		return nil
	case "[", "left":
		m.pokemonOptionPage = max(0, m.pokemonOptionPage-1)
		return nil
	case "]", "right":
		m.pokemonOptionPage = min(max(0, (len(m.pokemonOptions())-1)/m.pokemonPageSize()), m.pokemonOptionPage+1)
		return nil
	case "up":
		m.pokemonInfoOffset = max(0, m.pokemonInfoOffset-1)
		return nil
	case "down":
		m.pokemonInfoOffset++
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
		m.pokemonInput.SetValue("")
		m.pokemonLogOffset = 0
		return m.pokemonChoose(input)
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
	body := titleStyle.Render("宝可梦 / 关都文字冒险") + "\n\n" + muted.Render("单机冒险 + 联机对战/交换 · 151种图鉴 · 按昵称存档") + "\n\n1. 继续冒险 / 首次开始\n2. 指令与规则\n3. 返回游戏选择\n\n玩家：" + m.name.Value()
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
