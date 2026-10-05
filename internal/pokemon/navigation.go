package pokemon

import (
	"fmt"
	"strconv"
	"strings"
)

var itemAliases = []string{"pokeball", "greatball", "ultraball", "masterball", "potion", "superpotion", "hyperpotion", "revive", "antidote", "awakening", "paralyzeheal", "fullheal", "firestone", "waterstone", "thunderstone", "leafstone", "moonstone"}

func normalizeASCII(words []string) []string {
	words[0] = strings.ToLower(words[0])
	if len(words) > 1 {
		switch words[0] {
		case "buy", "use", "evolve", "catch":
			arg := strings.ToLower(words[1])
			for i, alias := range itemAliases {
				if arg == alias || arg == strconv.Itoa(i+1) {
					words[1] = itemOrder[i]
					break
				}
			}
		}
	}
	return words
}

type Place struct{ Name, Detail, Panel string }

var sceneNames = [12][4]string{
	{"海风草坡", "白栅栏训练场", "博士的后院", "伙伴实验室"},
	{"虫鸣林地", "森林营地", "博物馆侧门", "岩石道馆"},
	{"月见山洞口", "华蓝水道", "化石与海角小屋", "水之道馆"},
	{"港湾草地", "甲板练习场", "圣特安努号", "电之道馆"},
	{"隧道出口", "钟楼广场", "宝可梦塔", "静音纪念馆"},
	{"花园小径", "百货天台", "游戏厅地下通道", "花园道馆"},
	{"狩猎草海", "林边营地", "园长失物屋", "毒之道馆"},
	{"郊外草地", "磁轨广场", "西尔佛公司", "超能力道馆"},
	{"火山草坡", "熔岩观测站", "研究宅邸", "火之道馆"},
	{"常磐旧路", "归乡练习场", "关闭的旧办公室", "大地道馆"},
	{"冠军山道", "联盟训练营", "徽章检验厅", "联盟殿堂"},
	{"洞窟深处", "冠军交流营", "星光观测台", "传说调查站"},
}

func RegionPlaces(area int) []Place {
	n := sceneNames[area]
	return []Place{
		{"城市中心", "地图墙、商店、治疗站与旅人留言板汇集于此。", "center"},
		{n[0], "草丛中的野生伙伴正在活动，做好捕捉准备。", "grass-site"},
		{n[1], "本章有六名独立挑战者，胜利后才计入进度。", "trainer-site"},
		{n[2], "主线事件与支线线索所在的调查现场。", "story-site"},
		{n[3], "道馆或联盟考验；所需门槛会明确提示。", "gym-site"},
		{"城市中心商店", "查看全部货品与解锁条件；地图墙可以指引下一程。", "shop"},
		{"伙伴治疗站", "免费恢复；未通关的联盟连战会重新计数。", "clinic"},
		{"旅人留言板", "当地居民的原创插曲；调查现场后可解决委托。", "quest"},
	}
}

var regionTerrain = [12][2]string{
	{"... SEA BREEZE ...", "~~~~~ COAST ~~~~~"}, {"^^^ VIRIDIAN WOODS ^^^", "^^^ OLD STONES ^^^"},
	{"^^^^ MOON CAVES ^^^^", "~~~~ CERULEAN CANALS ~~~~"}, {"~~~~ VERMILION PORT ~~~~", "==== SHIP DECK ===="},
	{".... LAVENDER BELLS ....", "^^^^ SILENT TOWER ^^^^"}, {"*** CELADON GARDENS ***", "==== UNDERGROUND ===="},
	{"^^^ FUCHSIA SAFARI ^^^", "~~~~ REED MARSH ~~~~"}, {"==== SAFFRON SKYLINE ====", "|||| SILPH FLOORS ||||"},
	{"^^^^ CINNABAR VOLCANO ^^^^", "~~~~ SOUTHERN SEA ~~~~"}, {"^^^ VIRIDIAN HOMECOMING ^^^", "==== RETURN ROAD ===="},
	{"^^^^ VICTORY RIDGE ^^^^", "==== INDIGO PLATEAU ===="}, {".... CHAMPION STARLIGHT ....", "^^^^ CERULEAN CAVE ^^^^"},
}

// Numeric buildings keep tile widths constant in any Chinese terminal font.
func (g *Game) RegionMap() string {
	rows := []string{regionTerrain[g.Area][0], " [2]====[3]====[5]", "  ||     ||     ||", " [4]====[1]====[8]", "  ||     ||     ||", " [7]====[6]====[X]", regionTerrain[g.Area][1]}
	for i := range rows {
		rows[i] = strings.ReplaceAll(rows[i], fmt.Sprintf("[%d]", g.Location+1), fmt.Sprintf("<%d>", g.Location+1))
	}
	return strings.Join(rows, "\n")
}

