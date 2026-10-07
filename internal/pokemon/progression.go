package pokemon

import (
	"fmt"
	"strings"
)

// ExperienceAtLevel is the shared cubic growth curve used by this adventure.
// It deliberately matches existing saves, rather than changing species growth.
func ExperienceAtLevel(level int) int {
	level = max(1, min(100, level))
	return level * level * level
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
	progress.Required = ExperienceAtLevel(progress.NextLevel) - ExperienceAtLevel(m.Level)
	progress.Earned = max(0, min(progress.Required, m.Exp-ExperienceAtLevel(m.Level)))
	progress.Remaining = max(0, ExperienceAtLevel(progress.NextLevel)-m.Exp)
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
			rows = append(rows, fmt.Sprintf("进化 → %s：本地通信进化\n  不靠经验进化，伙伴菜单选择通信", name))
		case e.Level > 0:
			if m.Level >= 100 {
				rows = append(rows, fmt.Sprintf("进化 → %s：已满级，无法再通过升级触发进化", name))
				continue
			}
			// Level evolutions are checked when gaining a level, including wild
			// monsters caught at or above their species' evolution threshold.
			target := max(e.Level, m.Level+1)
			remaining := max(0, ExperienceAtLevel(target)-m.Exp)
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
