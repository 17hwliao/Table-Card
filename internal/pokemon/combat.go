package pokemon

import (
	"fmt"
	"strings"
)

// CombatState contains battle-only effects. Switching clears these effects;
// persistent HP/status remain on the monster. It is serializable for resumes.
type CombatState struct {
	Stages                                                      [7]int // attack, defense, special, speed, accuracy, evasion
	Confusion, Recharge, TrapTurns, TrapDamage                  int
	Flinch, Seeded, Reflect, LightScreen, Mist, Focus           bool
	ChargeMove, DisableSlot, DisableTurns, LastMove, LastDamage int
	Substitute, BideTurns, BideDamage, RepeatMove, RepeatTurns  int
	TransformedSpecies                                          int
	OriginalMoves, OriginalPP                                   [4]int
	CopiedStats                                                 [4]int // raw attack, defense, special, speed; HP is not copied
	ConvertedTypes                                              [2]int
	ConvertedCount                                              int
	ToxicCounter                                                int
	BadlyPoisoned                                               bool
}

func ResetCombat(m *Monster) {
	if m.Combat.TransformedSpecies > 0 {
		m.Moves = m.Combat.OriginalMoves
		m.PP = m.Combat.OriginalPP
	}
	m.Combat = CombatState{}
}
func EffectiveTypes(m *Monster) []int {
	if m.Combat.ConvertedCount > 0 && m.Combat.ConvertedCount <= 2 {
		return m.Combat.ConvertedTypes[:m.Combat.ConvertedCount]
	}
	if m.Combat.TransformedSpecies > 0 && m.Combat.TransformedSpecies <= 151 {
		return Dex[m.Combat.TransformedSpecies].Types
	}
	return Dex[m.Species].Types
}
func rawCombatStat(m *Monster, base, stage int) int {
	if m.Combat.TransformedSpecies > 0 && stage >= 0 && stage < 4 {
		return max(1, m.Combat.CopiedStats[stage])
	}
	return m.stat(base)
}
func combatStat(m *Monster, base, stage int) int {
	s := m.Combat.Stages[stage]
	n := rawCombatStat(m, base, stage)
	if s >= 0 {
		return max(1, n*(2+s)/2)
	}
	return max(1, n*2/(2-s))
}
func accuracyFactor(stage int) float64 {
	if stage >= 0 {
		return float64(3+stage) / 3
	}
	return 3 / float64(3-stage)
}
func movePriority(m *Monster, slot int) int {
	if slot < 0 || slot >= 4 || m.Moves[slot] < 0 {
		return 0
	}
	switch Moves[m.Moves[slot]].Key {
	case "QUICK_ATTACK":
		return 1
	case "COUNTER":
		return -1
	}
	return 0
}

func stageEffect(effect string) (index, amount, chance int, self bool) {
	names := []string{"ATTACK", "DEFENSE", "SPECIAL", "SPEED", "ACCURACY", "EVASION"}
	for i, n := range names {
		switch effect {
		case n + "_UP1_EFFECT":
			return i, 1, 100, true
		case n + "_UP2_EFFECT":
			return i, 2, 100, true
		case n + "_DOWN1_EFFECT":
			return i, -1, 100, false
		case n + "_DOWN2_EFFECT":
			return i, -2, 100, false
		case n + "_DOWN_SIDE_EFFECT":
			return i, -1, 33, false
		}
	}
	return 0, 0, 0, false
}

func moveStatus(move Move) (string, int) {
	e := move.Effect
	switch e {
	case "SLEEP_EFFECT":
		return "睡眠", 100
	case "POISON_EFFECT":
		return "中毒", 100
	case "PARALYZE_EFFECT":
		return "麻痹", 100
	}
	for _, v := range []struct{ key, status string }{{"BURN", "灼伤"}, {"FREEZE", "冰冻"}, {"PARALYZE", "麻痹"}, {"POISON", "中毒"}} {
		if strings.HasPrefix(e, v.key+"_SIDE_EFFECT") {
			chance := 10
			if strings.HasSuffix(e, "2") {
				chance = 30
			}
			if v.key == "POISON" && strings.HasSuffix(e, "2") {
				chance = 40
			}
			if v.key == "POISON" && strings.HasSuffix(e, "1") {
				chance = 20
			}
			return v.status, chance
		}
	}
	if e == "TWINEEDLE_EFFECT" {
		return "中毒", 20
	}
	return "", 0
}

