package pokemon

import (
	"errors"
	"fmt"
	"strconv"
)

func gymFlag(area int) string { return "gym-" + strconv.Itoa(area) }
func (g *Game) six() error {
	if g.TrainerWins[g.Area] < 6 {
		return fmt.Errorf("本章挑战进度%d/6，请先完成六名挑战者的对战", g.TrainerWins[g.Area])
	}
	return nil
}
func (g *Game) gym() error {
	a := Areas[g.Area]
	if a.Boss == "" {
		return errors.New("此章节没有道馆，请使用 story / league")
	}
	if err := g.six(); err != nil {
		return err
	}
	if g.Flags[gymFlag(g.Area)] {
		return errors.New("这枚徽章已经获得，输入 next 继续旅程")
	}
	required := map[int]string{2: "ticket", 3: "cut", 6: "surf", 7: "silph", 8: "key"}
	if flag := required[g.Area]; flag != "" && !g.Flags[flag] {
		return errors.New("请先输入 story 完成本城的事件")
	}
	team := make([]Monster, 0, len(a.BossTeam))
	for _, id := range a.BossTeam {
		team = append(team, newMonster(id, a.BossLevel))
	}
	if err := g.startBattle("gym", a.Boss, team, gymFlag(g.Area), 0); err != nil {
		return err
	}
	g.say("道馆馆主%s接受挑战！", a.Boss)
	return nil
}
func (g *Game) story() error {
	g.chapterDialogue()
	if g.Area == 11 {
		g.say("博士：冠军不是终点。记录野生伙伴的生活，试着完成151种图鉴吧。legend 144 / 145 / 146 / 150 选择急冻鸟、闪电鸟、火焰鸟、超梦。")
		return nil
	}
	if err := g.six(); err != nil {
		return err
	}
	switch g.Area {
	case 0:
		if !g.Flags["town"] {
			g.Flags["town"] = true
			g.say("博士查看了六场训练记录：你已学会与伙伴互相信赖。这份地图会陪你走过关都的八座道馆。向北前进吧。")
		} else {
			g.say("博士：真新镇的训练已经完成，输入 next 出发。")
		}
	case 1:
		g.say("小刚的岩石道馆已经开放，输入 gym 挑战。")
	case 2:
		if !g.Flags["ticket"] {
			g.Flags["ticket"] = true
			g.Items["月之石"] = min(999, g.Items["月之石"]+1)
			g.obtain(newMonster(138, 12))
			g.say("你保护了月见山的化石，研究员复原出菊石兽并交给你。在海角帮正辉修好传送装置，他赠送圣特安努号船票与月之石。华蓝道馆现已开放。")
		} else {
			g.say("已取得船票，输入 gym 挑战小霞。")
		}
	case 3:
		if !g.Flags["ticket"] {
			return errors.New("尚未取得船票，请回华蓝市处理剧情")
		}
		g.Flags["cut"] = true
		g.say("圣特安努号上，你帮助船长安顿伙伴，获得居合斩通行许可。你剪开道馆外的枝条，马志士等待挑战。")
	case 4:
		if !g.Flags["scope"] {
			g.Flags["tower"] = true
			g.say("塔中的幽灵阻断阶梯，你无法辨认它的身份。居民提到玉虹市游戏厅有检视镜线索。输入 next 前往玉虹，取得检视镜后用 go 5 返回紫苑镇。")
		} else if !g.Flags["flute"] {
			return g.event("火箭队·宝可梦塔", []int{109, 42}, 28, "flute")
		} else {
			g.say("富士老人已获救，宝可梦之笛可以唤醒挡路的卡比兽。")
		}
	case 5:
		if !g.Flags["scope"] {
			return g.event("坂木·地下基地", []int{95, 111, 115}, 28, "scope")
		}
		g.say("你已取得检视镜；回紫苑镇输入 story 救出富士老人。莉佳的道馆也等待挑战。")
	case 6:
		g.Flags["surf"] = true
		g.Flags["strength"] = true
		g.say("你在狩猎地带找到金牙，并将它交还园长。园长授予冲浪与怪力的旅行许可。水路已经开放，下一步挑战阿桔。")
	case 7:
		if !g.Flags["silph"] {
			return g.event("坂木·西尔佛公司", []int{111, 31, 34, 115}, 38, "silph")
		}
		g.say("西尔佛公司恢复秩序，娜姿的道馆已经开放。")
	case 8:
		g.Flags["key"] = true
		g.say("宅邸的研究手记记述着一种强大的人工精灵。你在旧实验室找到道馆钥匙，推开夏伯道馆的大门。")
	case 9:
		g.say("常磐道馆终于开门。坂木决定用最后一场正式对战接受你的挑战。输入 gym。")
	case 10:
		g.say("四天王等候在联盟大厅。持有八枚徽章即可输入 league 连续挑战五场；使用治疗会从头开始，背包药品不会重置进度。")
	}
	return nil
}
func (g *Game) event(name string, ids []int, level int, flag string) error {
	team := make([]Monster, 0, len(ids))
	for _, id := range ids {
		team = append(team, newMonster(id, level))
	}
	if err := g.startBattle("event", name, team, flag, 0); err != nil {
		return err
	}
	g.say("剧情对战开始：%s。", name)
	return nil
}
func (g *Game) eventReward(flag string) {
	if g.sideReward(flag) {
		return
	}
	switch flag {
	case "scope":
		g.say("地下基地恢复安静，你找到西尔佛检视镜。回紫苑镇输入 story 救出富士老人。")
	case "flute":
		g.say("富士老人走出宝可梦塔：愿每段旅途都有人与伙伴相伴。他赠送宝可梦之笛，浅红市的道路可以开放了。")
	case "silph":
		g.Items["大师球"] = min(999, g.Items["大师球"]+1)
		if len(g.Box) < 600 || len(g.Party) < 6 {
			g.obtain(newMonster(131, 25))
		}
		g.say("西尔佛公司获救。总裁赠送唯一的大师球，研究员托付拉普拉斯；娜姿道馆现在开放。")
	}
}
func (g *Game) advance() error {
	if g.Area == 11 {
		return errors.New("已抵达冠军后篇，可探索、捕捉和补全图鉴")
	}
	if g.Area == 10 {
		if !g.Completed {
			return errors.New("请先完成四天王与冠军连续挑战")
		}
	} else {
		if err := g.six(); err != nil {
			return err
		}
		if Areas[g.Area].Boss != "" && !g.Flags[gymFlag(g.Area)] {
			return errors.New("请先挑战道馆取得本城徽章")
		}
		required := map[int][]string{0: {"town"}, 2: {"ticket"}, 3: {"cut"}, 4: {"tower"}, 5: {"scope", "flute"}, 6: {"surf"}, 7: {"silph"}, 8: {"key"}}
		for _, flag := range required[g.Area] {
			if !g.Flags[flag] {
				if flag == "flute" {
					return errors.New("请先go 5 返回紫苑镇，输入 story 救出富士老人取得宝可梦之笛")
				}
				return errors.New("请先输入 story 完成本章节事件")
			}
		}
	}
	g.Area++
	g.Location = 0
	g.Unlocked = max(g.Unlocked, g.Area)
	g.say("来到%s。%s", g.AreaName(), Areas[g.Area].Intro)
	return nil
}
func (g *Game) league() error {
	if g.Area != 10 {
		return errors.New("请先到冠军之路·石英高原")
	}
	if g.Completed {
		return errors.New("联盟已通关，输入 next 开启冠军后篇")
	}
	if err := g.six(); err != nil {
		return err
	}
	if len(g.Badges) != 8 {
		return errors.New("联盟需要八枚徽章")
	}
	names := []string{"科拿", "希巴", "菊子", "阿渡", "劲敌·现任冠军"}
	teams := [][]int{{87, 91, 80, 124, 131}, {95, 107, 106, 95, 68}, {94, 42, 93, 24, 94}, {130, 148, 148, 142, 149}, {18, 65, 112, 130, 103, 6}}
	if g.Flags["starter-4"] {
		teams[4][5] = 9
	}
	if g.Flags["starter-7"] {
		teams[4][5] = 3
	}
	level := 54 + g.Elite*2
	team := make([]Monster, 0, 6)
	for _, id := range teams[g.Elite] {
		team = append(team, newMonster(id, level))
	}
	if err := g.startBattle("league", names[g.Elite], team, "", 0); err != nil {
		return err
	}
	g.say("联盟第%d/5场：%s。", g.Elite+1, names[g.Elite])
	return nil
}
func (g *Game) legend(arg string) error {
	if !g.Completed {
		return errors.New("成为冠军后才能调查传说精灵")
	}
	id := map[string]int{"急冻鸟": 144, "闪电鸟": 145, "火焰鸟": 146, "超梦": 150, "144": 144, "145": 145, "146": 146, "150": 150}[arg]
	if id == 0 {
		return errors.New("legend 144急冻鸟 / 145闪电鸟 / 146火焰鸟 / 150超梦")
	}
	if g.Caught[id] {
		return errors.New("这只传说精灵已被记录捕获，可继续探索冠军后篇草丛")
	}
	level := 50
	if id == 150 {
		level = 70
	}
	if err := g.startBattle("wild", "传说遭遇", []Monster{newMonster(id, level)}, "", 0); err != nil {
		return err
	}
	g.say("远处的回声停止了……%s Lv%d出现！", Dex[id].Name, level)
	return nil
}
