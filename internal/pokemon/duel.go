package pokemon

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Duel uses battle copies: adventure health, PP and experience are untouched.
// Both decisions are committed before either one is revealed or resolved.
type DuelAction struct {
	Kind string `json:"kind"`
	Slot int    `json:"slot"`
}
type Duel struct {
	Names   [2]string
	Teams   [2][]Monster
	Active  [2]int
	Pending [2]*DuelAction
	Turn    int
	Winner  int // -2 ongoing; -1 draw; 0/1 winner
	Log     []string
	g       *Game
}
type DuelView struct {
	Names        [2]string `json:"names"`
	Team         []Monster `json:"team"`
	Foe          Monster   `json:"foe"`
	Active       int       `json:"active"`
	FoeRemaining int       `json:"foeRemaining"`
	FoeTotal     int       `json:"foeTotal"`
	Turn         int       `json:"turn"`
	Submitted    bool      `json:"submitted"`
	Finished     bool      `json:"finished"`
	Winner       int       `json:"winner"`
	Seat         int       `json:"seat"`
	Log          []string  `json:"log"`
}

func NewDuel(names [2]string, teams [2][]Monster) (*Duel, error) {
	d := &Duel{Names: names, Winner: -2, Turn: 1, g: New("联机裁判")}
	d.g.Log = nil
	for s := range teams {
		if len(teams[s]) == 0 || len(teams[s]) > 6 {
			return nil, errors.New("双方需要1至6只队伍伙伴")
		}
		d.Teams[s] = append([]Monster(nil), teams[s]...)
		for i := range d.Teams[s] {
			m := &d.Teams[s][i]
			ResetCombat(m)
			m.HP = m.MaxHP()
			m.Status = ""
			m.Sleep = 0
			for slot, id := range m.Moves {
				if id < 0 {
					m.PP[slot] = 0
				} else {
					m.PP[slot] = Moves[id].PP
				}
			}
		}
	}
	d.g.Notice(fmt.Sprintf("训练家 %s 与 %s 发起了对战！双方押入300金币；胜者净赚300，败者净亏300。", names[0], names[1]))
	d.g.Notice(fmt.Sprintf("%s派出%s！%s派出%s！", names[0], d.Teams[0][0].Name(), names[1], d.Teams[1][0].Name()))
	d.Log = append([]string(nil), d.g.Log...)
	return d, nil
}
func (d *Duel) View(seat int) DuelView {
	foe := 1 - seat
	remaining := 0
	for _, m := range d.Teams[foe] {
		if m.HP > 0 {
			remaining++
		}
	}
	return DuelView{Names: d.Names, Team: append([]Monster(nil), d.Teams[seat]...), Foe: d.Teams[foe][d.Active[foe]], Active: d.Active[seat], FoeRemaining: remaining, FoeTotal: len(d.Teams[foe]), Turn: d.Turn, Submitted: d.Pending[seat] != nil, Finished: d.Winner != -2, Winner: d.Winner, Seat: seat, Log: append([]string(nil), d.Log...)}
}
func (d *Duel) Submit(seat int, a DuelAction) error {
	if seat < 0 || seat > 1 || d.Winner != -2 {
		return errors.New("对战已经结束")
	}
	if d.Pending[seat] != nil {
		return errors.New("本回合选择已提交，请等待对手")
	}
	p := d.Teams[seat][d.Active[seat]]
	switch a.Kind {
	case "move":
		if a.Slot < 0 || a.Slot >= 4 {
			return errors.New("技能编号为1至4")
		}
		if p.Moves[a.Slot] < 0 {
			return errors.New("该槽位尚未学习技能")
		}
		hasPP := false
		for _, pp := range p.PP {
			hasPP = hasPP || pp > 0
		}
		if hasPP && p.PP[a.Slot] <= 0 {
			return errors.New("该技能PP耗尽")
		}
		if p.Combat.DisableSlot == a.Slot+1 && p.Combat.DisableTurns > 0 {
			return errors.New("该技能暂时被封锁，请选择其他招式")
		}
	case "switch":
		if a.Slot < 0 || a.Slot >= len(d.Teams[seat]) || a.Slot == d.Active[seat] || d.Teams[seat][a.Slot].HP <= 0 {
			return errors.New("请选择另一只仍有战斗能力的伙伴")
		}
	default:
		return errors.New("对战只允许使用技能或换人")
	}
	d.Pending[seat] = &a
	if d.Pending[0] != nil && d.Pending[1] != nil {
		d.resolve()
	}
	return nil
}
func (d *Duel) resolve() {
	for seat, a := range d.Pending {
		if a.Kind == "switch" {
			ResetCombat(&d.Teams[seat][d.Active[seat]])
			d.Active[seat] = a.Slot
			ResetCombat(&d.Teams[seat][a.Slot])
			d.g.Notice(fmt.Sprintf("%s换上%s！", d.Names[seat], d.Teams[seat][a.Slot].Name()))
		}
	}
	order := [2]int{0, 1}
	p0, p1 := &d.Teams[0][d.Active[0]], &d.Teams[1][d.Active[1]]
	p0.Combat.LastDamage = 0
	p1.Combat.LastDamage = 0
	pr0, pr1 := 0, 0
	if d.Pending[0].Kind == "move" {
		pr0 = movePriority(p0, d.Pending[0].Slot)
	}
	if d.Pending[1].Kind == "move" {
		pr1 = movePriority(p1, d.Pending[1].Slot)
	}
	if pr1 > pr0 || (pr1 == pr0 && (speed(p1) > speed(p0) || (speed(p1) == speed(p0) && d.g.rng.Intn(2) == 0))) {
		order = [2]int{1, 0}
	}
	for _, seat := range order {
		if d.Pending[seat].Kind == "move" {
			d.g.attack(&d.Teams[seat][d.Active[seat]], &d.Teams[1-seat][d.Active[1-seat]], d.Pending[seat].Slot)
		}
	}
	d.g.EndResidual(&d.Teams[0][d.Active[0]], &d.Teams[1][d.Active[1]])
	for seat := 0; seat < 2; seat++ {
		p := &d.Teams[seat][d.Active[seat]]
		if p.HP == 0 {
			ResetCombat(p)
			d.g.Notice(fmt.Sprintf("%s的%s失去战斗能力！", d.Names[seat], p.Name()))
			for i, m := range d.Teams[seat] {
				if m.HP > 0 {
					d.Active[seat] = i
					ResetCombat(&d.Teams[seat][i])
					d.g.Notice(fmt.Sprintf("%s派出%s！", d.Names[seat], m.Name()))
					break
				}
			}
		}
	}
	alive := [2]bool{}
	for seat, team := range d.Teams {
		for _, m := range team {
			alive[seat] = alive[seat] || m.HP > 0
		}
	}
	if !alive[0] && !alive[1] {
		d.Winner = -1
	} else if !alive[0] {
		d.Winner = 1
	} else if !alive[1] {
		d.Winner = 0
	}
	d.Pending = [2]*DuelAction{}
	d.Turn++
	if d.Turn > 500 && d.Winner == -2 {
		d.Winner = -1
		d.g.Notice("达到500回合上限，按平局退还双方押金。")
	}
	d.Log = append([]string(nil), d.g.Log...)
}

// These operations validate all fields before network-imported adventure data
// can be used by the authoritative multiplayer coordinator.
func ValidateState(s State) error { return validate(s) }
func CloneState(s State) State {
	data, _ := json.Marshal(s)
	var out State
	_ = json.Unmarshal(data, &out)
	return out
}
func (g *Game) ReceivedTrade(index int, incoming Monster) error {
	if g.Battle != nil || g.Busy() || index < 0 || index >= len(g.Party) {
		return errors.New("交换需要在非战斗状态选择有效伙伴")
	}
	g.Party[index] = incoming
	g.Caught[incoming.Species] = true
	g.Seen[incoming.Species] = true
	g.Notice("交换完成，收到" + incoming.Name() + "！")
	for _, e := range Dex[incoming.Species].Evolves {
		if e.Trade {
			g.evolveMonster(&g.Party[index], e.Species)
			break
		}
	}
	return nil
}