func (g *Game) changeStage(m *Monster, index, amount int) {
	names := []string{"攻击", "防御", "特殊", "速度", "命中", "闪避"}
	old := m.Combat.Stages[index]
	m.Combat.Stages[index] = max(-6, min(6, old+amount))
	delta := m.Combat.Stages[index] - old
	if delta == 0 {
		g.say("%s的%s已到变化上限。", m.Name(), names[index])
		return
	}
	direction := "提高"
	if delta < 0 {
		direction = "降低"
		delta = -delta
	}
	g.say("%s的%s%s%d级（现为 %+d）。", m.Name(), names[index], direction, delta, m.Combat.Stages[index])
}

func MoveDescription(id int) string {
	if id < 0 || id >= len(Moves) || Moves[id].Name == "" {
		return "未学习招式"
	}
	m := Moves[id]
	category := "物理"
	if m.Type >= 10 {
		category = "特殊"
	}
	if m.Power == 0 {
		category = "变化"
	}
	text := fmt.Sprintf("%s · %s · 威力 %d · 命中 %d%% · 最大 PP %d", TypeNames[m.Type], category, m.Power, m.Accuracy, m.PP)
	effect := moveEffectDescription(m)
	if effect != "" {
		text += "\n" + effect
	}
	return text
}

func moveEffectDescription(m Move) string {
	if m.Key == "TOXIC" {
		return "令对手剧毒：每回合损失最大HP的1/16、2/16、3/16…，最高15/16；换人后退回普通中毒。"
	}
	if i, a, c, self := stageEffect(m.Effect); c > 0 {
		target := "对手"
		if self {
			target = "自身"
		}
		dir := "提高"
		if a < 0 {
			dir = "降低"
			a = -a
		}
		return fmt.Sprintf("%d%%概率%s%s%s%d级；变化范围 -6 到 +6，交换出场后重置。", c, target, []string{"攻击", "防御", "特殊", "速度", "命中", "闪避"}[i], dir, a)
	}
	if status, chance := moveStatus(m); status != "" {
		extra := map[string]string{"中毒": "每回合损失最大 HP 的1/16。", "灼伤": "物理攻击减半，每回合损失最大 HP 的1/16。", "麻痹": "速度降至1/4，每次行动有25%概率失败。", "睡眠": "持续2–4次行动后醒来。", "冰冻": "无法行动；本作每次行动有20%概率解冻。"}[status]
		return fmt.Sprintf("%d%%概率令对手%s；%s", chance, status, extra)
	}
	switch m.Key {
	case "DRAGON_RAGE":
		return "固定造成40点伤害，不按面板威力计算。"
	case "SONICBOOM":
		return "固定造成20点伤害。"
	case "SEISMIC_TOSS", "NIGHT_SHADE":
		return "固定造成与使用者等级相同的伤害。"
	case "PSYWAVE":
		return "随机造成1至使用者等级×1.5的伤害。"
	case "SUPER_FANG":
		return "削去对手当前 HP 的一半，至少1点。"
	case "QUICK_ATTACK":
		return "先制优先级+1，通常先于一般招式。"
	case "REST":
		return "恢复全部 HP 并清除原状态，自身睡眠2次行动。"
	case "COUNTER":
		return "后手返还本回合收到的普通/格斗物理伤害的2倍。"
	}
	switch m.Effect {
	case "NO_ADDITIONAL_EFFECT":
		if m.Key == "SLASH" || m.Key == "RAZOR_LEAF" || m.Key == "CRABHAMMER" || m.Key == "KARATE_CHOP" {
			return "高要害率；本作使用1/4要害概率。"
		}
		return "普通伤害；属性克制、本系加成、能力变化与随机浮动参与结算。"
	case "ORIGINAL_STORY_SKILL":
		return "本作原创剧情专属技，不属于红/蓝原版招式。"
	case "DRAIN_HP_EFFECT":
		return "造成伤害后恢复实际伤害的1/2 HP。"
	case "DREAM_EATER_EFFECT":
		return "仅对睡眠中的对手生效；吸取伤害的1/2 HP。"
	case "RECOIL_EFFECT":
		return "自身受到实际伤害的1/4反作用伤害。"
	case "ATTACK_TWICE_EFFECT", "TWINEEDLE_EFFECT":
		return "连续攻击2次，每次单独结算伤害。"
	case "TWO_TO_FIVE_ATTACKS_EFFECT":
		return "连续攻击2–5次；2/3/4/5次概率为37.5/37.5/12.5/12.5%。"
	case "CONFUSION_EFFECT":
		return "令对手混乱2–5次行动；50%概率攻击自身。"
	case "CONFUSION_SIDE_EFFECT":
		return "10%概率令对手混乱2–5次行动。"
	case "FLINCH_SIDE_EFFECT1":
		return "10%概率畏缩；只有尚未行动的对手会失去本次行动。"
	case "FLINCH_SIDE_EFFECT2":
		return "30%概率畏缩；只有尚未行动的对手会失去本次行动。"
	case "HEAL_EFFECT":
		return "恢复自身最大 HP 的1/2。"
	case "LEECH_SEED_EFFECT":
		return "每回合吸取对手最大 HP 的1/16；草属性免疫，交换出场解除。"
	case "REFLECT_EFFECT":
		return "物理防御加倍；交换出场后解除。"
	case "LIGHT_SCREEN_EFFECT":
		return "特殊防御加倍；交换出场后解除。"
	case "MIST_EFFECT":
		return "防止对手主动降低自身能力；交换出场后解除。"
	case "HAZE_EFFECT":
		return "清除双方能力变化、混乱与屏障等战斗状态。"
	case "FOCUS_ENERGY_EFFECT":
		return "提高本作要害率至1/4，不复现红蓝原版程序错误。"
	case "CHARGE_EFFECT", "FLY_EFFECT":
		return "第一回合蓄力，第二回合攻击；本作蓄力时仍可被命中。"
	case "HYPER_BEAM_EFFECT":
		return "攻击后需要休息一回合（击倒对手时无需休息）。"
	case "EXPLODE_EFFECT":
		return "结算伤害时对手防御减半；使用者随后失去战斗能力。"
	case "OHKO_EFFECT":
		return "命中后令对手失去战斗能力；自身等级较低或速度较低时失败。"
	case "TRAPPING_EFFECT":
		return "造成伤害并束缚对手2–5次行动，期间无法行动；本作简化束缚与换人。"
	case "THRASH_PETAL_DANCE_EFFECT":
		return "锁定连续使用2–3回合，然后自身混乱。"
	case "SWIFT_EFFECT":
		return "必中，忽略命中和闪避变化。"
	case "SPLASH_EFFECT":
		return "没有战斗效果。"
	case "SWITCH_AND_TELEPORT_EFFECT":
		return "野生对战中结束遭遇；训练家或玩家对战中失败。"
	case "JUMP_KICK_EFFECT":
		return "未命中时受到1点反作用伤害（第一世代规则）。"
	case "DISABLE_EFFECT":
		return "暂时封锁对手最近使用的招式1–7次行动。"
	case "SUBSTITUTE_EFFECT":
		return "消耗自身最大 HP 的1/4制造替身；替身承受伤害并阻挡状态。"
	case "METRONOME_EFFECT":
		return "随机使用一项第一世代招式（排除挥指本身）。"
	case "MIRROR_MOVE_EFFECT", "MIMIC_EFFECT":
		return "本次行动复制对手最近使用的招式；模仿在本作不永久修改招式槽。"
	case "CONVERSION_EFFECT":
		return "复制对手当前属性；持续到换人或对战结束，原物种保持不变。"
	case "TRANSFORM_EFFECT":
		return "复制对手属性、非HP能力、能力变化和招式；每招5PP。自身HP、等级、经验和永久物种不变，换人/结束后恢复。"
	case "BIDE_EFFECT":
		return "忍耐2次行动，随后返还期间实际承受伤害的2倍。"
	case "RAGE_EFFECT":
		return "造成伤害；之后受到攻击时自身攻击提高1级。"
	case "PAY_DAY_EFFECT":
		return "造成普通伤害；本作不额外生成招式金币。"
	}
	return "本作尚未实现该附加效果；招式基础数据为第一世代原始数值。"
}

