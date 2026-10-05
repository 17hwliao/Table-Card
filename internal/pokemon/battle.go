package pokemon

import (
	"errors"
	"strconv"
)

func (g *Game) battleCommand(command string, words []string) error {
	b := g.Battle
	switch command {
	case "招式", "攻击", "move", "attack":
		n := 1
		if len(words) > 1 {
			n, _ = strconv.Atoi(words[1])
		}
		if n < 1 || n > 4 {
			return errors.New("招式编号为1–4")
		}
		p := &g.Party[b.Active]
		hasPP := false
		for _, pp := range p.PP {
			hasPP = hasPP || pp > 0
		}
		if hasPP && p.PP[n-1] == 0 {
			return errors.New("该招式PP耗尽，请选择其他招式")
		}
		foe := &b.Foes[b.Enemy]
		enemyMove := g.enemyMove(foe, p)
		if speed(p) > speed(foe) || (speed(p) == speed(foe) && g.rng.Intn(2) == 0) {
			g.attack(p, foe, n-1)
			if foe.HP > 0 && p.HP > 0 {
				g.attack(foe, p, enemyMove)
			}
		} else {
			g.attack(foe, p, enemyMove)
			if p.HP > 0 && foe.HP > 0 {
				g.attack(p, foe, n-1)
			}
		}
		g.endTurn()
	case "捕捉", "抓", "catch":
		ball := "精灵球"
		if len(words) > 1 {
			ball = words[1]
		}
		return g.throwBall(ball)
	case "换精灵", "换", "switch":
		if len(words) < 2 {
			return errors.New("例如：switch 2")
		}
		n, _ := strconv.Atoi(words[1])
		n--
		if n < 0 || n >= len(g.Party) || g.Party[n].HP <= 0 || n == b.Active {
			return errors.New("请选择另一只仍有战斗能力的队伍精灵")
		}
		b.Active = n
		g.say("换上%s！", g.Party[n].Name())
		g.enemyTurn()
	case "使用", "use":
		return g.use(words, true)
	case "逃跑", "逃", "run":
		if b.Kind != "wild" {
			return errors.New("训练者对战不能逃跑")
		}
		p, f := &g.Party[b.Active], &b.Foes[b.Enemy]
		chance := 0.7 + float64(speed(p)-speed(f))/200
		if g.rng.Float64() < max(0.1, min(0.95, chance)) {
			g.say("成功离开草丛。散步后还可以再次探索。")
			g.Battle = nil
		} else {
			g.say("没有逃脱！")
			g.enemyTurn()
		}
	default:
		return errors.New("正在战斗：move 1–4 / catch 1 / switch 2 / use potion 1 / run")
	}
	return nil
}
func speed(m *Monster) int {
	s := m.stat(Dex[m.Species].Speed)
	if m.Status == "麻痹" {
		s = max(1, s/4)
	}
	return s
}

// Fifteen-type matchup table. Unlisted pairs are neutral. We deliberately do
// not emulate cartridge bugs or its full move catalogue.
var effectiveness = map[int]map[int]float64{
	1: {6: .5, 8: 0}, 2: {1: 2, 3: .5, 4: .5, 6: 2, 7: .5, 8: 0, 14: .5, 15: 2},
	3: {2: 2, 6: .5, 7: 2, 12: 2, 13: .5}, 4: {4: .5, 5: .5, 6: .5, 7: 2, 8: .5, 12: 2},
	5: {3: 0, 4: 2, 6: 2, 7: .5, 10: 2, 12: .5, 13: 2}, 6: {2: .5, 3: 2, 5: .5, 7: 2, 10: 2, 15: 2},
	7: {2: .5, 3: .5, 4: 2, 8: .5, 10: .5, 12: 2, 14: 2}, 8: {1: 0, 8: 2, 14: 2},
	10: {6: .5, 7: 2, 10: .5, 11: .5, 12: 2, 15: 2, 16: .5}, 11: {5: 2, 6: 2, 10: 2, 11: .5, 12: .5, 16: .5},
	12: {3: .5, 4: .5, 5: 2, 6: 2, 7: .5, 10: .5, 11: 2, 12: .5, 16: .5},
	13: {3: 2, 5: 0, 11: 2, 12: .5, 13: .5, 16: .5}, 14: {2: 2, 4: 2, 14: .5},
	15: {3: 2, 5: 2, 11: .5, 12: 2, 15: .5, 16: 2}, 16: {16: 2},
}

