package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/17hwliao/table-card-independent/internal/pokemon"
	"github.com/charmbracelet/x/ansi"
	"strconv"
	"strings"
)

type pokemonMenuLocation struct {
	panel        string
	target, page int
}
type pokemonNavigationCache struct {
	game      *pokemon.Game
	revision  uint64
	panel     string
	target    int
	choices   []pokemon.Choice
	info      string
	infoWidth int
	infoLines []string
}

func (m *Model) pokemonOptions() []pokemon.Choice {
	g := m.pokemonGame
	if g == nil {
		return nil
	}
	if m.pokemonPanel == "" {
		m.pokemonPanel = "root"
	}
	if g.Battle != nil && (m.pokemonPanel == "root" || m.pokemonPanel == "battle") {
		m.pokemonPanel = "battle"
	}
	if g.Battle == nil && m.pokemonPanel == "battle" {
		m.pokemonPanel = "root"
	}
	c := &m.pokemonNav
	if c.game != g || c.revision != g.LogRevision() || c.panel != m.pokemonPanel || c.target != m.pokemonTarget {
		*c = pokemonNavigationCache{game: g, revision: g.LogRevision(), panel: m.pokemonPanel, target: m.pokemonTarget, choices: g.Choices(m.pokemonPanel, m.pokemonTarget)}
	}
	return c.choices
}
func (m *Model) pokemonWide() bool { return m.width >= 96 && m.height >= 28 }
func (m *Model) pokemonLogRows() int {
	if m.pokemonWide() {
		return min(8, max(3, m.height/5))
	}
	return 3
}
func (m *Model) pokemonBodyRows() int { return max(4, m.height-9-m.pokemonLogRows()) }
func (m *Model) pokemonPageSize() int {
	if m.pokemonWide() {
		return min(9, max(1, m.pokemonBodyRows()-2))
	}
	infoRows := min(4, max(1, m.pokemonBodyRows()-4))
	if m.pokemonPanel == "map" || m.pokemonPanel == "world" {
		infoRows = min(7, max(1, m.pokemonBodyRows()-3))
	}
	return max(1, min(9, m.pokemonBodyRows()-infoRows-2))
}
func (m *Model) pokemonOpen(panel string, target int) {
	if panel == "" {
		return
	}
	if m.pokemonPanel != panel || m.pokemonTarget != target {
		if len(m.pokemonHistory) >= 16 {
			copy(m.pokemonHistory, m.pokemonHistory[1:])
			m.pokemonHistory = m.pokemonHistory[:15]
		}
		m.pokemonHistory = append(m.pokemonHistory, pokemonMenuLocation{m.pokemonPanel, m.pokemonTarget, m.pokemonOptionPage})
	}
	m.pokemonPanel, m.pokemonTarget, m.pokemonOptionPage, m.pokemonInfoOffset = panel, target, 0, 0
}
func (m *Model) pokemonBack() {
	g := m.pokemonGame
	if g.Battle != nil {
		m.pokemonPanel = "battle"
		m.pokemonTarget = 0
		m.pokemonOptionPage = 0
		m.pokemonInfoOffset = 0
		m.pokemonHistory = nil
		return
	}
	if n := len(m.pokemonHistory); n > 0 {
		last := m.pokemonHistory[n-1]
		m.pokemonHistory = m.pokemonHistory[:n-1]
		m.pokemonPanel, m.pokemonTarget, m.pokemonOptionPage = last.panel, last.target, last.page
		m.pokemonInfoOffset = 0
	} else {
		m.pokemonPanel = "root"
		m.pokemonOptionPage = 0
		m.pokemonInfoOffset = 0
	}
}
func (m *Model) pokemonChoose(input string) tea.Cmd {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "0" || input == "back" {
		m.pokemonBack()
		return nil
	}
	if input == "[" || input == "]" {
		delta := 1
		if input == "[" {
			delta = -1
		}
		m.pokemonOptionPage = max(0, min(max(0, (len(m.pokemonOptions())-1)/m.pokemonPageSize()), m.pokemonOptionPage+delta))
		return nil
	}
	command, next, target := input, "", 0
	if n, err := strconv.Atoi(input); err == nil {
		choices := m.pokemonOptions()
		index := m.pokemonOptionPage*m.pokemonPageSize() + n - 1
		if n < 1 || n > m.pokemonPageSize() || index < 0 || index >= len(choices) {
			m.status = "请选择本页列出的编号；[ / ] 翻页"
			return nil
		}
		choice := choices[index]
		if !choice.Enabled {
			m.status = "该选项当前不可用；查看灰色标记或指南中的解锁条件"
			return nil
		}
		command, next, target = choice.Command, choice.Panel, choice.Target
	} else if !strings.Contains(input, " ") {
		shortcuts := map[string]string{"m": "map", "map": "map", "w": "world", "c": "center", "h": "guide", "help": "guide", "p": "party", "party": "party", "b": "bag", "bag": "bag", "q": "quest", "quest": "quest", "shop": "shop", "box": "pc", "dex": "dex"}
		if panel, ok := shortcuts[input]; ok {
			if m.pokemonGame.Battle != nil && panel != "party" && panel != "bag" && panel != "guide" {
				m.status = "战斗中：1–4招式；B背包 / P队伍 / catch 1 投球 / run 逃跑"
				return nil
			}
			if m.pokemonLoadError {
				m.status = "原存档保留；先备份，再通过重置菜单确认，或Del返回"
				return nil
			}
			visit := map[string]string{"center": "visit 1", "shop": "visit 6", "quest": "visit 8"}[panel]
			if visit != "" && m.pokemonGame.Battle == nil {
				if err := m.pokemonGame.Command(visit); err != nil {
					m.status = err.Error()
					return nil
				}
			}
			m.pokemonOpen(panel, 0)
			if visit == "" || m.savePokemon() {
				m.status = "菜单已打开 · 本页数字+Enter · 0 / Esc 返回"
			}
			return nil
		}
	}
	if command == "" {
		m.pokemonOpen(next, target)
		m.status = "菜单已打开 · 本页数字+Enter · 0 / Esc 返回"
		return nil
	}
	if m.pokemonLoadError && command != "new yes" && command != "新冒险 确认" {
		m.status = "备份原存档后输入 new yes 确认重置，或Del返回"
		return nil
	}
	if command == "new yes" || command == "新冒险 确认" {
		m.pokemonGeneration++
		m.pokemonGame = pokemon.New(strings.TrimSpace(m.name.Value()))
		m.pokemonLoadError = false
		m.pokemonCache = pokemonRenderCache{}
		m.pokemonNav = pokemonNavigationCache{}
		m.pokemonHistory = nil
		m.pokemonPanel, m.pokemonTarget, m.pokemonOptionPage, m.pokemonInfoOffset = "root", 0, 0, 0
	} else {
		if err := m.pokemonGame.Command(command); err != nil {
			m.pokemonGame.Notice(err.Error())
			m.status = err.Error()
			m.savePokemon()
			return nil
		}
		if next != "" {
			m.pokemonOpen(next, target)
		}
	}
	if m.pokemonGame.Battle != nil && (next == "battle" || m.pokemonPanel == "root" || m.pokemonPanel == "gym-site" || m.pokemonPanel == "story-site" || m.pokemonPanel == "quest") {
		m.pokemonPanel = "battle"
		m.pokemonOptionPage = 0
	}
	if m.pokemonGame.Battle == nil && (m.pokemonPanel == "battle" || m.pokemonPanel == "balls" || m.pokemonPanel == "switch") {
		m.pokemonPanel = "root"
		m.pokemonOptionPage = 0
	}
	if m.pokemonGame.Battle != nil && (m.pokemonPanel == "bag" || m.pokemonPanel == "use-target") && strings.HasPrefix(command, "use ") {
		m.pokemonPanel = "battle"
		m.pokemonOptionPage = 0
	}
	m.status = "操作已完成 · 0 / Esc 返回 · H 指南 · M 地图 · C 城市中心"
	saved := m.savePokemon()
	if m.pokemonGame.Busy() {
		if !saved {
			m.status += "；投球仍继续"
		}
		return m.pokemonTimer()
	}
	return nil
}