func (g *Game) WorldMap() string {
	chart := "[01]--[02]--[03]--[04]\n                   |\n[08]--[07]--[06]--[05]\n |\n[09]--[10]--[11]--[12]"
	return strings.ReplaceAll(chart, fmt.Sprintf("[%02d]", g.Area+1), fmt.Sprintf("<%02d>", g.Area+1))
}

type Choice struct {
	Label, Command, Panel, Detail string
	Target                        int
	Enabled                       bool
}

func option(label, command, panel string, target int) Choice {
	return Choice{Label: label, Command: command, Panel: panel, Target: target, Enabled: true}
}

// Choices enumerate actions; the renderer paginates nine at a time. Commands
// are all ASCII, including item IDs and destination IDs.
func (g *Game) Choices(panel string, target int) []Choice {
	if len(g.Party) == 0 && panel != "reset" {
		return []Choice{option("妙蛙种子 · 草 / 毒", "starter 1", "root", 0), option("小火龙 · 火", "starter 2", "root", 0), option("杰尼龟 · 水", "starter 3", "root", 0)}
	}
	var out []Choice
	add := func(label, command, next string, value int) { out = append(out, option(label, command, next, value)) }
	switch panel {
	case "battle":
		if g.Battle == nil {
			return g.Choices("root", 0)
		}
		p := g.Party[g.Battle.Active]
		for i, id := range p.Moves {
			add(fmt.Sprintf("%s · PP %d/%d", Moves[id].Name, p.PP[i], Moves[id].PP), fmt.Sprintf("move %d", i+1), "battle", 0)
		}
		add("投掷精灵球", "", "balls", 0)
		out[len(out)-1].Enabled = g.Battle.Kind == "wild"
		add("使用背包药品", "", "bag", 0)
		add("换上队伍伙伴", "", "switch", 0)
		add("逃离野生遭遇", "run", "root", 0)
		out[len(out)-1].Enabled = g.Battle.Kind == "wild"
		add("查看伙伴信息", "", "party", 0)
		add("查看对手队伍数量", "", "opponent", 0)
	case "root":
		add("地区地图 / 探索建筑", "", "map", 0)
		add("返回城市中心", "visit 1", "center", 0)
		add("进入草丛", "grass", "battle", 0)
		add("挑战下一位训练者", "challenge", "battle", 0)
		add("推进主线剧情", "story", "root", 0)
		add("挑战道馆 / 联盟", g.arenaCommand(), "root", 0)
		add("队伍与招式", "", "party", 0)
		add("背包物品", "", "bag", 0)
		add("旅行指南 / 更多功能", "", "guide", 0)
	case "center":
		add("地图墙 · 选择地区建筑", "", "map", 0)
		add("商店 · 全部商品", "visit 6", "shop", 0)
		add("治疗伙伴", "heal", "center", 0)
		add("队伍与进化", "", "party", 0)
		add("电脑 · 存入 / 取出", "", "pc", 0)
		add("当地支线委托", "visit 8", "quest", 0)
		add("伙伴专属技教学", "", "training", 0)
		add("世界地图 / 前往城市", "", "world", 0)
		add("旅行指南", "", "guide", 0)
	case "map":
		for i, p := range RegionPlaces(g.Area) {
			add(p.Name, fmt.Sprintf("visit %d", i+1), p.Panel, 0)
		}
	case "world":
		for i, a := range Areas {
			suffix := "已解锁"
			if i > g.Unlocked {
				suffix = "未解锁"
			}
			if i == g.Area {
				suffix = "当前位置"
			}
			add(a.Name+" · "+suffix, fmt.Sprintf("go %d", i+1), "center", 0)
			out[len(out)-1].Enabled = i <= g.Unlocked
		}
	case "guide":
		add("支线任务日志", "", "quest", 0)
		add("世界地图", "", "world", 0)
		add("前进下一章节", "next", "center", 0)
		add("151种图鉴", "", "dex", 0)
		add("传说调查", "", "legends", 0)
		add("查看完整 ASCII 指令", "help", "guide", 0)
		add("主线门槛与进度", "progress", "guide", 0)
		add("专属技教学", "", "training", 0)
		add("重新开始（需再次确认）", "", "reset", 0)
	case "grass-site":
		add("进入草丛", "grass", "battle", 0)
		add("返回地图", "", "map", 0)
	case "trainer-site":
		add("挑战下一位训练者", "challenge", "battle", 0)
		add("查看本章进度", "progress", "trainer-site", 0)
	case "story-site":
		add("处理主线事件", "story", "story-site", 0)
		add("查看当地支线", "", "quest", 0)
		add("向下一章节前进", "next", "center", 0)
	case "gym-site":
		add("道馆 / 联盟对战", g.arenaCommand(), "gym-site", 0)
		add("主线事件", "story", "gym-site", 0)
		add("继续旅程", "next", "center", 0)
	case "clinic":
		add("免费治疗全队", "heal", "clinic", 0)
		add("整理电脑", "", "pc", 0)
	case "shop":
		add("商店地图墙", "", "map", 0)
		for i, item := range itemOrder {
			label := fmt.Sprintf("%s · $%d", item, prices[item])
			if item == "大师球" {
				label = "大师球 · 西尔佛剧情奖励（不出售）"
			} else if !g.available(item) {
				if item == "超级球" {
					label += " [1徽章]"
				} else if item == "高级球" {
					label += " [3徽章]"
				} else {
					label += " [解锁华蓝]"
				}
			}
			add(label, "", "quantity", i+1)
			out[len(out)-1].Enabled = g.available(item)
		}
	case "quantity":
		for _, n := range []int{1, 5, 10, 20, 99} {
			add(fmt.Sprintf("购买 %d 个", n), fmt.Sprintf("buy %d %d", target, n), "shop", 0)
		}
	case "bag", "balls":
		for i, item := range itemOrder {
			if g.Items[item] == 0 || (panel == "balls" && i >= 4) {
				continue
			}
			next := "use-target"
			command := ""
			if i < 4 {
				next = "balls"
				command = fmt.Sprintf("catch %d", i+1)
			}
			if i >= 12 {
				next = "evolve-target"
			}
			add(fmt.Sprintf("%s ×%d", item, g.Items[item]), command, next, i+1)
			if (i < 4 && (g.Battle == nil || g.Battle.Kind != "wild")) || (i >= 12 && g.Battle != nil) {
				out[len(out)-1].Enabled = false
			}
		}
	case "party", "switch", "deposit", "training", "use-target", "evolve-target":
		for i, p := range g.Party {
			command := ""
			next := "detail"
			value := i
			switch panel {
			case "switch":
				command = fmt.Sprintf("switch %d", i+1)
				next = "battle"
			case "deposit":
				command = fmt.Sprintf("deposit %d", i+1)
				next = "pc"
			case "training":
				command = fmt.Sprintf("train %d", i+1)
				next = "training"
			case "use-target":
				command = fmt.Sprintf("use %d %d", target, i+1)
				next = "bag"
			case "evolve-target":
				command = fmt.Sprintf("evolve %d %d", target, i+1)
				next = "party"
			}
			add(fmt.Sprintf("%s Lv%d · HP %d/%d", p.Name(), p.Level, p.HP, p.MaxHP()), command, next, value)
		}
	case "opponent":
		add("返回对战行动", "", "battle", 0)
	case "detail":
		if target < 0 || target >= len(g.Party) {
			return nil
		}
		if g.Battle != nil {
			add("换上这位伙伴", fmt.Sprintf("switch %d", target+1), "battle", 0)
			break
		}
		add("学习专属技", fmt.Sprintf("train %d", target+1), "detail", target)
		add("存入电脑", fmt.Sprintf("deposit %d", target+1), "party", 0)
		add("本地通信进化", fmt.Sprintf("trade %d", target+1), "detail", target)
	case "pc":
		add("存入队伍伙伴", "", "deposit", 0)
		add("取出电脑伙伴", "", "withdraw", 0)
	case "withdraw":
		for i, p := range g.Box {
			add(fmt.Sprintf("%s Lv%d", p.Name(), p.Level), fmt.Sprintf("withdraw %d", i+1), "pc", 0)
		}
	case "dex":
		for id := 1; id <= 151; id++ {
			state := "未遇见"
			if g.Seen[id] {
				state = "已遇见"
			}
			if g.Caught[id] {
				state = "已捕获"
			}
			add(fmt.Sprintf("#%03d %s · %s", id, Dex[id].Name, state), "", "dex-detail", id)
		}
	case "dex-detail":
		add("返回图鉴", "", "dex", 0)
	case "legends":
		for _, id := range []int{144, 145, 146, 150} {
			add(Dex[id].Name, fmt.Sprintf("legend %d", id), "battle", 0)
			out[len(out)-1].Enabled = g.Completed && !g.Caught[id]
		}
	case "quest":
		switch g.QuestStage(g.Area) {
		case 0:
			add("接受委托 · 先听对方说完", "quest 1", "quest", 0)
			add("接受委托 · 先检查线索", "quest 2", "quest", 0)
		case 1:
			add("前往调查现场寻找线索", "visit 4", "quest", 0)
		case 2:
			add("解决委托对战", "quest 3", "battle", 0)
		case 3:
			add("查看旅途收集与回声", "quest", "quest", 0)
		}
		add("返回城市中心", "visit 1", "center", 0)
	case "reset":
		add("确认重置本昵称存档", "new yes", "root", 0)
		add("取消并返回", "", "root", 0)
	}
	return out
}
func (g *Game) arenaCommand() string {
	if g.Area == 10 {
		return "league"
	}
	if g.Area == 11 {
		return "legend 150"
	}
	return "gym"
}