func typeFactor(t int, m *Monster) float64 {
	f := 1.0
	for _, target := range Dex[m.Species].Types {
		if v, ok := effectiveness[t][target]; ok {
			f *= v
		}
	}
	return f
}
func damageEstimate(a, d *Monster, move Move) float64 {
	f := typeFactor(move.Type, d)
	if f == 0 {
		return 0
	}
	if move.Name == "龙之怒" {
		return 40
	}
	if move.Name == "黑夜魔影" {
		return float64(a.Level)
	}
	if move.Power == 0 {
		return 0
	}
	atk, def := a.stat(Dex[a.Species].Attack), d.stat(Dex[d.Species].Defense)
	if move.Type >= 10 {
		atk = a.stat(Dex[a.Species].Special)
		def = d.stat(Dex[d.Species].Special)
	} else if a.Status == "灼伤" {
		atk = max(1, atk/2)
	}
	stab := 1.0
	for _, t := range Dex[a.Species].Types {
		if move.Type == t {
			stab = 1.5
		}
	}
	return float64(((2*a.Level/5+2)*move.Power*atk/max(1, def))/50+2) * stab * f
}
func (g *Game) enemyMove(a, d *Monster) int {
	best := -1
	score := -1.0
	for i, id := range a.Moves {
		if a.PP[i] <= 0 {
			continue
		}
		move := Moves[id]
		s := damageEstimate(a, d, move)
		if move.Power == 0 && move.Status != "" && d.Status == "" && typeFactor(move.Type, d) > 0 {
			s = 12 + g.rng.Float64()*18
		}
		s *= float64(move.Accuracy) / 100
		if s > score {
			best, score = i, s
		}
	}
	if best < 0 {
		return 0
	}
	return best
}
func (g *Game) attack(a, d *Monster, slot int) {
	if a.HP <= 0 || d.HP <= 0 {
		return
	}
	switch a.Status {
	case "睡眠":
		a.Sleep--
		if a.Sleep > 0 {
			g.say("%s仍在睡眠。", a.Name())
			return
		}
		a.Status = ""
		g.say("%s醒来了。", a.Name())
	case "冰冻":
		if g.rng.Intn(5) != 0 {
			g.say("%s被冻结，无法行动。", a.Name())
			return
		}
		a.Status = ""
		g.say("%s解冻了。", a.Name())
	case "麻痹":
		if g.rng.Intn(4) == 0 {
			g.say("%s因麻痹无法行动。", a.Name())
			return
		}
	}
	move := Moves[a.Moves[slot]]
	struggle := true
	for _, pp := range a.PP {
		if pp > 0 {
			struggle = false
		}
	}
	if struggle {
		move = Move{Name: "挣扎", Type: 1, Power: 50, Accuracy: 100}
	} else {
		if a.PP[slot] <= 0 {
			return
		}
		a.PP[slot]--
	}
	g.say("%s使用%s！", a.Name(), move.Name)
	if g.rng.Intn(100) >= move.Accuracy {
		g.say("攻击没有命中。")
		return
	}
	factor := typeFactor(move.Type, d)
	if struggle {
		factor = 1
	}
	if factor == 0 {
		g.say("对%s没有效果。", d.Name())
		return
	}
	amount := 0
	if move.Power > 0 || move.Name == "黑夜魔影" {
		estimate := damageEstimate(a, d, move)
		if struggle {
			estimate = float64(a.Level + 10)
		}
		amount = max(1, int(estimate*float64(217+g.rng.Intn(39))/255))
		if move.Name == "龙之怒" || move.Name == "黑夜魔影" {
			amount = int(estimate)
		}
		amount = min(amount, d.HP)
		d.HP -= amount
		g.say("造成%d伤害，%s HP %d/%d。", amount, d.Name(), d.HP, d.MaxHP())
		if factor > 1 {
			g.say("效果拔群！")
		} else if factor < 1 {
			g.say("效果不佳。")
		}
	}
	if move.Name == "吸血" {
		a.HP = min(a.MaxHP(), a.HP+max(1, amount/2))
	}
	if struggle {
		a.HP = max(0, a.HP-max(1, amount/4))
	}
	if d.HP > 0 && d.Status == "" && move.Status != "" && g.rng.Intn(100) < move.Chance {
		immune := false
		for _, t := range Dex[d.Species].Types {
			immune = immune || (move.Status == "中毒" && t == 4) || (move.Status == "灼伤" && t == 10) || (move.Status == "冰冻" && t == 15)
		}
		if !immune {
			d.Status = move.Status
			d.Sleep = 2 + g.rng.Intn(3)
			g.say("%s陷入%s。", d.Name(), d.Status)
		}
	}
}
func (g *Game) enemyTurn() {
	if g.Battle == nil {
		return
	}
	b := g.Battle
	p, f := &g.Party[b.Active], &b.Foes[b.Enemy]
	g.attack(f, p, g.enemyMove(f, p))
	g.endTurn()
}
func (g *Game) endTurn() {
	b := g.Battle
	if b == nil {
		return
	}
	b.Turns++
	p, f := &g.Party[b.Active], &b.Foes[b.Enemy]
	for _, m := range []*Monster{p, f} {
		if m.HP > 0 && (m.Status == "中毒" || m.Status == "灼伤") {
			amount := max(1, m.MaxHP()/8)
			m.HP = max(0, m.HP-amount)
			g.say("%s受到%s伤害，HP %d/%d。", m.Name(), m.Status, m.HP, m.MaxHP())
		}
	}
	if f.HP <= 0 {
		g.say("%s失去战斗能力。", f.Name())
		g.gainExp(b.Active, Dex[f.Species].BaseExp*f.Level/7)
	}
	if !g.Ready() {
		loss := g.Money / 10
		g.Money -= loss
		if !g.Completed {
			g.Elite = 0
		}
		g.Battle = nil
		g.heal()
		g.say("队伍失去战斗能力。救护员带你恢复，支付%d救护费用。本次挑战未计胜利。", loss)
		return
	}
	if p.HP <= 0 {
		b.Active = g.activeIndex()
		g.say("自动换上%s。", g.Party[b.Active].Name())
	}
	if f.HP <= 0 {
		b.Enemy++
		if b.Enemy >= len(b.Foes) {
			g.victory()
			return
		}
		g.say("%s派出%s Lv%d。", b.Name, b.Foes[b.Enemy].Name(), b.Foes[b.Enemy].Level)
	}
}
func (g *Game) gainExp(index, amount int) {
	m := &g.Party[index]
	g.say("%s得到%d经验。", m.Name(), amount)
	m.Exp = min(1000000, m.Exp+max(1, amount))
	for m.Level < 100 && m.Exp >= ExperienceAtLevel(m.Level+1) {
		oldHP := m.MaxHP()
		m.Level++
		if m.HP > 0 {
			m.HP += m.MaxHP() - oldHP
		}
		g.refreshMoves(m)
		g.say("%s升到Lv%d。", m.Name(), m.Level)
		for _, e := range Dex[m.Species].Evolves {
			if e.Level > 0 && m.Level >= e.Level {
				g.evolveMonster(m, e.Species)
				break
			}
		}
	}
}
func (g *Game) refreshMoves(m *Monster) {
	next := moveSet(m.Species, m.Level)
	if m.Signature {
		next[3] = signatureMove(m.Species)
	}
	for i, id := range next {
		if id != m.Moves[i] {
			m.PP[i] = Moves[id].PP
		}
	}
	m.Moves = next
}
func (g *Game) evolveMonster(m *Monster, id int) {
	old := m.Name()
	hp := m.MaxHP()
	m.Species = id
	if m.HP > 0 {
		m.HP = max(1, min(m.MaxHP(), m.HP+m.MaxHP()-hp))
	}
	g.refreshMoves(m)
	g.Caught[id] = true
	g.Seen[id] = true
	g.say("%s进化成了%s！", old, m.Name())
}
func (g *Game) victory() {
	b := g.Battle
	g.Battle = nil
	if b.Kind == "wild" {
		g.say("野生对战结束。捕获需要在对手倒下前投球。")
		return
	}
	reward := b.Foes[len(b.Foes)-1].Level * 60
	g.Money = min(1000000000, g.Money+reward)
	g.say("战胜%s，获得%d零钱。", b.Name, reward)
	switch b.Kind {
	case "trainer":
		if g.TrainerWins[b.Area] < b.Challenge {
			g.TrainerWins[b.Area] = b.Challenge
		}
		g.say("%s挑战进度：%d/6。", Areas[b.Area].Name, g.TrainerWins[b.Area])
	case "gym":
		g.Flags[b.Flag] = true
		g.Badges = append(g.Badges, Areas[b.Area].Badge)
		g.say("获得%s！输入 next 继续旅程。", Areas[b.Area].Badge)
	case "event":
		g.Flags[b.Flag] = true
		g.eventReward(b.Flag)
	case "league":
		g.Elite++
		if g.Elite >= 5 {
			g.Completed = true
			g.Unlocked = 11
			g.Flags["champion"] = true
			g.say("联盟殿堂记录下你与伙伴的名字。你已成为冠军！大木博士邀请你继续图鉴调查；前进可进入华蓝洞窟后篇。")
		} else {
			g.say("联盟进度%d/5。继续输入 league 迎战下一位；治疗会重置本轮进度。", g.Elite)
		}
	}
}