// SanitizeMonster repairs template move pollution in legacy saves and retains
// legitimate moves inherited from earlier evolutionary forms.
func SanitizeMonster(m *Monster) {
	if m.Species < 1 || m.Species > 151 {
		return
	}
	if m.Combat.TransformedSpecies > 0 {
		original := *m
		ResetCombat(&original)
		SanitizeMonster(&original)
		m.Combat.OriginalMoves = original.Moves
		m.Combat.OriginalPP = original.PP
		for i, id := range m.Moves {
			if id < 0 || id >= len(Moves) {
				m.Moves[i] = -1
				m.PP[i] = 0
			} else {
				m.PP[i] = max(0, min(5, m.PP[i]))
			}
		}
		return
	}
	allowed := map[int]bool{}
	var add func(int)
	add = func(id int) {
		for _, l := range Dex[id].Learnset {
			if l.Level <= m.Level {
				allowed[l.Move] = true
			}
		}
		for parent := 1; parent <= 151; parent++ {
			for _, e := range Dex[parent].Evolves {
				if e.Species == id {
					add(parent)
				}
			}
		}
	}
	add(m.Species)
	if m.Signature {
		allowed[signatureMove(m.Species)] = true
	}
	seen := map[int]bool{}
	next := [4]int{-1, -1, -1, -1}
	pp := [4]int{}
	n := 0
	for i, id := range m.Moves {
		if allowed[id] && !seen[id] {
			next[n] = id
			pp[n] = max(0, min(Moves[id].PP, m.PP[i]))
			seen[id] = true
			n++
		}
	}
	for _, id := range moveSet(m.Species, m.Level) {
		if n >= 4 {
			break
		}
		if id >= 0 && !seen[id] {
			next[n] = id
			pp[n] = Moves[id].PP
			seen[id] = true
			n++
		}
	}
	if m.Signature && !seen[signatureMove(m.Species)] {
		next[3] = signatureMove(m.Species)
		pp[3] = Moves[next[3]].PP
	}
	m.Moves = next
	m.PP = pp
}

// EndResidual is shared by NPC and server-owned player battles.
func (g *Game) EndResidual(a, b *Monster) {
	for _, pair := range [][2]*Monster{{a, b}, {b, a}} {
		m, other := pair[0], pair[1]
		if m.HP > 0 && (m.Status == "中毒" || m.Status == "灼伤") {
			amount := max(1, m.MaxHP()/16)
			if m.Status == "中毒" && m.Combat.BadlyPoisoned {
				m.Combat.ToxicCounter = min(15, m.Combat.ToxicCounter+1)
				amount *= m.Combat.ToxicCounter
			}
			loss := min(m.HP, amount)
			m.HP -= loss
			g.say("%s受到%s伤害%d，HP %d/%d。", m.Name(), m.Status, loss, m.HP, m.MaxHP())
		}
		if m.HP > 0 && m.Combat.Seeded {
			loss := min(m.HP, max(1, m.MaxHP()/16))
			m.HP -= loss
			heal := 0
			if other.HP > 0 {
				heal = min(loss, other.MaxHP()-other.HP)
				other.HP += heal
			}
			g.say("寄生种子：%s失去%d HP，%s恢复%d HP。", m.Name(), loss, other.Name(), heal)
		}
		m.Combat.Flinch = false
	}
}
