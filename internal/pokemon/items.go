package pokemon

import (
	"errors"
	"strconv"
)

var itemOrder = []string{"精灵球", "超级球", "高级球", "大师球", "伤药", "好伤药", "厉害伤药", "活力碎片", "解毒药", "解眠药", "解麻药", "万灵药", "火之石", "水之石", "雷之石", "叶之石", "月之石"}
var prices = map[string]int{"精灵球": 200, "超级球": 600, "高级球": 1200, "伤药": 300, "好伤药": 700, "厉害伤药": 1200, "活力碎片": 1500, "解毒药": 100, "解眠药": 250, "解麻药": 200, "万灵药": 600, "火之石": 2100, "水之石": 2100, "雷之石": 2100, "叶之石": 2100, "月之石": 2100}

func (g *Game) available(item string) bool {
	if prices[item] == 0 {
		return false
	}
	if item == "超级球" {
		return len(g.Badges) >= 1
	}
	if item == "高级球" {
		return len(g.Badges) >= 3
	}
	if prices[item] == 2100 {
		return g.Unlocked >= 2
	}
	return true
}
func (g *Game) shopInfo() {
	g.say("商店 · 零钱%d；buy 物品编号 数量。进度提升后解锁高级球种和进化石。", g.Money)
	for _, item := range itemOrder {
		if g.available(item) {
			g.say("%s：%d / 个", item, prices[item])
		}
	}
}
func (g *Game) buy(words []string) error {
	if len(words) < 2 {
		return errors.New("例如：buy pokeball 5（或 buy 1 5）")
	}
	item := words[1]
	if !g.available(item) {
		return errors.New("商店尚未出售该物品，输入 shop查看清单")
	}
	qty := 1
	if len(words) > 2 {
		qty, _ = strconv.Atoi(words[2])
	}
	if qty < 1 || qty > 99 {
		return errors.New("每次购买数量为1–99")
	}
	if g.Items[item]+qty > 999 {
		return errors.New("该物品最多保管999个")
	}
	if qty > g.Money/prices[item] {
		return errors.New("零钱不足")
	}
	g.Items[item] += qty
	g.Money -= qty * prices[item]
	g.say("买到%s ×%d，剩余零钱%d。", item, qty, g.Money)
	return nil
}
func (g *Game) use(words []string, inBattle bool) error {
	if len(words) < 2 {
		return errors.New("例如：use potion 1（或 use 5 1）")
	}
	item := words[1]
	if g.Items[item] <= 0 {
		return errors.New("背包中没有该物品")
	}
	index := 0
	if inBattle {
		index = g.Battle.Active
	}
	if len(words) > 2 {
		n, err := strconv.Atoi(words[2])
		if err != nil {
			return errors.New("队伍编号应为数字")
		}
		index = n - 1
	}
	if index < 0 || index >= len(g.Party) {
		return errors.New("队伍编号不存在")
	}
	m := &g.Party[index]
	effective := false
	switch item {
	case "伤药", "好伤药", "厉害伤药":
		amount := map[string]int{"伤药": 20, "好伤药": 50, "厉害伤药": 200}[item]
		if m.HP > 0 && m.HP < m.MaxHP() {
			m.HP = min(m.MaxHP(), m.HP+amount)
			effective = true
		}
	case "活力碎片":
		if m.HP == 0 {
			m.HP = max(1, m.MaxHP()/2)
			effective = true
		}
	case "解毒药", "解眠药", "解麻药", "万灵药":
		target := map[string]string{"解毒药": "中毒", "解眠药": "睡眠", "解麻药": "麻痹"}[item]
		if m.Status != "" && (item == "万灵药" || m.Status == target) {
			m.Status = ""
			m.Sleep = 0
			effective = true
		}
	default:
		return errors.New("该物品不能这样使用；精灵球用 catch，进化石用 evolve")
	}
	if !effective {
		return errors.New("该物品对所选精灵无效，未消耗")
	}
	g.Items[item]--
	g.say("对%s使用%s，HP %d/%d [%s]。", m.Name(), item, m.HP, m.MaxHP(), statusName(m.Status))
	if inBattle {
		g.enemyTurn()
	}
	return nil
}
func (g *Game) boxCommand(command, arg string) error {
	n, err := strconv.Atoi(arg)
	n--
	if err != nil {
		return errors.New("请输入数字编号，例如 deposit 2 / withdraw 1")
	}
	if command == "存入" || command == "deposit" {
		if len(g.Party) <= 1 {
			return errors.New("队伍必须保留至少一只伙伴")
		}
		if n < 0 || n >= len(g.Party) || len(g.Box) >= 600 {
			return errors.New("队伍编号不存在或电脑已满")
		}
		g.Box = append(g.Box, g.Party[n])
		g.say("%s存入电脑。", g.Party[n].Name())
		g.Party = append(g.Party[:n], g.Party[n+1:]...)
	} else {
		if len(g.Party) >= 6 || n < 0 || n >= len(g.Box) {
			return errors.New("队伍已满或电脑编号不存在")
		}
		g.Party = append(g.Party, g.Box[n])
		g.say("%s加入队伍。", g.Box[n].Name())
		g.Box = append(g.Box[:n], g.Box[n+1:]...)
	}
	return nil
}
func (g *Game) evolveCommand(command string, words []string) error {
	trade := command == "交换" || command == "trade"
	index := 0
	item := ""
	if trade {
		if len(words) > 1 {
			n, _ := strconv.Atoi(words[1])
			index = n - 1
		}
	} else {
		if len(words) < 2 {
			return errors.New("例如：evolve thunderstone 1（或 evolve 15 1）")
		}
		item = words[1]
		if len(words) > 2 {
			n, _ := strconv.Atoi(words[2])
			index = n - 1
		}
		if g.Items[item] <= 0 {
			return errors.New("背包中没有该进化石")
		}
	}
	if index < 0 || index >= len(g.Party) {
		return errors.New("队伍编号不存在")
	}
	m := &g.Party[index]
	for _, e := range Dex[m.Species].Evolves {
		if (trade && e.Trade) || (!trade && e.Item == item && item != "") {
			if !trade {
				g.Items[item]--
			} else {
				g.say("研究助手与你进行一次本地交换并归还伙伴。")
			}
			g.evolveMonster(m, e.Species)
			return nil
		}
	}
	return errors.New("所选伙伴不满足这种进化方式；等级进化会在升级时自动进行")
}