func (m *Model) pokemonView() string {
	g := m.pokemonGame
	if g == nil {
		return "准备文字冒险…"
	}
	if m.width < 42 || m.height < 20 {
		return "请把终端调整到至少42列×20行\n" + m.pokemonInput.View() + "\nDel 保存返回 / F5 存档"
	}
	width := m.width - 4
	if m.pokemonCache.game != g || m.pokemonCache.revision != g.LogRevision() || m.pokemonCache.width != width {
		header := titleStyle.Render("牌桌 / 宝可梦 · "+g.AreaName()) + "\n" + muted.Render(fmt.Sprintf("%s · $%d · 挑战%d/6 · 徽章%d/8 · 支线%d/12", g.Name, g.Money, g.TrainerWins[g.Area], len(g.Badges), g.QuestCount()))
		header += "\n" + ansi.Truncate("目标："+g.Objective(), width, "… [H指南]")
		var logs []string
		for _, entry := range g.Log {
			logs = append(logs, strings.Split(ansi.Wrap(entry, width, ""), "\n")...)
		}
		m.pokemonCache = pokemonRenderCache{game: g, revision: g.LogRevision(), width: width, header: header, lines: logs}
	}
	choices := m.pokemonOptions()
	size := m.pokemonPageSize()
	pages := max(1, (len(choices)+size-1)/size)
	m.pokemonOptionPage = min(m.pokemonOptionPage, pages-1)
	menuWidth := width
	if m.pokemonWide() {
		menuWidth = width - 37
	}
	menu := []string{titleStyle.Render(ansi.Truncate(g.PanelTitle(m.pokemonPanel, m.pokemonTarget), menuWidth, "…"))}
	for i := 0; i < size; i++ {
		index := m.pokemonOptionPage*size + i
		if index >= len(choices) {
			break
		}
		c := choices[index]
		label := fmt.Sprintf(" %d  %s", i+1, c.Label)
		if !c.Enabled {
			label += " [不可用]"
			menu = append(menu, muted.Render(ansi.Truncate(label, menuWidth, "…")))
		} else {
			menu = append(menu, ansi.Truncate(label, menuWidth, "…"))
		}
	}
	if len(choices) == 0 {
		menu = append(menu, muted.Render("这里暂无物品或伙伴；0 返回"))
	}
	menu = append(menu, muted.Render(fmt.Sprintf("第%d/%d页 · [ / ] 翻页 · 0 返回", m.pokemonOptionPage+1, pages)))
	infoWidth := width
	infoRows := min(4, max(1, m.pokemonBodyRows()-4))
	if m.pokemonWide() {
		infoWidth = 33
		infoRows = m.pokemonBodyRows()
	}
	var info string
	if b := g.Battle; b != nil {
		p, f := g.Party[b.Active], b.Foes[b.Enemy]
		info = fmt.Sprintf("对手 %s Lv%d\n%s [%s]\n伙伴 %s Lv%d\n%s [%s]\n对战：%s", f.Name(), f.Level, hpBar(f.HP, f.MaxHP()), f.Status, p.Name(), p.Level, hpBar(p.HP, p.MaxHP()), p.Status, b.Name)
		if g.Busy() {
			info += "\n精灵球 " + strings.Repeat("● ", g.Capture.Step) + strings.Repeat("○ ", 3-g.Capture.Step) + "摇动中"
		}
		if m.pokemonPanel != "battle" {
			info += "\n" + g.PanelInfo(m.pokemonPanel, m.pokemonTarget)
		}
	} else if m.pokemonPanel == "map" || m.pokemonPanel == "world" {
		info = g.RegionMap()
		if m.pokemonPanel == "world" {
			info = g.WorldMap() + "\n<编号> 当前位置 · 解锁后可旅行"
		}
		if !m.pokemonWide() {
			infoRows = min(7, max(1, m.pokemonBodyRows()-3))
		}
	} else if m.pokemonWide() && (m.pokemonPanel == "center" || m.pokemonPanel == "root" || m.pokemonPanel == "shop") {
		info = g.RegionMap() + "\n<编号> 当前建筑 / X 出城旅行\n" + g.PanelInfo(m.pokemonPanel, m.pokemonTarget)
	} else {
		info = g.PanelInfo(m.pokemonPanel, m.pokemonTarget)
	}
	if m.pokemonNav.info != info || m.pokemonNav.infoWidth != infoWidth {
		m.pokemonNav.info, m.pokemonNav.infoWidth = info, infoWidth
		m.pokemonNav.infoLines = strings.Split(ansi.Wrap(info, infoWidth, ""), "\n")
	}
	wrapped := m.pokemonNav.infoLines
	if len(wrapped) > infoRows {
		visible := max(1, infoRows-1)
		m.pokemonInfoOffset = min(m.pokemonInfoOffset, max(0, len(wrapped)-visible))
		total := len(wrapped)
		portion := append([]string(nil), wrapped[m.pokemonInfoOffset:min(len(wrapped), m.pokemonInfoOffset+visible)]...)
		wrapped = append(portion, fmt.Sprintf("↑↓ 阅读 %d/%d", m.pokemonInfoOffset+1, total))
	} else {
		m.pokemonInfoOffset = 0
	}
	infoText := strings.Join(wrapped, "\n")
	body := infoText + "\n" + strings.Join(menu, "\n")
	if m.pokemonWide() {
		body = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(35).Render(infoText), " │ ", strings.Join(menu, "\n"))
	}
	body = lipgloss.NewStyle().Height(m.pokemonBodyRows()).Render(body)
	logs := m.pokemonCache.lines
	visibleLogs := m.pokemonLogRows() - 1
	m.pokemonLogOffset = min(m.pokemonLogOffset, max(0, len(logs)-visibleLogs))
	end := max(0, len(logs)-m.pokemonLogOffset)
	start := max(0, end-visibleLogs)
	log := muted.Render("最近记录 · PgUp / PgDn 翻阅") + "\n" + strings.Join(logs[start:end], "\n")
	log = lipgloss.NewStyle().Height(m.pokemonLogRows()).Render(log)
	footer := statusStyle.Render(ansi.Truncate(m.status, width, "…")) + "\n" + m.pokemonInput.View() + "\n" + muted.Render(ansi.Truncate("1–9+Enter M地图 C中心 H指南 0返回", width, "…")) + "\n" + muted.Render(ansi.Truncate("[ ]翻页 ↑↓读 F5存 F9音 Del退", width, "…"))
	content := m.pokemonCache.header + "\n" + muted.Render(strings.Repeat("─", width)) + "\n" + body + "\n" + log + "\n" + footer
	rows := strings.Split(content, "\n")
	for i, row := range rows {
		rows[i] = ansi.Truncate(row, width, "")
	}
	return strings.Join(rows, "\n")
}

const pokemonRules = `宝可梦 · 数字菜单文字冒险
输入本页数字 + Enter；[ / ] 翻页，0 / Esc 返回。
M 地区地图 / C 城市中心 / W 世界地图 / H 指南。
P 队伍 / B 背包 / Q 支线；字母也需 Enter。
初始伙伴1妙蛙种子 / 2小火龙 / 3杰尼龟。
主线每章六名训练者 → 剧情 → 道馆 → 下一章。
战斗1–4招式；菜单提供球种、药品、换人和逃跑。
catch 1–4 投球；三次摇球都通过才成功。
商店编号选商品、再选数量；全部货品列在各页。
buy pokeball 5 / use potion 1 / evolve thunderstone 1。
各地建筑4调查，留言板承接十二个原创支线。
Lv20且2徽章或3支线：城市中心教学原创专属技。
八徽章后挑战联盟五连战；治疗会重置连战。
↑↓ 阅读说明 / PgUp、PgDn 阅读记录。
F5存档 / F9声音 / Del保存返回；按昵称存档。
指南中的重置选项需确认；new yes 同样确认重置。
这是简化战斗、原创对话的关都改编，不是原版模拟器。`
