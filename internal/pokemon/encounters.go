package pokemon

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type EncounterSlot struct{ Species, Level, Weight int }
type EncounterTable struct {
	Area                  int
	Location, Map, Method string
	StepRate              int
	Slots                 []EncounterSlot
}

//go:embed encounters_red.json
var redEncountersJSON []byte
var redEncounters []EncounterTable

func loadEncounters() {
	if err := json.Unmarshal(redEncountersJSON, &redEncounters); err != nil {
		panic(err)
	}
	if len(redEncounters) != len(Areas) {
		panic("incomplete Red encounter maps")
	}
	for i, t := range redEncounters {
		if t.Area != i || len(t.Slots) != 10 {
			panic("invalid Red encounter slots")
		}
		total := 0
		wild := []int{}
		seen := map[int]bool{}
		for _, s := range t.Slots {
			if s.Species < 1 || s.Species > 151 || s.Level < 1 || s.Level > 100 || s.Weight <= 0 {
				panic("invalid encounter")
			}
			total += s.Weight
			if !seen[s.Species] {
				wild = append(wild, s.Species)
				seen[s.Species] = true
			}
		}
		if total != 256 {
			panic("invalid encounter weights")
		}
		Areas[i].Wild = wild
	}
}

func EncounterLocation(area int) string {
	if area < 0 || area >= len(redEncounters) {
		return "未知地点"
	}
	return redEncounters[area].Location
}

// Percentages are conditional on an encounter, not chance per exploration command.
// This adventure deliberately performs one encounter per explicit grass command.
func EncounterInfo(area int) string {
	if area < 0 || area >= len(redEncounters) {
		return "没有野生遭遇资料"
	}
	t := redEncounters[area]
	weights := map[int]int{}
	levels := map[int][2]int{}
	for _, s := range t.Slots {
		weights[s.Species] += s.Weight
		lv, ok := levels[s.Species]
		if !ok {
			lv = [2]int{s.Level, s.Level}
		}
		lv[0] = min(lv[0], s.Level)
		lv[1] = max(lv[1], s.Level)
		levels[s.Species] = lv
	}
	ids := make([]int, 0, len(weights))
	for id := range weights {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	rows := []string{fmt.Sprintf("红版 · %s · %s遭遇", t.Location, t.Method), "概率为已经遇敌后的物种占比；grass / 菜单进入草丛必定触发一次遭遇。"}
	for _, id := range ids {
		lv := levels[id]
		rows = append(rows, fmt.Sprintf("%s · Lv%d–%d · %.2f%%", Dex[id].Name, lv[0], lv[1], float64(weights[id])*100/256))
	}
	rows = append(rows, "御三家 / 伊布 / 化石 / 传说不在普通随机遭遇中；赠送、复活、交换、固定事件另行获得。")
	if area == 4 {
		rows = append(rows, "宝可梦塔需要西尔佛检视镜辨认幽灵。")
	}
	return strings.Join(rows, "\n")
}

func (g *Game) wildEncounter() (int, int) {
	t := redEncounters[g.Area]
	roll := g.rng.Intn(256)
	for _, s := range t.Slots {
		roll -= s.Weight
		if roll < 0 {
			return s.Species, s.Level
		}
	}
	return t.Slots[0].Species, t.Slots[0].Level
}
