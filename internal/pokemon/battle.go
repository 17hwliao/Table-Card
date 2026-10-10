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
		if p.Moves[n-1] < 0 {
			return errors.New("该招式槽尚未学习招式")
		}
		hasPP := false
		for _, pp := range p.PP {
			hasPP = hasPP || pp > 0
		}
		if hasPP && p.PP[n-1] == 0 && p.Combat.ChargeMove == 0 && p.Combat.RepeatTurns == 0 {
			return errors.New("该招式PP耗尽，请选择其他招式")
		}
		if p.Combat.DisableSlot == n && p.Combat.DisableTurns > 0 {
			return errors.New("该招式暂时被封锁，请选择其他招式")
		}
		foe := &b.Foes[b.Enemy]
		enemyMove := g.enemyMove(foe, p)
		p.Combat.LastDamage = 0
		foe.Combat.LastDamage = 0
		pp, fp := movePriority(p, n-1), movePriority(foe, enemyMove)
		if pp > fp || (pp == fp && (speed(p) > speed(foe) || (speed(p) == speed(foe) && g.rng.Intn(2) == 0))) {
			g.attack(p, foe, n-1)
			if g.Battle != nil && foe.HP > 0 && p.HP > 0 {
				g.attack(foe, p, enemyMove)
			}
		} else {
			g.attack(foe, p, enemyMove)
			if g.Battle != nil && p.HP > 0 && foe.HP > 0 {
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
		ResetCombat(&g.Party[b.Active])
		ResetCombat(&g.Party[n])
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
			for i := range g.Party {
				ResetCombat(&g.Party[i])
			}
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
	s := combatStat(m, Dex[m.Species].Speed, 3)
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
	for _, target := range EffectiveTypes(m) {
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
	if move.Key == "DRAGON_RAGE" {
		return 40
	}
	if move.Key == "NIGHT_SHADE" || move.Key == "SEISMIC_TOSS" {
		return float64(a.Level)
	}
	if move.Key == "SONICBOOM" {
		return 20
	}
	if move.Key == "SUPER_FANG" {
		return float64(max(1, d.HP/2))
	}
	if move.Power == 0 {
		return 0
	}
	atk, def := combatStat(a, Dex[a.Species].Attack, 0), combatStat(d, Dex[d.Species].Defense, 1)
	if move.Type >= 10 {
		atk = combatStat(a, Dex[a.Species].Special, 2)
		def = combatStat(d, Dex[d.Species].Special, 2)
		if d.Combat.LightScreen {
			def *= 2
		}
	} else if a.Status == "灼伤" {
		atk = max(1, atk/2)
	}
	if move.Type < 10 && d.Combat.Reflect {
		def *= 2
	}
	if move.Effect == "EXPLODE_EFFECT" {
		def = max(1, def/2)
	}
	stab := 1.0
	for _, t := range EffectiveTypes(a) {
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
		if id < 0 || a.PP[i] <= 0 || (a.Combat.DisableSlot == i+1 && a.Combat.DisableTurns > 0) {
			continue
		}
		move := Moves[id]
		s := damageEstimate(a, d, move)
		if status, _ := moveStatus(move); move.Power == 0 && status != "" && d.Status == "" && typeFactor(move.Type, d) > 0 {
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
	g.EndResidual(p, f)
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
		ResetCombat(p)
		b.Active = g.activeIndex()
		ResetCombat(&g.Party[b.Active])
		g.say("自动换上%s。", g.Party[b.Active].Name())
	}
	if f.HP <= 0 {
		ResetCombat(f)
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
	m.Exp = min(ExperienceForSpecies(m.Species, 100), m.Exp+max(1, amount))
	for m.Level < 100 && m.Exp >= ExperienceForSpecies(m.Species, m.Level+1) {
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
	if m.Combat.TransformedSpecies > 0 {
		original := *m
		ResetCombat(&original)
		g.refreshMoves(&original)
		m.Combat.OriginalMoves = original.Moves
		m.Combat.OriginalPP = original.PP
		return
	}
	SanitizeMonster(m)
	for _, entry := range Dex[m.Species].Learnset {
		if entry.Level != m.Level {
			continue
		}
		known := false
		for _, id := range m.Moves {
			known = known || id == entry.Move
		}
		if known {
			continue
		}
		slot := -1
		for i, id := range m.Moves {
			if id < 0 {
				slot = i
				break
			}
		}
		if slot < 0 {
			slot = 0
			copy(m.Moves[:3], m.Moves[1:])
			copy(m.PP[:3], m.PP[1:])
			slot = 3
		}
		m.Moves[slot] = entry.Move
		m.PP[slot] = Moves[entry.Move].PP
		g.say("%s学会%s！", m.Name(), Moves[entry.Move].Name)
	}
	if m.Signature {
		m.Moves[3] = signatureMove(m.Species)
		m.PP[3] = min(m.PP[3], Moves[m.Moves[3]].PP)
	}
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
	for i := range g.Party {
		ResetCombat(&g.Party[i])
	}
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
