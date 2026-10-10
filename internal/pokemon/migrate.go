package pokemon

import "errors"

func NormalizeData(s *State) error { return MigrateState(s) }

func ValidateCombat(c CombatState) bool {
	if c.ToxicCounter < 0 || c.ToxicCounter > 15 {
		return false
	}
	if c.TransformedSpecies < 0 || c.TransformedSpecies > 151 || c.ConvertedCount < 0 || c.ConvertedCount > 2 {
		return false
	}
	for i := 0; i < c.ConvertedCount; i++ {
		if TypeNames[c.ConvertedTypes[i]] == "" {
			return false
		}
	}
	if c.TransformedSpecies > 0 {
		for i, id := range c.OriginalMoves {
			if id < -1 || id >= len(Moves) || c.OriginalPP[i] < 0 || c.OriginalPP[i] > 40 {
				return false
			}
		}
		for _, stat := range c.CopiedStats {
			if stat < 1 || stat > 1000 {
				return false
			}
		}
	}
	for _, s := range c.Stages {
		if s < -6 || s > 6 {
			return false
		}
	}
	return c.Confusion >= 0 && c.Confusion <= 5 && c.Recharge >= 0 && c.Recharge <= 1 && c.TrapTurns >= 0 && c.TrapTurns <= 5 && c.TrapDamage >= 0 && c.TrapDamage <= 10000 && c.ChargeMove >= 0 && c.ChargeMove <= len(Moves) && c.DisableSlot >= 0 && c.DisableSlot <= 4 && c.DisableTurns >= 0 && c.DisableTurns <= 7 && c.LastMove >= 0 && c.LastMove <= len(Moves) && c.LastDamage >= 0 && c.LastDamage <= 100000 && c.Substitute >= 0 && c.Substitute <= 10000 && c.BideTurns >= 0 && c.BideTurns <= 2 && c.BideDamage >= 0 && c.BideDamage <= 100000 && c.RepeatMove >= 0 && c.RepeatMove <= 4 && c.RepeatTurns >= 0 && c.RepeatTurns <= 3
}

// MigrateState validates indices before touching factual species/move tables.
// Legacy HP is clamped because Gen I base stats replaced modern values. This
// does not heal fainted monsters or reset PP on already legal moves.
func MigrateState(s *State) error {
	if s == nil || s.Version < 1 || s.Version > 3 {
		return errors.New("不支持的宝可梦存档版本")
	}
	if len(s.Party) > 6 || len(s.Box) > 600 || (s.Battle != nil && len(s.Battle.Foes) > 6) {
		return errors.New("宝可梦队伍数量不合法")
	}
	sets := [][]Monster{s.Party, s.Box}
	if s.Battle != nil {
		sets = append(sets, s.Battle.Foes)
	}
	for _, set := range sets {
		for _, m := range set {
			if m.Species < 1 || m.Species > 151 || m.Level < 1 || m.Level > 100 || m.Exp < 0 || m.Exp > 1250000 || m.HP < 0 || m.HP > 10000 {
				return errors.New("宝可梦基础数据不合法")
			}
			for i, id := range m.Moves {
				if id < -1 || id >= len(Moves) || m.PP[i] < 0 || m.PP[i] > 40 {
					return errors.New("宝可梦招式数据不合法")
				}
			}
		}
	}
	old := s.Version < 3
	for _, set := range sets {
		for i := range set {
			m := &set[i]
			if old {
				MigrateMonsterExperience(m)
				ResetCombat(m)
			}
			m.HP = min(m.HP, m.MaxHP())
			SanitizeMonster(m)
		}
	}
	s.Version = 3
	return validate(*s)
}
