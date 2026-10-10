package app

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/17hwliao/table-card-independent/internal/snake"
	"github.com/17hwliao/table-card-independent/internal/table"
	modeui "github.com/17hwliao/table-card-independent/internal/terminal/ui"
)

type snakeTickMsg struct{ generation, step uint64 }

func localModes() []modeInfo {
	return []modeInfo{{ID: table.SnakeMode, Name: "贪吃蛇", MinSeats: 1, MaxSeats: 1, Progress: "本地单机 · 离线可玩 · 方向键控制"}, {ID: table.PokemonMode, Name: "宝可梦文字冒险", MinSeats: 1, MaxSeats: 1, Progress: "关都冒险 · 联机对战 / 交换 · 自动存档"}}
}

func NewSnakeWithServer(address, name string) *Model {
	m := New(address, name)
	m.startSnake()
	return m
}

// NewSnake opens the same terminal application directly in its offline mode.
func NewSnake(name string) *Model {
	m := New("", name)
	m.startSnake()
	return m
}

func (m *Model) startSnake() {
	m.snakeGeneration++
	m.snakeStep = 0
	m.snakeGame = snake.New(time.Now().UnixNano())
	m.snakeStarted, m.snakePaused = false, false
	m.page = snakeScreen
	m.overlay = ""
	m.chatOn = false
	m.status = "按 Enter / 空格开始"
	if err := m.sound.SetMode(table.SnakeMode); err != nil {
		m.status = "声音加载失败：" + err.Error()
	}
}
func (m *Model) snakeTimer() tea.Cmd {
	if m.snakeGame == nil || !m.snakeStarted || m.snakePaused || m.snakeGame.Finished() {
		return nil
	}
	generation, step := m.snakeGeneration, m.snakeStep
	return tea.Tick(m.snakeGame.Interval(), func(time.Time) tea.Msg { return snakeTickMsg{generation, step} })
}
func (m *Model) snakeTick(msg snakeTickMsg) tea.Cmd {
	if m.page != snakeScreen || msg.generation != m.snakeGeneration || msg.step != m.snakeStep || m.snakeGame == nil || !m.snakeStarted || m.snakePaused || m.snakeGame.Finished() {
		return nil
	}
	m.snakeStep++
	if m.snakeGame.Step() {
		m.sound.Effect(20)
	}
	state := m.snakeGame.Snapshot()
	m.snakeBest = max(m.snakeBest, state.Score)
	if state.Finished {
		m.status = state.Reason + " · F2 / R 再开 · Del 返回"
		m.sound.Effect(3)
		return nil
	}
	return m.snakeTimer()
}
func (m *Model) pauseSnake() {
	if m.snakeGame == nil || !m.snakeStarted || m.snakeGame.Finished() || m.snakePaused {
		return
	}
	m.snakePaused = true
	m.snakeGeneration++ // invalidate the timer already in flight
	m.snakeGame.ClearTurns()
	m.status = "已暂停 · P / 空格继续"
}
func (m *Model) snakeKey(key string) tea.Cmd {
	switch key {
	case "delete", "q":
		m.snakeGeneration++
		m.snakeGame = nil
		m.page = homeScreen
		m.menuOn = true
		m.status = "已返回贪吃蛇菜单"
		for i, item := range m.modes {
			if item.ID == table.SnakeMode {
				m.mode = i
				m.seats = 1
				break
			}
		}
		return nil
	case "f2", "r":
		m.startSnake()
		return nil
	case "esc":
		m.pauseSnake()
		return nil
	case "p", "space", " ", "enter":
		if m.snakeGame == nil || m.snakeGame.Finished() {
			return nil
		}
		if !m.snakeStarted || m.snakePaused {
			m.snakeStarted = true
			m.snakePaused = false
			m.snakeGeneration++
			m.snakeGame.ClearTurns()
			m.status = "方向键 / WASD 控制 · P 暂停"
			return m.snakeTimer()
		}
		if key != "enter" {
			m.pauseSnake()
		}
		return nil
	}
	if m.snakeGame == nil || m.snakePaused || !m.snakeStarted {
		return nil
	}
	switch key {
	case "up", "w":
		m.snakeGame.Turn(snake.Up)
	case "right", "d":
		m.snakeGame.Turn(snake.Right)
	case "down", "s":
		m.snakeGame.Turn(snake.Down)
	case "left", "a":
		m.snakeGame.Turn(snake.Left)
	}
	return nil
}
func (m *Model) snakeMenuKey(key string) tea.Cmd {
	switch key {
	case "1", "enter", "space", " ":
		m.startSnake()
	case "2", "f1", "?":
		m.overlay = rulesFor(table.SnakeMode)
	case "3", "esc", "8":
		m.menuOn = false
	}
	return nil
}
func (m *Model) snakeLobbyView() string {
	body := titleStyle.Render("贪吃蛇 / SNAKE") + "\n\n" + muted.Render("本地单机 · 无需联网或启动服务端") + "\n\n1. 开始游戏\n2. 查看规则\n3. 返回游戏选择\n\n" + fmt.Sprintf("本次最高分 %d", m.snakeBest) + "\n\n" + muted.Render("Enter 开始 · F1 规则 · Esc 返回 · M 音乐")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

var snakeBodyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7BC99A"))
var snakeHeadStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E9B44C")).Bold(true)
var snakeFoodStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F17783"))

func (m *Model) snakeView() string {
	if m.snakeGame == nil {
		return "正在准备贪吃蛇…"
	}
	state := m.snakeGame.Snapshot()
	cellWidth := 2
	if m.width < snake.Width*2+4 {
		cellWidth = 1
	}
	blank, body, food := "  ", snakeBodyStyle.Render("██"), snakeFoodStyle.Render("◆ ")
	head := snakeHeadStyle.Render([]string{"▲ ", "▶ ", "▼ ", "◀ "}[state.Direction])
	if cellWidth == 1 {
		blank = " "
		body = snakeBodyStyle.Render("█")
		food = snakeFoodStyle.Render("◆")
		head = snakeHeadStyle.Render([]string{"▲", "▶", "▼", "◀"}[state.Direction])
	}
	edge := muted.Render("│")
	var b strings.Builder
	b.Grow(snake.Width * snake.Height * cellWidth * 4)
	b.WriteString(muted.Render("┌"+strings.Repeat("─", snake.Width*cellWidth)+"┐") + "\n")
	for y := 0; y < snake.Height; y++ {
		b.WriteString(edge)
		for x := 0; x < snake.Width; x++ {
			point := snake.Point{X: x, Y: y}
			switch {
			case point == state.Head:
				b.WriteString(head)
			case state.Occupied[y][x]:
				b.WriteString(body)
			case !state.Won && point == state.Food:
				b.WriteString(food)
			default:
				b.WriteString(blank)
			}
		}
		b.WriteString(edge + "\n")
	}
	b.WriteString(muted.Render("└" + strings.Repeat("─", snake.Width*cellWidth) + "┘"))
	phase := "准备开始"
	if m.snakeStarted {
		phase = "游玩中"
	}
	if m.snakePaused {
		phase = "已暂停"
	}
	if state.Finished {
		phase = "游戏结束 · " + state.Reason
	}
	if state.Won {
		phase = "棋盘填满 · 胜利"
	}
	title := titleStyle.Render("牌桌 / 贪吃蛇 · 本地单机")
	score := fmt.Sprintf("分数 %d · 长度 %d · 等级 %d · 本次最高 %d", state.Score, state.Length, state.Level, m.snakeBest)
	content := modeui.JoinLeft(title, muted.Render(score), snakeHeadStyle.Render(phase), b.String(), statusStyle.Render(m.status), muted.Render("方向键 / WASD · P/空格 暂停 · F2 重开\nDel 返回菜单 · F1 规则 · M 声音"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
