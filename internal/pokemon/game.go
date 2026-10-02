package pokemon

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type Monster struct {
	Species, Level, HP, Exp int
	Status                  string
	Sleep                   int
	Moves, PP               [4]int
}

func (m Monster) Name() string      { return Dex[m.Species].Name }
func (m Monster) MaxHP() int        { return ((Dex[m.Species].HP+8)*2*m.Level)/100 + m.Level + 10 }
func (m Monster) stat(base int) int { return ((base+8)*2*m.Level)/100 + 5 }
func newMonster(id, level int) Monster {
	m := Monster{Species: id, Level: level, Exp: level * level * level, Moves: moveSet(id, level)}
	m.HP = m.MaxHP()
	for i, n := range m.Moves {
		m.PP[i] = Moves[n].PP
	}
	return m
}

type Battle struct {
	Kind, Name                            string // wild, trainer, gym, event, league
	Foes                                  []Monster
	Enemy, Active, Area, Challenge, Turns int
	Flag                                  string
}
type Capture struct {
	Ball        string
	Probability float64
	Passed      [3]bool
	Step        int
}
type State struct {
	Version                      int
	Name                         string
	Area, Unlocked, Money, Elite int
	Party, Box                   []Monster
	Items                        map[string]int
	TrainerWins                  []int
	Badges                       []string
	Flags                        map[string]bool
	Seen, Caught                 map[int]bool
	Log                          []string
	Battle                       *Battle
	Capture                      *Capture
	Completed                    bool
}
type Game struct {
	State
	rng *rand.Rand
}

