// Package garden implements a two-player lane-defense relay. Its simulation is
// independent of terminal drawing and uses integer positions and fixed steps.
package garden

import (
	"encoding/json"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/17hwliao/table-card-independent/internal/table"
)

const Rows, Cols = 5, 9
const Cell = 1000
const Step = 100 * time.Millisecond
const MaxZombies = 120 // Global cap; transfers never require dropping a zombie.
const MaxShots = 256   // Per lawn.

type PlantKind int

const (
	Sunflower PlantKind = iota + 1
	Peashooter
	Wallnut
	Cherry
	Snowpea
	Pepper
	Repeater
	Chomper
	Mine
	Squash
)

type PlantDef struct {
	Name               string
	Cost, HP, Cooldown int
	Hint               string
}

var Plants = [...]PlantDef{
	{},
	{"向日葵", 50, 160, 50, "每10秒产25阳光"},
	{"豌豆射手", 100, 180, 50, "每1.4秒向本行射击"},
	{"坚果", 50, 1200, 140, "阻挡并吸收啃食伤害"},
	{"樱桃炸弹", 150, 120, 300, "0.8秒后炸毁周围3×3敌人"},
	{"寒冰射手", 175, 180, 70, "命中减速5秒"},
	{"火爆辣椒", 125, 120, 300, "0.8秒后清除本行敌人"},
	{"双发射手", 200, 180, 70, "每次发射两枚弹丸"},
	{"大嘴花", 150, 240, 70, "吞噬近身敌人，消化12秒"},
	{"土豆雷", 25, 100, 180, "埋置8秒后武装，接触触发爆炸"},
	{"窝瓜", 50, 160, 180, "发现近身敌人后压毁，消耗自身"},
}

type ZombieKind int

const (
	Normal ZombieKind = iota
	Cone
	Bucket
	Dancer
	Backup
	Pole
	Newspaper
	Football
	Door
)

type ZombieDef struct {
	Name                   string
	HP, Armor, Speed, Bite int
}

var Zombies = [...]ZombieDef{
	{"普通", 180, 0, 22, 12}, {"路障", 180, 180, 22, 12},
	{"铁桶", 180, 480, 18, 12}, {"舞王", 240, 0, 22, 12},
	{"伴舞", 140, 0, 28, 10}, {"撑杆", 240, 0, 38, 12},
	{"报纸", 180, 140, 16, 12}, {"橄榄球", 260, 480, 40, 16},
	{"铁栅门", 180, 400, 20, 12},
}

type Plant struct {
	Kind PlantKind `json:"kind"`
	HP   int       `json:"hp"`
	Wait int       `json:"wait"` // Shoot, produce, fuse, arm or digestion timer in steps.
}
type Zombie struct {
	ID         uint64     `json:"id"`
	Kind       ZombieKind `json:"kind"`
	Row        int        `json:"row"`
	X          int        `json:"x"`
	HP         int        `json:"hp"`
	Armor      int        `json:"armor"`
	Slow       int        `json:"slow"`
	BiteWait   int        `json:"biteWait"`
	SummonWait int        `json:"summonWait"`
	Jumped     bool       `json:"jumped"`
	Summoned   bool       `json:"summoned"`
	Hops       int        `json:"hops"`
}
type Shot struct {
	Row  int  `json:"row"`
	X    int  `json:"x"`
	Slow bool `json:"slow"`
}
type Effect struct {
	Row  int    `json:"row"`
	Col  int    `json:"col"`
	Kind string `json:"kind"`
	TTL  int    `json:"ttl"`
}
type Lawn struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Grid     [Rows][Cols]Plant `json:"grid"`
	Zombies  []Zombie          `json:"zombies"`
	Shots    []Shot            `json:"shots"`
	Effects  []Effect          `json:"effects"`
	Cooldown [11]int           `json:"cooldown"`
	Sun      int               `json:"sun"`
	Kills    int               `json:"kills"`
	Received int               `json:"received"`
	Actions  int               `json:"actions"`
	Alive    bool              `json:"alive"`
}
type Snapshot struct {
	Players  []Lawn   `json:"players"`
	Started  bool     `json:"started"`
	StartAt  int64    `json:"startAt"`
	Elapsed  int      `json:"elapsed"`
	Wave     int      `json:"wave"`
	NextWave int      `json:"nextWave"`
	Finished bool     `json:"finished"`
	Winner   string   `json:"winner"`
	Draw     bool     `json:"draw"`
	Events   []string `json:"events"`
}
type Action struct {
	Type  string    `json:"type"`
	Row   int       `json:"row"`
	Col   int       `json:"col"`
	Plant PlantKind `json:"plant"`
}
type Engine struct {
	mu                    sync.RWMutex
	lawns                 [2]Lawn
	startAt, last         time.Time
	rng                   *rand.Rand
	steps, wave, nextWave int
	nextID                uint64
	finished              bool
	winner                string
	draw                  bool
	events                []string
	lastBot               [2]int
}

