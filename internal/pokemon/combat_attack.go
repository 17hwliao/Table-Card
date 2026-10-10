package pokemon

func (g *Game) attack(a, d *Monster, slot int) {
	if a.HP <= 0 || d.HP <= 0 {
		return
	}
	if a.Combat.Flinch {
		g.say("%s畏缩，无法行动。", a.Name())
		a.Combat.Flinch = false
		return
	}
	if a.Combat.Recharge > 0 {
		a.Combat.Recharge--
		g.say("%s需要休息一回合。", a.Name())
		return
	}
	if a.Combat.TrapTurns > 0 {
		a.Combat.TrapTurns--
		loss := min(a.HP, max(1, a.Combat.TrapDamage))
		a.HP -= loss
		g.say("%s受到束缚%d伤害，无法行动。", a.Name(), loss)
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
			g.say("%s被冰冻，无法行动。", a.Name())
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
	if a.Combat.Confusion > 0 {
		a.Combat.Confusion--
		g.say("%s陷入混乱。", a.Name())
		if g.rng.Intn(2) == 0 {
			loss := max(1, ((2*a.Level/5+2)*40*combatStat(a, Dex[a.Species].Attack, 0)/combatStat(a, Dex[a.Species].Defense, 1))/50+2)
			loss = min(loss, a.HP)
			a.HP -= loss
			g.say("%s因混乱伤害自身%d HP。", a.Name(), loss)
			return
		}
		if a.Combat.Confusion == 0 {
			g.say("%s恢复清醒。", a.Name())
		}
	}
	if a.Combat.DisableTurns > 0 {
		a.Combat.DisableTurns--
		if a.Combat.DisableTurns == 0 {
			a.Combat.DisableSlot = 0
			g.say("%s的招式封锁结束。", a.Name())
		}
	}
	if a.Combat.BideTurns > 0 {
		a.Combat.BideTurns--
		if a.Combat.BideTurns > 0 {
			g.say("%s继续忍耐。", a.Name())
			return
		}
		loss := min(d.HP, a.Combat.BideDamage*2)
		d.HP -= loss
		a.Combat.BideDamage = 0
		g.say("%s释放忍耐，造成%d伤害。", a.Name(), loss)
		return
	}
	if a.Combat.RepeatTurns > 0 {
		slot = a.Combat.RepeatMove - 1
	}
	if slot < 0 || slot >= 4 {
		return
	}
	struggle := true
	for i, pp := range a.PP {
		if a.Moves[i] >= 0 && pp > 0 && (a.Combat.DisableSlot != i+1 || a.Combat.DisableTurns <= 0) {
			struggle = false
		}
	}
	move := Move{Name: "挣扎", Key: "STRUGGLE", Type: 1, Power: 50, Accuracy: 100, Effect: "RECOIL_EFFECT"}
	releasing := a.Combat.ChargeMove > 0
	if releasing {
		move = Moves[a.Combat.ChargeMove-1]
		a.Combat.ChargeMove = 0
	} else if !struggle {
		id := a.Moves[slot]
		if id < 0 || id >= len(Moves) || a.PP[slot] <= 0 {
			return
		}
		if a.Combat.DisableSlot == slot+1 && a.Combat.DisableTurns > 0 {
			g.say("%s的%s暂时被封锁。", a.Name(), Moves[id].Name)
			return
		}
		move = Moves[id]
		if a.Combat.RepeatTurns <= 0 {
			a.PP[slot]--
		}
	}
	a.Combat.LastMove = move.ID + 1
	g.say("%s使用%s！", a.Name(), move.Name)
	if move.Effect == "METRONOME_EFFECT" {
		choices := []int{}
		for i, m := range Moves {
			if m.Key != "" && m.Key != "METRONOME" && m.Key != "STRUGGLE" {
				choices = append(choices, i)
			}
		}
		move = Moves[choices[g.rng.Intn(len(choices))]]
		g.say("挥指唤出%s。", move.Name)
	}
	if move.Effect == "MIRROR_MOVE_EFFECT" || move.Effect == "MIMIC_EFFECT" {
		id := d.Combat.LastMove - 1
		if id < 0 || id >= len(Moves) || Moves[id].Effect == "MIMIC_EFFECT" || Moves[id].Effect == "MIRROR_MOVE_EFFECT" {
			g.say("对手没有可复制的招式。 ")
			return
		}
		move = Moves[id]
		g.say("复制了%s。", move.Name)
	}
	if move.Effect == "TRANSFORM_EFFECT" {
		if d.Combat.Substitute > 0 {
			g.say("对手的替身阻挡了变身。 ")
			return
		}
		if a.Combat.TransformedSpecies == 0 {
			a.Combat.OriginalMoves = a.Moves
			a.Combat.OriginalPP = a.PP
		}
		a.Combat.TransformedSpecies = d.Species
		if d.Combat.TransformedSpecies > 0 {
			a.Combat.TransformedSpecies = d.Combat.TransformedSpecies
		}
		for i, base := range []int{Dex[d.Species].Attack, Dex[d.Species].Defense, Dex[d.Species].Special, Dex[d.Species].Speed} {
			a.Combat.CopiedStats[i] = rawCombatStat(d, base, i)
		}
		a.Combat.Stages = d.Combat.Stages
		types := EffectiveTypes(d)
		a.Combat.ConvertedCount = len(types)
		copy(a.Combat.ConvertedTypes[:], types)
		a.Moves = d.Moves
		for i, id := range a.Moves {
			a.PP[i] = 0
			if id >= 0 {
				a.PP[i] = min(5, Moves[id].PP)
			}
		}
		g.say("%s变身为%s的战斗形态，复制属性、能力与招式（每招5PP）；自身HP保持%d/%d。", a.Name(), d.Name(), a.HP, a.MaxHP())
		return
	}
	if move.Effect == "CONVERSION_EFFECT" {
		types := EffectiveTypes(d)
		a.Combat.ConvertedCount = len(types)
		copy(a.Combat.ConvertedTypes[:], types)
		g.say("%s复制了对手当前属性；换人后恢复原本属性。", a.Name())
		return
	}
	if move.Effect == "SPLASH_EFFECT" {
		g.say("但是没有发生任何事情。 ")
		return
	}
	if (move.Effect == "CHARGE_EFFECT" || move.Effect == "FLY_EFFECT") && !releasing {
		a.Combat.ChargeMove = move.ID + 1
		g.say("%s开始蓄力，下次行动发动攻击。", a.Name())
		return
	}
	index, delta, chance, self := stageEffect(move.Effect)
	selfTarget := self || move.Effect == "HEAL_EFFECT" || move.Effect == "REFLECT_EFFECT" || move.Effect == "LIGHT_SCREEN_EFFECT" || move.Effect == "MIST_EFFECT" || move.Effect == "FOCUS_ENERGY_EFFECT" || move.Effect == "SUBSTITUTE_EFFECT" || move.Effect == "BIDE_EFFECT" || move.Effect == "HAZE_EFFECT"
	accuracy := float64(move.Accuracy)
	if !selfTarget {
		accuracy *= accuracyFactor(a.Combat.Stages[4]) / accuracyFactor(d.Combat.Stages[5])
	}
	if !selfTarget && move.Effect != "SWIFT_EFFECT" && g.rng.Float64()*100 >= accuracy {
		g.say("招式没有命中。 ")
		if move.Effect == "JUMP_KICK_EFFECT" {
			a.HP = max(0, a.HP-1)
			g.say("%s因踢空损失1 HP。", a.Name())
		}
		return
	}
	factor := typeFactor(move.Type, d)
	if struggle {
		factor = 1
	}
	if !selfTarget && factor == 0 && (move.Power > 0 || move.Key == "NIGHT_SHADE" || move.Key == "THUNDER_WAVE") {
		g.say("对%s没有效果。", d.Name())
		return
	}
	if chance > 0 && g.rng.Intn(100) < chance {
		target := d
		if self {
			target = a
		}
		if !self && (d.Combat.Mist || d.Combat.Substitute > 0) {
			g.say("%s的能力变化被保护效果阻挡。", d.Name())
		} else {
			g.changeStage(target, index, delta)
		}
		if move.Power == 0 {
			return
		}
	}
	switch move.Effect {
	case "HEAL_EFFECT":
		old := a.HP
		if move.Key == "REST" {
			a.HP = a.MaxHP()
			a.Status = "睡眠"
			a.Sleep = 2
			g.say("%s睡眠2次行动并恢复%d HP。", a.Name(), a.HP-old)
		} else {
			a.HP = min(a.MaxHP(), a.HP+max(1, a.MaxHP()/2))
			g.say("%s恢复%d HP。", a.Name(), a.HP-old)
		}
		return
	case "REFLECT_EFFECT":
		a.Combat.Reflect = true
		g.say("%s建立反射壁，物理防御加倍。", a.Name())
		return
	case "LIGHT_SCREEN_EFFECT":
		a.Combat.LightScreen = true
		g.say("%s建立光墙，特殊防御加倍。", a.Name())
		return
	case "MIST_EFFECT":
		a.Combat.Mist = true
		g.say("%s受到白雾保护。", a.Name())
		return
	case "FOCUS_ENERGY_EFFECT":
		a.Combat.Focus = true
		g.say("%s集中精神，要害率提升。", a.Name())
		return
	case "HAZE_EFFECT":
		ResetCombat(a)
		ResetCombat(d)
		g.say("白雾清除了双方战斗能力变化与临时效果。 ")
		return
	case "BIDE_EFFECT":
		a.Combat.BideTurns = 2
		a.Combat.BideDamage = 0
		g.say("%s开始忍耐2次行动。", a.Name())
		return
	case "DISABLE_EFFECT":
		found := 0
		for i, id := range d.Moves {
			if id >= 0 && id+1 == d.Combat.LastMove {
				found = i + 1
			}
		}
		if found == 0 {
			g.say("没有可封锁的最近招式。 ")
			return
		}
		d.Combat.DisableSlot = found
		d.Combat.DisableTurns = 1 + g.rng.Intn(7)
		g.say("%s的%s被封锁%d次行动。", d.Name(), Moves[d.Moves[found-1]].Name, d.Combat.DisableTurns)
		return
	case "SUBSTITUTE_EFFECT":
		cost := max(1, a.MaxHP()/4)
		if a.Combat.Substitute > 0 || a.HP <= cost {
			g.say("HP不足或替身已经存在。 ")
			return
		}
		a.HP -= cost
		a.Combat.Substitute = cost
		g.say("%s消耗%d HP制造替身。", a.Name(), cost)
		return
	case "SWITCH_AND_TELEPORT_EFFECT":
		if g.Battle != nil && g.Battle.Kind == "wild" {
			g.say("野生遭遇结束。 ")
			g.Battle = nil
		} else {
			g.say("训练家/玩家对战无法用此招式结束。 ")
		}
		return
	case "LEECH_SEED_EFFECT":
		if hasType(d, 12) || d.Combat.Substitute > 0 {
			g.say("寄生种子被阻挡。 ")
		} else {
			d.Combat.Seeded = true
			g.say("%s被种下寄生种子。", d.Name())
		}
		return
	}
	hits := 1
	switch move.Effect {
	case "ATTACK_TWICE_EFFECT", "TWINEEDLE_EFFECT":
		hits = 2
	case "TWO_TO_FIVE_ATTACKS_EFFECT":
		roll := g.rng.Intn(8)
		switch {
		case roll < 3:
			hits = 2
		case roll < 6:
			hits = 3
		case roll == 6:
			hits = 4
		default:
			hits = 5
		}
	}
	total := 0
	hitSubstitute := d.Combat.Substitute > 0
	for h := 0; h < hits && d.HP > 0; h++ {
		estimate := damageEstimate(a, d, move)
		if move.Key == "PSYWAVE" {
			estimate = float64(1 + g.rng.Intn(max(1, a.Level*3/2)))
		}
		if struggle {
			neutral := move
			neutral.Type = 0
			estimate = damageEstimate(a, d, neutral)
		}
		if move.Key == "COUNTER" {
			estimate = float64(a.Combat.LastDamage * 2)
		}
		if move.Effect == "OHKO_EFFECT" {
			if a.Level < d.Level || speed(a) < speed(d) {
				g.say("一击必杀条件不满足。 ")
				return
			}
			estimate = float64(d.HP)
		}
		fixed := move.Key == "DRAGON_RAGE" || move.Key == "SONICBOOM" || move.Key == "NIGHT_SHADE" || move.Key == "SEISMIC_TOSS" || move.Key == "PSYWAVE" || move.Key == "SUPER_FANG" || move.Key == "COUNTER" || move.Effect == "OHKO_EFFECT"
		if move.Effect == "DREAM_EATER_EFFECT" && d.Status != "睡眠" {
			g.say("食梦仅能对睡眠中的对手生效。 ")
			return
		}
		if move.Power <= 0 && !fixed {
			break
		}
		critChance := 16
		if a.Combat.Focus || move.Key == "SLASH" || move.Key == "RAZOR_LEAF" || move.Key == "CRABHAMMER" || move.Key == "KARATE_CHOP" {
			critChance = 4
		}
		if !fixed && g.rng.Intn(critChance) == 0 {
			estimate *= 1.5
			g.say("击中要害！")
		}
		if !fixed {
			estimate *= float64(217+g.rng.Intn(39)) / 255
		}
		amount := max(1, int(estimate))
		if move.Key == "COUNTER" && estimate == 0 {
			g.say("没有可返还的普通/格斗伤害。 ")
			return
		}
		if d.Combat.Substitute > 0 {
			amount = min(amount, d.Combat.Substitute)
			d.Combat.Substitute -= amount
			g.say("替身承受%d伤害（剩余%d）。", amount, d.Combat.Substitute)
			if d.Combat.Substitute == 0 {
				g.say("%s的替身消失。", d.Name())
			}
			continue
		}
		amount = min(amount, d.HP)
		d.HP -= amount
		total += amount
		d.Combat.BideDamage += amount
		if last := d.Combat.LastMove - 1; last >= 0 && last < len(Moves) && Moves[last].Effect == "RAGE_EFFECT" {
			g.changeStage(d, 0, 1)
		}
		if move.Type == 1 || move.Type == 2 {
			d.Combat.LastDamage += amount
		}
		g.say("造成%d伤害，%s HP %d/%d。", amount, d.Name(), d.HP, d.MaxHP())
	}
	if hits > 1 {
		g.say("连续攻击%d次。", hits)
	}
	if total > 0 && factor != 1 {
		if factor > 1 {
			g.say("效果拔群！")
		} else {
			g.say("效果不佳。")
		}
	}
	if move.Effect == "DRAIN_HP_EFFECT" || move.Effect == "DREAM_EATER_EFFECT" {
		old := a.HP
		a.HP = min(a.MaxHP(), a.HP+max(1, total/2))
		if total > 0 {
			g.say("%s吸取%d HP。", a.Name(), a.HP-old)
		}
	}
	if move.Effect == "RECOIL_EFFECT" && total > 0 {
		loss := min(a.HP, max(1, total/4))
		a.HP -= loss
		g.say("%s受到%d反作用伤害。", a.Name(), loss)
	}
	if move.Effect == "EXPLODE_EFFECT" {
		a.HP = 0
		g.say("%s失去战斗能力。", a.Name())
	}
	if move.Effect == "HYPER_BEAM_EFFECT" && d.HP > 0 {
		a.Combat.Recharge = 1
	}
	if move.Effect == "THRASH_PETAL_DANCE_EFFECT" {
		if a.Combat.RepeatTurns == 0 {
			a.Combat.RepeatTurns = 2 + g.rng.Intn(2)
			a.Combat.RepeatMove = slot + 1
		}
		a.Combat.RepeatTurns--
		if a.Combat.RepeatTurns == 0 {
			a.Combat.Confusion = 2 + g.rng.Intn(4)
			g.say("连续攻击结束，%s陷入混乱。", a.Name())
		}
	}
	if hitSubstitute || d.HP <= 0 {
		return
	}
	if move.Effect == "TRAPPING_EFFECT" && total > 0 {
		d.Combat.TrapTurns = 1 + g.rng.Intn(4)
		d.Combat.TrapDamage = total
		g.say("%s受到束缚%d次行动。", d.Name(), d.Combat.TrapTurns)
	}
	if move.Effect == "CONFUSION_EFFECT" || (move.Effect == "CONFUSION_SIDE_EFFECT" && g.rng.Intn(10) == 0) {
		d.Combat.Confusion = 2 + g.rng.Intn(4)
		g.say("%s陷入混乱%d次行动。", d.Name(), d.Combat.Confusion)
	}
	if (move.Effect == "FLINCH_SIDE_EFFECT1" && g.rng.Intn(10) == 0) || (move.Effect == "FLINCH_SIDE_EFFECT2" && g.rng.Intn(10) < 3) {
		d.Combat.Flinch = true
	}
	status, statusChance := moveStatus(move)
	if status != "" && d.Status == "" && g.rng.Intn(100) < statusChance {
		immune := (status == "中毒" && hasType(d, 4)) || (status == "灼伤" && hasType(d, 10)) || (status == "冰冻" && hasType(d, 15))
		if !immune {
			d.Status = status
			if status == "中毒" {
				d.Combat.BadlyPoisoned = move.Key == "TOXIC"
				d.Combat.ToxicCounter = 0
			}
			d.Sleep = 2 + g.rng.Intn(3)
			g.say("%s陷入%s。%s", d.Name(), status, moveEffectDescription(move))
		} else {
			g.say("%s免疫%s。", d.Name(), status)
		}
	}
}

func hasType(m *Monster, t int) bool {
	for _, id := range EffectiveTypes(m) {
		if id == t {
			return true
		}
	}
	return false
}