var panelNames = map[string]string{"root": "冒险行动", "battle": "对战行动", "opponent": "对手队伍", "center": "城市中心", "map": "地区地图 · 建筑编号", "world": "关都世界地图", "guide": "旅行指南", "shop": "城市中心商店", "quantity": "购买数量", "bag": "背包", "balls": "选择精灵球", "party": "队伍", "switch": "替换伙伴", "detail": "伙伴详情", "pc": "电脑", "deposit": "存入电脑", "withdraw": "取出伙伴", "dex": "151种图鉴", "dex-detail": "物种资料", "training": "专属技教学", "use-target": "药品使用对象", "evolve-target": "进化石使用对象", "legends": "冠军后传说调查", "quest": "当地居民的委托", "reset": "新冒险确认"}

func (g *Game) PanelTitle(panel string, target int) string {
	if len(g.Party) == 0 && panel != "reset" {
		return "大木博士实验室 · 选择初始伙伴"
	}

	if name, ok := panelNames[panel]; ok {
		return name
	}
	return RegionPlaces(g.Area)[g.Location].Name
}

func (g *Game) PanelInfo(panel string, target int) string {
	switch panel {
	case "quest":
		return g.QuestInfo()
	case "training":
		return "原创伙伴专属技：Lv20以上，并持有2枚徽章或完成3个支线。每位伙伴首次教学$500，替换第四个招式；进化后保留学习资格。"
	case "detail":
		if target < 0 || target >= len(g.Party) {
			return "伙伴已离开队伍"
		}
		p := g.Party[target]
		var rows []string
		rows = append(rows, fmt.Sprintf("%s Lv%d · %s\nHP %d/%d", p.Name(), p.Level, statusName(p.Status), p.HP, p.MaxHP()))
		rows = append(rows, p.ExperienceSummary(), p.EvolutionSummary())
		for i, id := range p.Moves {
			rows = append(rows, fmt.Sprintf("%d %s [%s] PP %d/%d", i+1, Moves[id].Name, TypeNames[Moves[id].Type], p.PP[i], Moves[id].PP))
		}
		return strings.Join(rows, "\n")
	case "opponent":
		return g.OpponentInfo()
	case "dex-detail":
		if target < 1 || target > 151 {
			return "物种编号1–151"
		}
		s := Dex[target]
		var types []string
		for _, t := range s.Types {
			types = append(types, TypeNames[t])
		}
		return fmt.Sprintf("#%03d %s / %s\n属性 %s · 捕获率 %d\n基础 HP %d / 攻击 %d / 防御 %d\n特殊 %d / 速度 %d", target, s.Name, s.English, strings.Join(types, " / "), s.CatchRate, s.HP, s.Attack, s.Defense, s.Special, s.Speed)
	case "shop":
		return "全部货品均列出；灰色商品尚未解锁。数字选择物品后，再选择数量。M 地图墙 / C 城市中心。"
	case "quantity":
		if target >= 1 && target <= len(itemOrder) {
			item := itemOrder[target-1]
			return fmt.Sprintf("%s · 单价$%d · 持有%d · 零钱$%d", item, prices[item], g.Items[item], g.Money)
		}
	case "guide":
		return "主线目标：" + g.Objective() + "\n本章训练者六场胜利、主线事件和徽章分别计数。M 地图 / C 城市中心 / Q 当地委托。\n下一页、上一页用 ] / [；药品和进化石按菜单编号选择。\nF5 自动存档重试 / F9 音乐 / Del 保存离开。"
	case "world":
		return "选择已解锁城市；主线推进仍需本章六次挑战、剧情及徽章。紫苑与玉虹之间可往返补完检视镜和笛子事件。"
	case "reset":
		return "这会替换当前昵称的存档。选择1确认；选择2取消。旧存档损坏时请先自行备份。"
	}
	return RegionPlaces(g.Area)[g.Location].Detail
}