func New(name string) *Game {
	g := &Game{State: State{Version: 1, Name: name, Money: 4000, Items: map[string]int{"精灵球": 10, "伤药": 5}, TrainerWins: make([]int, len(Areas)), Flags: map[string]bool{}, Seen: map[int]bool{}, Caught: map[int]bool{}}, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
	g.say("欢迎来到真新镇，%s。大木博士已经准备好三位初始伙伴。", name)
	g.say("输入：选择 妙蛙种子 / 选择 小火龙 / 选择 杰尼龟。")
	g.say("这是重新编写的关都文字冒险；真新镇与沿途章节各有6名独立挑战者。")
	return g
}
func (g *Game) say(format string, args ...any) {
	g.Log = append(g.Log, fmt.Sprintf(format, args...))
	if len(g.Log) > 120 {
		g.Log = append([]string(nil), g.Log[len(g.Log)-120:]...)
	}
}
func (g *Game) Notice(text string) { g.say("%s", text) }
func (g *Game) AreaName() string   { return Areas[g.Area].Name }
func (g *Game) Objective() string  { return Areas[g.Area].Goal }
func (g *Game) Busy() bool         { return g.Capture != nil }
func (g *Game) CaptureStep() int {
	if g.Capture == nil {
		return -1
	}
	return g.Capture.Step
}
func (g *Game) Ready() bool {
	for _, m := range g.Party {
		if m.HP > 0 {
			return true
		}
	}
	return false
}
func (g *Game) activeIndex() int {
	for i, m := range g.Party {
		if m.HP > 0 {
			return i
		}
	}
	return -1
}
func (g *Game) Command(input string) error {
	words := strings.Fields(strings.TrimSpace(input))
	if len(words) == 0 {
		return nil
	}
	if g.Busy() {
		return errors.New("精灵球正在摇动，请等待捕捉结果")
	}
	g.say("› %s", input)
	command := strings.ToLower(words[0])
	arg := ""
	if len(words) > 1 {
		arg = words[1]
	}
	if command == "帮助" || command == "help" {
		g.say("探索：进草丛 / 挑战 / 道馆 / 剧情 / 前进 / 前往 地名 / 地图 / 联盟 / 传说。")
		g.say("战斗：招式 1–4 / 捕捉 精灵球 / 换精灵 1–6 / 使用 伤药 1 / 逃跑。")
		g.say("管理：队伍 / 背包 / 商店 / 购买 精灵球 5 / 治疗 / 图鉴 1 / 电脑 / 存入 2 / 取出 1 / 进化 雷之石 1 / 交换 1。")
		g.say("操作后自动存档；F5手动存档。新冒险 确认会替换本昵称的存档。")
		return nil
	}
	if command == "队伍" || command == "party" {
		g.partyInfo()
		return nil
	}
	if command == "背包" || command == "bag" {
		g.bagInfo()
		return nil
	}
	if command == "图鉴" || command == "dex" {
		page := 1
		if arg != "" {
			page, _ = strconv.Atoi(arg)
		}
		g.dexInfo(page)
		return nil
	}
	if command == "地图" || command == "map" {
		for i, a := range Areas {
			state := "未解锁"
			if i <= g.Unlocked {
				state = "已解锁"
			}
			if i == g.Area {
				state = "当前位置"
			}
			g.say("%d. %s [%s]", i+1, a.Name, state)
		}
		return nil
	}
	if command == "进度" || command == "任务" {
		g.say("目标：%s；本章挑战 %d/6；徽章 %d/8。", g.Objective(), g.TrainerWins[g.Area], len(g.Badges))
		return nil
	}
	if command == "选择" || command == "starter" {
		if len(g.Party) > 0 {
			return errors.New("初始伙伴已经领取")
		}
		id := map[string]int{"妙蛙种子": 1, "小火龙": 4, "杰尼龟": 7, "1": 1, "2": 4, "3": 7}[arg]
		if id == 0 {
			return errors.New("请选择妙蛙种子、小火龙或杰尼龟")
		}
		g.Party = append(g.Party, newMonster(id, 5))
		g.Caught[id] = true
		g.Seen[id] = true
		g.Flags["starter-"+strconv.Itoa(id)] = true
		g.say("你与%s成为伙伴，得到图鉴和初始物品。镇上的6名训练者已等候挑战。", Dex[id].Name)
		return nil
	}
	if len(g.Party) == 0 {
		return errors.New("请先用“选择 妙蛙种子 / 小火龙 / 杰尼龟”领取伙伴")
	}
	if g.Battle != nil {
		return g.battleCommand(command, words)
	}
	switch command {
	case "进草丛", "草丛", "探索", "grass":
		if !g.Ready() {
			return errors.New("队伍全部失去战斗能力，请先治疗")
		}
		a := Areas[g.Area]
		id := a.Wild[g.rng.Intn(len(a.Wild))]
		if g.Completed && g.Area == 11 && g.rng.Intn(4) == 0 {
			id = 1 + g.rng.Intn(151)
		}
		level := a.MinLevel + g.rng.Intn(a.MaxLevel-a.MinLevel+1)
		g.startBattle("wild", "野生遭遇", []Monster{newMonster(id, level)}, "", 0)
		g.say("草叶晃动！野生%s Lv%d出现。", Dex[id].Name, level)
	case "挑战", "challenge":
		if g.Area >= 11 {
			return errors.New("冠军后篇没有强制挑战者，可以探索草丛和传说遭遇")
		}
		n := g.TrainerWins[g.Area]
		if n >= 6 {
			return errors.New("本章6名挑战者均已击败，使用剧情、道馆或前进")
		}
		a := Areas[g.Area]
		level := a.MinLevel + 2 + n
		team := []Monster{newMonster(a.Trainers[n], level)}
		if g.Area > 1 {
			team = append(team, newMonster(a.Trainers[(n+1)%6], level))
		}
		name := []string{"路边新手", "捕虫少年", "远行学生", "营地训练者", "山路旅人", "城镇守门人"}[n]
		if g.Area == 0 {
			name = []string{"邻居小岚", "牧场学徒", "练习生小禾", "研究助手", "海边少年", "劲敌见习战"}[n]
		}
		if err := g.startBattle("trainer", name, team, "", n+1); err != nil {
			return err
		}
		g.say("第%d/6名挑战者%s向你发起对战。胜利才计入进度。", n+1, name)
	case "道馆", "gym":
		return g.gym()
	case "剧情", "story":
		return g.story()
	case "联盟", "league":
		return g.league()
	case "前进", "next":
		return g.advance()
	case "前往", "go":
		if arg == "" {
			return errors.New("例如：前往 紫苑镇；用地图查看已解锁地点")
		}
		for i, a := range Areas {
			if strings.Contains(a.Name, arg) || arg == strconv.Itoa(i+1) {
				if i > g.Unlocked {
					return errors.New("该地点尚未解锁")
				}
				g.Area = i
				g.say("来到%s。%s", a.Name, a.Intro)
				return nil
			}
		}
		return errors.New("没有找到该地点")
	case "治疗", "heal":
		g.heal()
		if g.Elite > 0 && !g.Completed {
			g.Elite = 0
			g.say("离开联盟接受治疗，本次连续挑战从四天王第一位重新开始。")
		}
		g.say("伙伴的HP、异常状态与PP全部恢复。治疗免费。")
	case "商店", "shop":
		g.shopInfo()
	case "购买", "买", "buy":
		return g.buy(words)
	case "使用", "use":
		return g.use(words, false)
	case "电脑", "box":
		for i, m := range g.Box {
			g.say("电脑%d：%s Lv%d HP %d/%d", i+1, m.Name(), m.Level, m.HP, m.MaxHP())
		}
		if len(g.Box) == 0 {
			g.say("电脑中暂无精灵；携带队伍上限6只，额外捕获会自动存入。")
		}
	case "存入", "deposit", "取出", "withdraw":
		return g.boxCommand(command, arg)
	case "进化", "evolve", "交换", "trade":
		return g.evolveCommand(command, words)
	case "传说", "legend":
		return g.legend(arg)
	default:
		return errors.New("未知指令。输入“帮助”查看命令；命令与参数用空格分隔")
	}
	return nil
}
func (g *Game) startBattle(kind, name string, foes []Monster, flag string, challenge int) error {
	active := g.activeIndex()
	if active < 0 {
		return errors.New("队伍全部失去战斗能力，请先治疗")
	}
	g.Battle = &Battle{Kind: kind, Name: name, Foes: foes, Active: active, Area: g.Area, Flag: flag, Challenge: challenge}
	for _, foe := range foes {
		g.Seen[foe.Species] = true
	}
	return nil
}
func (g *Game) heal() {
	for i := range g.Party {
		m := &g.Party[i]
		m.HP = m.MaxHP()
		m.Status = ""
		m.Sleep = 0
		for j, id := range m.Moves {
			m.PP[j] = Moves[id].PP
		}
	}
}
func (g *Game) partyInfo() {
	for i, m := range g.Party {
		g.say("%d. %s Lv%d HP %d/%d [%s]", i+1, m.Name(), m.Level, m.HP, m.MaxHP(), statusName(m.Status))
		for j, id := range m.Moves {
			move := Moves[id]
			g.say("  招式%d %s [%s] PP %d/%d", j+1, move.Name, TypeNames[move.Type], m.PP[j], move.PP)
		}
	}
}
func statusName(status string) string {
	if status == "" {
		return "正常"
	}
	return status
}
func (g *Game) bagInfo() {
	g.say("零钱：%d；徽章：%s；电脑精灵：%d。", g.Money, strings.Join(g.Badges, "、"), len(g.Box))
	for _, name := range itemOrder {
		if g.Items[name] > 0 {
			g.say("%s ×%d", name, g.Items[name])
		}
	}
}
func (g *Game) dexInfo(page int) {
	page = max(1, min(8, page))
	g.say("图鉴：见过%d / 捕获%d / 151；第%d页。", len(g.Seen), len(g.Caught), page)
	for id := (page-1)*20 + 1; id <= min(page*20, 151); id++ {
		state := "未遇见"
		if g.Seen[id] {
			state = "见过"
		}
		if g.Caught[id] {
			state = "已拥有"
		}
		g.say("#%03d %s [%s] 捕获率参数%d", id, Dex[id].Name, state, Dex[id].CatchRate)
	}
}
