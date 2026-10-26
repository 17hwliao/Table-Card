package garden

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	game "github.com/17hwliao/table-card-independent/internal/garden"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
	"github.com/charmbracelet/x/ansi"
)

type Controller struct {
	row, col         int
	selected         game.PlantKind
	opponent, shovel bool
}

func New() *Controller               { return &Controller{selected: game.Peashooter} }
func (*Controller) Mode() table.Mode { return table.GardenMode }
func (c *Controller) Reset()         { *c = Controller{selected: game.Peashooter} }

var text = lipgloss.NewStyle().Foreground(lipgloss.Color("#93AB98"))
var gold = lipgloss.NewStyle().Foreground(lipgloss.Color("#F3D562")).Bold(true)
var warning = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8780")).Bold(true)
var grass = [2]string{"#183B29", "#204631"}
var plantColors = [11]string{"", "#F3D562", "#76DC83", "#DAB57E", "#FF7185", "#9BDAFF", "#F78443", "#61D691", "#C78EFF", "#D8BF90", "#B8E878"}

// Original character frames, independent from npvz's Emoji rendering.
var sprites = [11][3]string{
	{}, {" \\|/ ", " (o) ", " /|\\ "}, {" (@)>", "  |  ", " / \\ "},
	{" .--.", " |::|", " '--'"}, {" /\\  ", " oo  ", " \\__/"},
	{" (*)>", "  |  ", " / \\ "}, {"  /! ", " /## ", " \\_/ "},
	{"(@)>>", "  |  ", " / \\ "}, {" \\/  ", " <##>", " /|  "},
	{" /!  ", "(ooo)", "-----"}, {" .--.", "(o_o)", " \\_/ "},
}
var zombieHeads = [9]string{"(oo)", " /\\ ", "[##]", "(~~)", "(oo)", "/--/", "[==]", "[OO]", "|##|"}
var abbreviations = [11]string{"", "SUN", "PEA", "NUT", "BOM", "ICE", "HOT", "TWO", "EAT", "MIN", "SQH"}

func decode(s ui.Snapshot) (game.Snapshot, int, bool) {
	var v game.Snapshot
	if json.Unmarshal(s.Game, &v) != nil || len(v.Players) != 2 {
		return v, 0, false
	}
	for i, l := range v.Players {
		if l.ID == s.PlayerID {
			return v, i, true
		}
	}
	return v, 0, false
}
func (c *Controller) Key(s ui.Snapshot, key string) ui.Result {
	v, _, ok := decode(s)
	if !ok {
		return ui.Result{Handled: true}
	}
	switch key {
	case "tab", "v":
		c.opponent = !c.opponent
		return ui.Result{Handled: true, Status: "Tab / V 切换草坪；查看对方时无法放置植物"}
	case "up", "w":
		c.row = (c.row + game.Rows - 1) % game.Rows
	case "down", "s":
		c.row = (c.row + 1) % game.Rows
	case "left", "a":
		c.col = (c.col + game.Cols - 1) % game.Cols
	case "right", "d":
		c.col = (c.col + 1) % game.Cols
	case "x":
		c.shovel = !c.shovel
	case "esc":
		c.shovel = false
	case "enter", "space", " ":
		return c.action(v)
	default:
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			n := int(key[0] - '0')
			if n == 0 {
				n = 10
			}
			c.selected = game.PlantKind(n)
			c.shovel = false
		} else {
			return ui.Result{}
		}
	}
	return ui.Result{Handled: true}
}
func (c *Controller) action(v game.Snapshot) ui.Result {
	if c.opponent {
		return ui.Result{Handled: true, Status: "正在查看对手，Tab / V 返回自己草坪后操作"}
	}
	if v.Finished {
		return ui.Result{Handled: true, Status: "对局结束 · F2再开一局 · Del回到模式大厅"}
	}
	if !v.Started {
		return ui.Result{Handled: true, Status: "等待开局倒计时"}
	}
	a := game.Action{Type: "plant", Row: c.row, Col: c.col, Plant: c.selected}
	if c.shovel {
		a.Type = "shovel"
	}
	return ui.Result{Handled: true, Action: ui.Action(a)}
}

type layout struct {
	cardColumns, cardRows int
	cellWidth, cellHeight int
	boardTop, minHeight   int
}

const cardWidth, cardStride = 11, 12