func NewEngine(players []table.Player) (*Engine, error) {
	if len(players) != 2 || players[0].ID == "" || players[1].ID == "" || players[0].ID == players[1].ID {
		return nil, errors.New("草坪接力只支持两位不同玩家")
	}
	start := time.Now().Add(3 * time.Second)
	e := &Engine{startAt: start, last: start, nextWave: 150, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
	for i, p := range players {
		e.lawns[i] = Lawn{ID: p.ID, Name: p.Name, Sun: 150, Alive: true, Zombies: make([]Zombie, 0, MaxZombies), Shots: make([]Shot, 0, MaxShots)}
	}
	e.note("开局150阳光；15秒后第一波。击败的僵尸满血转移到对方同一行。")
	return e, nil
}
func (*Engine) Mode() table.Mode    { return table.GardenMode }
func (e *Engine) Finished() bool    { e.mu.RLock(); defer e.mu.RUnlock(); return e.finished }
func (e *Engine) View(_ string) any { e.mu.RLock(); defer e.mu.RUnlock(); return e.snapshot() }
func (e *Engine) snapshot() Snapshot {
	v := Snapshot{Players: make([]Lawn, 2), Started: e.steps > 0, StartAt: e.startAt.UnixMilli(), Elapsed: e.steps / 10, Wave: e.wave, NextWave: max(0, e.nextWave-e.steps+9) / 10, Finished: e.finished, Winner: e.winner, Draw: e.draw, Events: append([]string(nil), e.events...)}
	for i, l := range e.lawns {
		v.Players[i] = l
		v.Players[i].Zombies = append([]Zombie(nil), l.Zombies...)
		v.Players[i].Shots = append([]Shot(nil), l.Shots...)
		v.Players[i].Effects = append([]Effect(nil), l.Effects...)
	}
	return v
}
func (e *Engine) note(s string) {
	e.events = append(e.events, s)
	if len(e.events) > 6 {
		copy(e.events, e.events[len(e.events)-6:])
		e.events = e.events[:6]
	}
}
func (e *Engine) Apply(id string, raw json.RawMessage) (any, error) {
	var a Action
	if json.Unmarshal(raw, &a) != nil {
		return nil, errors.New("无效的草坪操作")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	seat := -1
	for i := range e.lawns {
		if e.lawns[i].ID == id {
			seat = i
		}
	}
	if seat < 0 {
		return nil, errors.New("玩家不在本局中")
	}
	if e.finished {
		return nil, errors.New("对局已结束；F2再开一局")
	}
	if time.Now().Before(e.startAt) {
		return nil, errors.New("等待开局倒计时")
	}
	if err := e.apply(seat, a); err != nil {
		return nil, err
	}
	return e.snapshot(), nil
}
func (e *Engine) apply(seat int, a Action) error {
	if a.Row < 0 || a.Row >= Rows || a.Col < 0 || a.Col >= Cols {
		return errors.New("请选择草坪内的位置")
	}
	l := &e.lawns[seat]
	p := &l.Grid[a.Row][a.Col]
	switch a.Type {
	case "plant":
		if a.Plant < Sunflower || a.Plant > Squash {
			return errors.New("无效的植物卡片")
		}
		d := Plants[a.Plant]
		if p.Kind != 0 {
			return errors.New("此格已有植物；X切换铲子后铲除")
		}
		if l.Cooldown[a.Plant] > 0 {
			return errors.New("卡片尚在冷却")
		}
		if l.Sun < d.Cost {
			return errors.New("阳光不足")
		}
		wait := 0
		switch a.Plant {
		case Sunflower:
			wait = 100
		case Cherry, Pepper:
			wait = 8
		case Mine:
			wait = 80
		}
		*p = Plant{Kind: a.Plant, HP: d.HP, Wait: wait}
		l.Sun -= d.Cost
		l.Cooldown[a.Plant] = d.Cooldown
	case "shovel":
		if p.Kind == 0 {
			return errors.New("此格没有植物")
		}
		*p = Plant{}
	default:
		return errors.New("不支持的草坪操作")
	}
	l.Actions++
	return nil
}

// Tick implements the existing server clock interface. At most ten steps are
// caught up after a stall; the remaining wall-clock gap is deliberately dropped.
func (e *Engine) Tick(now time.Time, bots []string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.finished || now.Before(e.startAt) || now.Before(e.last.Add(Step)) {
		return false
	}
	count := min(10, int(now.Sub(e.last)/Step))
	for n := 0; n < count && !e.finished; n++ {
		e.steps++
		for i := range e.lawns {
			for _, id := range bots {
				if id == e.lawns[i].ID && e.steps-e.lastBot[i] >= 10 {
					e.bot(i)
					e.lastBot[i] = e.steps
					break
				}
			}
		}
		e.step()
	}
	e.last = e.last.Add(time.Duration(count) * Step)
	if now.Sub(e.last) >= Step {
		e.last = now
	}
	return true
}
func (e *Engine) totalZombies() int { return len(e.lawns[0].Zombies) + len(e.lawns[1].Zombies) }
func (e *Engine) makeZombie(kind ZombieKind, row, hops int) Zombie {
	e.nextID++
	d := Zombies[kind]
	return Zombie{ID: e.nextID, Kind: kind, Row: row, X: Cols * Cell, HP: d.HP, Armor: d.Armor, SummonWait: 90, Hops: hops}
}
func (e *Engine) effect(l *Lawn, row, col int, kind string) {
	if len(l.Effects) >= 32 {
		return
	}
	l.Effects = append(l.Effects, Effect{row, max(0, min(Cols-1, col)), kind, 5})
}
func (e *Engine) step() {
	if e.steps >= e.nextWave {
		if e.totalZombies()+2 <= MaxZombies {
			e.wave++
			kind := Normal
			choices := []ZombieKind{Normal, Normal, Cone, Pole, Newspaper, Bucket, Door, Dancer, Football}
			if e.wave > 2 {
				kind = choices[e.rng.Intn(min(len(choices), 2+e.wave/2))]
			}
			row := e.rng.Intn(Rows)
			for i := range e.lawns {
				e.lawns[i].Zombies = append(e.lawns[i].Zombies, e.makeZombie(kind, row, 0))
			}
		}
		e.nextWave = e.steps + max(30, 160-e.steps/30)
	}
	// Both boards finish combat before any transfer becomes visible. Even an
	// explosion cannot trigger a same-step relay chain on the opponent's lawn.
	var transfers [2][]Zombie
	for i := range e.lawns {
		l := &e.lawns[i]
		if e.steps%50 == 0 {
			l.Sun = min(9999, l.Sun+25)
		}
		for k := range l.Cooldown {
			l.Cooldown[k] = max(0, l.Cooldown[k]-1)
		}
		fx := l.Effects[:0]
		for _, f := range l.Effects {
			f.TTL--
			if f.TTL > 0 {
				fx = append(fx, f)
			}
		}
		l.Effects = fx
		e.plants(l)
		e.projectiles(l)
		e.zombies(l)
		alive := l.Zombies[:0]
		for _, z := range l.Zombies {
			if z.HP <= 0 {
				l.Kills++
				e.effect(l, z.Row, z.X/Cell, "hit")
				// Reset HP/armor and transient behavior, retain original species,
				// lane and relay count. Armor break never changes the species ID.
				revived := e.makeZombie(z.Kind, z.Row, z.Hops+1)
				transfers[1-i] = append(transfers[1-i], revived)
			} else {
				alive = append(alive, z)
			}
		}
		l.Zombies = alive
	}
	for i := range e.lawns {
		l := &e.lawns[i]
		l.Received += len(transfers[i])
		l.Zombies = append(l.Zombies, transfers[i]...)
	}
	if !e.lawns[0].Alive || !e.lawns[1].Alive {
		e.finished = true
		e.draw = !e.lawns[0].Alive && !e.lawns[1].Alive
		if !e.draw {
			for _, l := range e.lawns {
				if l.Alive {
					e.winner = l.ID
					e.note(l.Name + " 守住了脑子，获胜！")
				}
			}
		} else {
			e.note("双方在同一时钟周期被吃掉脑子，平局。")
		}
	}
}

func nearest(l *Lawn, row, x, reach int) int {
	index := -1
	distance := reach + 1
	for i, z := range l.Zombies {
		if z.HP > 0 && z.Row == row && z.X >= x-250 && z.X-x <= reach && z.X-x < distance {
			index = i
			distance = z.X - x
		}
	}
	return index
}
func damage(z *Zombie, n int) {
	absorbed := min(z.Armor, n)
	z.Armor -= absorbed
	z.HP -= n - absorbed
}
func (e *Engine) blast(l *Lawn, row, col, radius int, wholeRow bool) {
	for i := range l.Zombies {
		z := &l.Zombies[i]
		if z.HP > 0 && ((wholeRow && z.Row == row) || (!wholeRow && abs(z.Row-row) <= radius && abs(z.X-col*Cell) <= radius*Cell+500)) {
			z.HP = 0
		}
	}
	e.effect(l, row, col, "blast")
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func (e *Engine) plants(l *Lawn) {
	for row := 0; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			p := &l.Grid[row][col]
			if p.Kind == 0 || p.HP <= 0 {
				continue
			}
			p.Wait = max(0, p.Wait-1)
			if p.Wait > 0 {
				continue
			}
			switch p.Kind {
			case Sunflower:
				l.Sun = min(9999, l.Sun+25)
				p.Wait = 100
			case Peashooter, Snowpea, Repeater:
				if nearest(l, row, col*Cell, Cols*Cell) < 0 {
					continue
				}
				count := 1
				if p.Kind == Repeater {
					count = 2
				}
				if len(l.Shots)+count <= MaxShots {
					for n := 0; n < count; n++ {
						l.Shots = append(l.Shots, Shot{row, col*Cell + 400 - n*180, p.Kind == Snowpea})
					}
				}
				p.Wait = 14
			case Cherry, Pepper:
				e.blast(l, row, col, 1, p.Kind == Pepper)
				*p = Plant{}
			case Chomper:
				if n := nearest(l, row, col*Cell, 900); n >= 0 {
					l.Zombies[n].HP = 0
					p.Wait = 120
					e.effect(l, row, col, "hit")
				}
			case Mine, Squash:
				reach := 350
				if p.Kind == Squash {
					reach = 850
				}
				if n := nearest(l, row, col*Cell, reach); n >= 0 {
					if p.Kind == Squash {
						x := l.Zombies[n].X
						for i := range l.Zombies {
							if l.Zombies[i].Row == row && abs(l.Zombies[i].X-x) <= 500 {
								l.Zombies[i].HP = 0
							}
						}
						e.effect(l, row, col, "blast")
					} else {
						e.blast(l, row, col, 0, false)
					}
					*p = Plant{}
				}
			}
		}
	}
}
func (e *Engine) projectiles(l *Lawn) {
	write := l.Shots[:0]
	for _, s := range l.Shots {
		next := s.X + 350
		index := -1
		best := Cols*Cell + 1
		for i, z := range l.Zombies {
			if z.HP > 0 && z.Row == s.Row && z.X >= s.X-180 && z.X <= next+180 && z.X < best {
				best = z.X
				index = i
			}
		}
		if index >= 0 {
			z := &l.Zombies[index]
			damage(z, 25)
			if s.Slow && !(z.Kind == Door && z.Armor > 0) {
				z.Slow = 50
			}
			e.effect(l, z.Row, z.X/Cell, "hit")
		} else if next <= Cols*Cell+500 {
			s.X = next
			write = append(write, s)
		}
	}
	l.Shots = write
}
func (e *Engine) zombies(l *Lawn) {
	initial := len(l.Zombies)
	for i := 0; i < initial; i++ {
		z := &l.Zombies[i]
		if z.HP <= 0 {
			continue
		}
		d := Zombies[z.Kind]
		if z.Slow > 0 {
			z.Slow--
		}
		if z.BiteWait > 0 {
			z.BiteWait--
		}
		if z.Kind == Dancer && !z.Summoned {
			z.SummonWait--
			if z.SummonWait <= 0 {
				z.Summoned = true
				row, x := z.Row, z.X
				for _, r := range []int{row - 1, row + 1} {
					if r >= 0 && r < Rows && e.totalZombies() < MaxZombies {
						backup := e.makeZombie(Backup, r, 0)
						backup.X = min(Cols*Cell, x+300)
						l.Zombies = append(l.Zombies, backup)
					}
				}
				z = &l.Zombies[i]
			}
		}
		col := min(Cols-1, z.X/Cell)
		if col >= 0 {
			p := &l.Grid[z.Row][col]
			if p.Kind != 0 && z.X <= col*Cell+650 {
				if z.Kind == Pole && !z.Jumped {
					z.Jumped = true
					z.X = max(-1, (col-1)*Cell+550)
					if z.X < 0 {
						l.Alive = false
					}
					continue
				}
				if z.BiteWait == 0 {
					p.HP -= d.Bite
					z.BiteWait = 5
					if p.HP <= 0 {
						*p = Plant{}
						e.effect(l, z.Row, col, "hit")
					}
				}
				continue
			}
		}
		speed := d.Speed
		if z.Kind == Newspaper && z.Armor == 0 {
			speed = 42
		}
		if z.Kind == Pole && z.Jumped {
			speed = 22
		}
		// Shared time pressure prevents an indefinitely static defensive setup.
		speed = speed * (10 + min(10, e.steps/1200)) / 10
		if z.Slow > 0 {
			speed = max(1, speed/2)
		}
		z.X -= speed
		if z.X < 0 {
			l.Alive = false
		}
	}
}

// Bots use only the displayed own lawn and public resources. Every placement
// goes through the same cost/cooldown/occupancy checks as a human command.
func (e *Engine) bot(seat int) {
	l := &e.lawns[seat]
	place := func(k PlantKind, r, c int) bool { return e.apply(seat, Action{"plant", r, c, k}) == nil }
	for _, z := range l.Zombies {
		if z.X < 1800 && place(Pepper, z.Row, 8) {
			return
		}
	}
	for r := 0; r < Rows; r++ {
		if l.Grid[r][0].Kind == 0 && place(Sunflower, r, 0) {
			return
		}
		if l.Grid[r][2].Kind == 0 && (nearest(l, r, 0, Cols*Cell) >= 0 || e.steps > 100) && place(Peashooter, r, 2) {
			return
		}
	}
	for r := 0; r < Rows; r++ {
		if l.Grid[r][4].Kind == 0 && nearest(l, r, 0, Cols*Cell) >= 0 && place(Wallnut, r, 4) {
			return
		}
		if l.Grid[r][1].Kind == 0 && place(Snowpea, r, 1) {
			return
		}
		if l.Grid[r][3].Kind == 0 && place(Repeater, r, 3) {
			return
		}
	}
}
