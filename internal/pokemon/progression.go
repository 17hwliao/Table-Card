package pokemon

import (
	"fmt"
	"strings"
)

// ExperienceAtLevel remains the legacy save curve. New monsters use their
// species' documented Gen I growth curve via ExperienceForSpecies.
func ExperienceAtLevel(level int) int {
	level = max(1, min(100, level))
	return level * level * level
}

func ExperienceForSpecies(species, level int) int {
	level = max(1, min(100, level))
	if level == 1 {
		return 0
	}
	n := level * level * level
	switch Dex[species].Growth {
	case "FAST":
		return 4 * n / 5
	case "SLOW":
		return 5 * n / 4
	case "MEDIUM_SLOW":
		return max(0, 6*n/5-15*level*level+100*level-140)
	default:
		return n
	}
}

// MigrateMonsterExperience retains level and fraction earned at that level.
// Call only once when importing the legacy v1/v2 cubic-curve save.
func MigrateMonsterExperience(m *Monster) {
	oldStart := ExperienceAtLevel(m.Level)
	newStart := ExperienceForSpecies(m.Species, m.Level)
	if m.Level >= 100 {
		m.Exp = newStart
		return
	}
	oldRequired := ExperienceAtLevel(m.Level+1) - oldStart
	earned := max(0, min(oldRequired-1, m.Exp-oldStart))
	newRequired := ExperienceForSpecies(m.Species, m.Level+1) - newStart
	m.Exp = newStart + earned*newRequired/max(1, oldRequired)
}

type ExperienceProgress struct {
	Total, Earned, Required, Remaining, NextLevel int
	MaxLevel                                      bool
}

func (m Monster) ExperienceProgress() ExperienceProgress {
	progress := ExperienceProgress{Total: m.Exp, MaxLevel: m.Level >= 100}
	if progress.MaxLevel {
		return progress
	}
	progress.NextLevel = m.Level + 1
	progress.Required = ExperienceForSpecies(m.Species, progress.NextLevel) - ExperienceForSpecies(m.Species, m.Level)
	progress.Earned = max(0, min(progress.Required, m.Exp-ExperienceForSpecies(m.Species, m.Level)))
	progress.Remaining = max(0, ExperienceForSpecies(m.Species, progress.NextLevel)-m.Exp)
	return progress
}

func (m Monster) ExperienceSummary() string {
	p := m.ExperienceProgress()
	if p.MaxLevel {
		return fmt.Sprintf("累计经验 %d · Lv100 已满级", p.Total)
	}
	return fmt.Sprintf("累计经验 %d\n本级经验 %d/%d\n升到 Lv%d 还需 %d 经验", p.Total, p.Earned, p.Required, p.NextLevel, p.Remaining)
}

func (m Monster) EvolutionSummary() string {
	evolutions := Dex[m.Species].Evolves
	if len(evolutions) == 0 {
		return "进化：已是最终阶段 / 无后续进化"
	}
	rows := make([]string, 0, len(evolutions))
	for _, e := range evolutions {
		name := Dex[e.Species].Name
		switch {
		case e.Item != "":
			rows = append(rows, fmt.Sprintf("进化 → %s：使用%s\n  不靠经验进化，背包选择进化石", name, e.Item))
		case e.Trade:
			rows = append(rows, fmt.Sprintf("进化 → %s：与在线玩家完成交换后进化\n  不靠经验进化，伙伴菜单选择交换", name))
		case e.Level > 0:
			if m.Level >= 100 {
				rows = append(rows, fmt.Sprintf("进化 → %s：已满级，无法再通过升级触发进化", name))
				continue
			}
			// Level evolutions are checked when gaining a level, including wild
			// monsters caught at or above their species' evolution threshold.
			target := max(e.Level, m.Level+1)
			remaining := max(0, ExperienceForSpecies(m.Species, target)-m.Exp)
			condition := fmt.Sprintf("达到 Lv%d", e.Level)
			if m.Level >= e.Level {
				condition = fmt.Sprintf("等级已达标，下次升级至 Lv%d 时进化", target)
			}
			rows = append(rows, fmt.Sprintf("进化 → %s：%s\n  还差 %d 级 / %d 经验", name, condition, target-m.Level, remaining))
		}
	}
	return strings.Join(rows, "\n")
}

// Remaining includes the current opponent and all living reserves, so the
// count also stays correct while reading a resumed battle or during capture.
func (b *Battle) OpponentCounts() (total, remaining, defeated int) {
	if b == nil {
		return 0, 0, 0
	}
	total = len(b.Foes)
	for _, foe := range b.Foes {
		if foe.HP > 0 {
			remaining++
		}
	}
	return total, remaining, total - remaining
}

func (g *Game) OpponentInfo() string {
	b := g.Battle
	if b == nil {
		return "当前没有对战中的对手"
	}
	total, remaining, defeated := b.OpponentCounts()
	return fmt.Sprintf("对手队伍 %d 只 · 剩余 %d 只\n已击败 %d 只 · 后备 %d 只\n当前出场 %d/%d", total, remaining, defeated, max(0, remaining-1), b.Enemy+1, total)
}