// Rendering and pointer hit detection share fixed card rectangles. Changing
// costs/cooldowns cannot shift the clickable positions between live frames.
func geometry(s ui.Snapshot) layout {
	columns := max(1, min(5, s.Width/cardStride))
	rows := (10 + columns - 1) / columns
	width := 6
	if s.Width < 68 {
		width = 4
	}
	return layout{
		cardColumns: columns, cardRows: rows,
		cellWidth: width, cellHeight: max(1, min(3, (s.Height-12-rows)/game.Rows)),
		boardTop: 5 + rows, minHeight: 17 + rows,
	}
}
func (c *Controller) Mouse(s ui.Snapshot, x, y int) ui.Result {
	g := geometry(s)
	if s.Width < 40 || s.Height < g.minHeight {
		return ui.Result{Handled: true, Status: fmt.Sprintf("请扩大终端到至少40列×%d行可用区域", g.minHeight)}
	}
	v, mine, ok := decode(s)
	if !ok {
		return ui.Result{}
	}
	if y >= 2 && y < 2+g.cardRows && x >= 0 && x < g.cardColumns*cardStride && x%cardStride < cardWidth {
		n := (y-2)*g.cardColumns + x/cardStride + 1
		if n > len(game.Plants)-1 {
			return ui.Result{}
		}
		if c.opponent {
			return ui.Result{Handled: true, Status: "对方草坪只读；Tab / V 返回自己草坪后选卡"}
		}
		c.selected, c.shovel = game.PlantKind(n), false
		d := game.Plants[n]
		status := "已选择" + d.Name + " · 点击自己的草坪种植"
		if v.Players[mine].Cooldown[n] > 0 {
			status += fmt.Sprintf(" · 冷却%d秒", (v.Players[mine].Cooldown[n]+9)/10)
		}
		if v.Players[mine].Sun < d.Cost {
			status += " · 阳光不足"
		}
		return ui.Result{Handled: true, Status: status}
	}
	w, h := g.cellWidth, g.cellHeight
	if x < 1 || x >= 1+game.Cols*w || y < g.boardTop || y >= g.boardTop+game.Rows*h {
		return ui.Result{}
	}
	c.col, c.row = (x-1)/w, (y-g.boardTop)/h
	return c.action(v)
}
func (c *Controller) View(s ui.Snapshot) string {
	g := geometry(s)
	if s.Width < 40 || s.Height < g.minHeight {
		return fmt.Sprintf("草坪接力需要至少40列×%d行可用区域。\n请扩大终端窗口；实时对局仍在继续。", g.minHeight)
	}
	v, mine, ok := decode(s)
	if !ok {
		return "正在铺设草坪…"
	}
	index := mine
	if c.opponent {
		index = 1 - mine
	}
	l := v.Players[index]
	w, h := g.cellWidth, g.cellHeight
	boardWidth := game.Cols * w
	clip := func(value string) string { return ansi.Truncate(value, max(10, s.Width), "…") }
	var b strings.Builder
	title := "你的草坪 / " + l.Name
	if c.opponent {
		title = "对手实时草坪 / " + l.Name + " · 只读"
	}
	b.WriteString(clip(gold.Render(title)) + "\n")
	status := fmt.Sprintf("阳光 %d · 击败 %d · 接收 %d · 第%d波 · %ds · %ds后新波", l.Sun, l.Kills, l.Received, v.Wave, v.Elapsed, v.NextWave)
	if !v.Started {
		status = fmt.Sprintf("准备 %d 秒 · 没有小推车 · 脑子被吃即失败", max(int64(1), (v.StartAt-time.Now().UnixMilli()+999)/1000))
	}
	if v.Finished {
		status = "对局结束 · 双方同时失守，平局"
		if !v.Draw {
			for _, p := range v.Players {
				if p.ID == v.Winner {
					status = "胜者：" + p.Name + " · F2再开一局"
				}
			}
		}
	}
	b.WriteString(clip(text.Render(status)) + "\n")
	for group := 0; group < g.cardRows; group++ {
		var cards []string
		for n := 1 + group*g.cardColumns; n <= min(10, (group+1)*g.cardColumns); n++ {
			d := game.Plants[n]
			key := n % 10
			label := fmt.Sprintf("[%d:%s %d]", key, abbreviations[n], d.Cost)
			if l.Cooldown[n] > 0 {
				label = fmt.Sprintf("[%d:%s %ds]", key, abbreviations[n], (l.Cooldown[n]+9)/10)
			}
			label += strings.Repeat(" ", max(0, cardWidth-ansi.StringWidth(label)))
			style := text
			if game.PlantKind(n) == c.selected && !c.shovel {
				style = gold.Background(lipgloss.Color("#3C5835"))
			}
			if l.Cooldown[n] > 0 || l.Sun < d.Cost {
				style = style.Faint(true)
			}
			cards = append(cards, style.Render(label))
		}
		b.WriteString(clip(strings.Join(cards, " ")) + "\n")
	}
	selected := game.Plants[c.selected]
	label := selected.Name + " · " + selected.Hint
	if c.shovel {
		label = "铲子 · 移除选中植物，不返还阳光"
	}
	if c.opponent {
		label = "只读查看 · " + label
	}
	b.WriteString(clip(text.Render(label)) + "\n")
	b.WriteString(text.Render(" "))
	for col := 0; col < game.Cols; col++ {
		b.WriteString(text.Render(fmt.Sprintf("%-*d", w, col+1)))
	}
	b.WriteString("\n")
	b.WriteString(text.Render("┌"+strings.Repeat("─", boardWidth)+"┐") + "\n")
	for row := 0; row < game.Rows; row++ {
		for sub := 0; sub < h; sub++ {
			b.WriteString(text.Render("│"))
			for col := 0; col < game.Cols; col++ {
				p := l.Grid[row][col]
				value := strings.Repeat(" ", w)
				color := "#789E81"
				if p.Kind > 0 && int(p.Kind) < len(sprites) {
					color = plantColors[p.Kind]
					if h == 1 {
						value = abbreviations[p.Kind]
					} else {
						value = sprites[p.Kind][sub]
					}
					if h == 3 && sub == 2 {
						bars := max(1, p.HP*(w-1)/game.Plants[p.Kind].HP)
						value = strings.Repeat("▰", bars)
					}
				}
				var nearest *game.Zombie
				count := 0
				for i := range l.Zombies {
					z := &l.Zombies[i]
					if z.Row == row && min(game.Cols-1, max(0, z.X/game.Cell)) == col {
						count++
						if nearest == nil || z.X < nearest.X {
							nearest = z
						}
					}
				}
				if nearest != nil {
					color = "#DEADBB"
					value = zombieHeads[nearest.Kind]
					if nearest.Slow > 0 {
						color = "#83D8EF"
					}
					if sub == 1 {
						if v.Elapsed%2 == 0 {
							value = " /| "
						} else {
							value = " \\| "
						}
					}
					if h == 3 && sub == 2 {
						value = fmt.Sprintf("%d", nearest.HP+nearest.Armor)
					}
					if count > 1 && sub == 0 {
						value = fmt.Sprintf("Zx%d", count)
					}
				} else if sub == 0 {
					for _, shot := range l.Shots {
						if shot.Row == row && shot.X/game.Cell == col {
							value = "> ·"
							color = "#9BE796"
							break
						}
					}
				}
				for _, fx := range l.Effects {
					if fx.Row == row && fx.Col == col && sub == 0 {
						if fx.Kind == "blast" {
							value = "*!!*"
							color = "#FFC85D"
						} else if p.Kind == 0 {
							value = " * "
							color = "#FF8980"
						}
					}
				}
				bg := grass[(row+col)%2]
				if row == c.row && col == c.col && !c.opponent {
					bg = "#496F36"
				}
				style := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(lipgloss.Color(bg))
				value = ansi.Truncate(value, w, "")
				value += strings.Repeat(" ", max(0, w-ansi.StringWidth(value)))
				b.WriteString(style.Render(value))
			}
			b.WriteString(text.Render("│") + "\n")
		}
	}
	b.WriteString(text.Render("└"+strings.Repeat("─", boardWidth)+"┘") + "\n")
	brain := fmt.Sprintf("脑子：安全 · 所选 第%d行 第%d列", c.row+1, c.col+1)
	if !l.Alive {
		brain = "脑子已被吃掉"
	}
	if len(l.Zombies) > 0 {
		z := l.Zombies[0]
		for _, other := range l.Zombies {
			if other.X < z.X {
				z = other
			}
		}
		brain += fmt.Sprintf(" · 最近：第%d行 %s", z.Row+1, game.Zombies[z.Kind].Name)
	}
	b.WriteString(clip(warning.Render(brain)) + "\n")
	b.WriteString(clip(text.Render("鼠标选卡→点草坪种植 · Tab/V 看对方 · 1–9/0选卡 · X铲子")))
	return b.String()
}
