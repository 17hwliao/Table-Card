package pokemon

import (
	"errors"
	"math"
)

// CaptureProbability integrates the two uniform random draws used by Gen I.
// Great Ball's HP divisor is distinct; a generic modern ball multiplier would
// give different totals. This is an independently expressed probability model.
func CaptureProbability(m Monster, ball string) float64 {
	if ball == "大师球" {
		return 1
	}
	ceiling, divisor := 255, 12
	if ball == "超级球" {
		ceiling, divisor = 200, 8
	}
	if ball == "高级球" {
		ceiling = 150
	}
	bonus := 0
	switch m.Status {
	case "睡眠", "冰冻":
		bonus = 25
	case "中毒", "灼伤", "麻痹":
		bonus = 12
	}
	n := ceiling + 1
	hpFactor := (m.MaxHP() * 255 / divisor) / max(1, m.HP/4)
	return float64(min(bonus, n))/float64(n) + float64(max(0, min(Dex[m.Species].CatchRate+1, n-bonus)))/float64(n)*float64(min(hpFactor+1, 256))/256
}
func (g *Game) throwBall(ball string) error {
	if g.Battle.Kind != "wild" {
		return errors.New("不能捕获训练者的伙伴")
	}
	if ball != "精灵球" && ball != "超级球" && ball != "高级球" && ball != "大师球" {
		return errors.New("可用球种：精灵球、超级球、高级球、大师球")
	}
	if len(g.Party) >= 6 && len(g.Box) >= 600 {
		return errors.New("队伍与电脑已满，请先整理电脑")
	}
	if g.Items[ball] <= 0 {
		return errors.New("背包中没有这种球，请先购买")
	}
	foe := g.Battle.Foes[g.Battle.Enemy]
	p := CaptureProbability(foe, ball)
	gate := math.Cbrt(p)
	c := &Capture{Ball: ball, Probability: p}
	for i := range c.Passed {
		c.Passed[i] = g.rng.Float64() < gate
	}
	g.Items[ball]--
	g.Capture = c
	g.say("抛出%s！捕捉%s；本次总成功率约%.1f%%。", ball, foe.Name(), p*100)
	return nil
}

// AdvanceCapture reveals one pre-rolled gate. Saving the gates prevents reloads
// during the animation from generating a different capture outcome.
func (g *Game) AdvanceCapture() {
	c := g.Capture
	if c == nil || g.Battle == nil {
		return
	}
	step := c.Step
	if !c.Passed[step] {
		g.say("第%d次摇动失败，精灵球打开了！", step+1)
		g.Capture = nil
		g.enemyTurn()
		return
	}
	c.Step++
	g.say("精灵球第%d次摇动……", c.Step)
	if c.Step == 3 {
		foe := g.Battle.Foes[g.Battle.Enemy]
		g.say("咔哒！三次摇动通过，捕获%s！", foe.Name())
		g.obtain(foe)
		g.Battle = nil
		g.Capture = nil
	}
}
func (g *Game) obtain(m Monster) {
	if len(g.Party) >= 6 && len(g.Box) >= 600 {
		g.say("电脑已满，无法接收赠送精灵；请先整理电脑。")
		return
	}
	g.Caught[m.Species] = true
	g.Seen[m.Species] = true
	if len(g.Party) < 6 {
		g.Party = append(g.Party, m)
		g.say("%s加入携带队伍。", m.Name())
	} else {
		g.Box = append(g.Box, m)
		g.say("队伍已满，%s送到电脑第%d格。", m.Name(), len(g.Box))
	}
}
