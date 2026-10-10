// Package pokemon implements an original text adventure using factual species
// data. Battles are intentionally simplified; capture totals follow Gen I.
package pokemon

import (
	_ "embed"
	"encoding/json"
)

type Evolution struct {
	Species, Level int
	Trade          bool
	Item           string
}
type Species struct {
	ID                                                      int
	Name, English                                           string
	Types                                                   []int
	HP, Attack, Defense, Special, Speed, CatchRate, BaseExp int
	Evolves                                                 []Evolution
	Growth                                                  string
	Learnset                                                []LearnMove
}

type LearnMove struct{ Level, Move int }

//go:embed species.json
var speciesJSON []byte
var Dex [152]Species

func init() {
	var data []Species
	if err := json.Unmarshal(speciesJSON, &data); err != nil {
		panic(err)
	}
	var seen [152]bool
	for _, s := range data {
		if s.ID < 1 || s.ID > 151 || seen[s.ID] || s.Name == "" || len(s.Types) == 0 || s.HP <= 0 || s.CatchRate < 1 || s.CatchRate > 255 {
			panic("invalid species data")
		}
		seen[s.ID] = true
		Dex[s.ID] = s
	}
	if len(data) != 151 {
		panic("incomplete species data")
	}
	for id := 1; id <= 151; id++ {
		Moves[25+id] = Move{Name: signatureName(id), Type: Dex[id].Types[0], Power: 95, Accuracy: 100, PP: 8, Effect: "ORIGINAL_STORY_SKILL"}
	}
	var moves []Move
	if err := json.Unmarshal(genOneMovesJSON, &moves); err != nil {
		panic(err)
	}
	for _, move := range moves {
		if move.ID < 0 || move.ID >= len(Moves) {
			panic("invalid move ID")
		}
		Moves[move.ID] = move
	}
	loadEncounters()
}

var TypeNames = map[int]string{1: "一般", 2: "格斗", 3: "飞行", 4: "毒", 5: "地面", 6: "岩石", 7: "虫", 8: "幽灵", 10: "火", 11: "水", 12: "草", 13: "电", 14: "超能力", 15: "冰", 16: "龙"}

type Move struct {
	Name                      string
	Type, Power, Accuracy, PP int
	Status                    string
	Chance                    int
	ID                        int
	Key, Effect               string
}

//go:embed moves_gen1.json
var genOneMovesJSON []byte

// Indices 0..176 preserve old saves. Original story skills remain explicitly separate.
var Moves = make([]Move, 316)

func moveSet(id, level int) [4]int {
	result := [4]int{-1, -1, -1, -1}
	known := []int{}
	for _, entry := range Dex[id].Learnset {
		if entry.Level > level {
			continue
		}
		duplicate := false
		for _, n := range known {
			duplicate = duplicate || n == entry.Move
		}
		if !duplicate {
			known = append(known, entry.Move)
		}
	}
	if len(known) > 4 {
		known = known[len(known)-4:]
	}
	copy(result[:], known)
	return result
}

type Area struct {
	Name, Intro, Goal  string
	MinLevel, MaxLevel int
	Wild, Trainers     []int
	Boss, Badge        string
	BossTeam           []int
	BossLevel          int
}

var Areas = []Area{
	{"真新镇", "海风穿过白色围栏。大木博士邀请你从一位训练新人成长为独当一面的伙伴。", "选择初始伙伴，赢得镇上6名训练者的挑战，再向博士汇报。", 2, 4, []int{16, 19, 10, 13, 21, 29, 32}, []int{19, 16, 10, 13, 21, 25}, "", "", nil, 0},
	{"常磐森林·尼比市", "常磐市补给后，你进入虫鸣不断的森林。尼比市的岩石道馆就在路的尽头。", "完成6次路途挑战，击败小刚取得岩石徽章。", 6, 10, []int{10, 11, 12, 13, 14, 15, 25, 21, 56}, []int{10, 13, 11, 14, 12, 15}, "小刚", "岩石徽章", []int{74, 95}, 14},
	{"月见山·华蓝市", "山洞里有人争抢古老化石。穿过月见山，你来到水道环绕的华蓝市。", "完成6次挑战，处理化石事件、帮助正辉，挑战小霞。", 10, 16, []int{41, 74, 35, 46, 27, 23, 43, 54, 63}, []int{41, 74, 27, 23, 43, 54}, "小霞", "蓝色徽章", []int{120, 121}, 21},
	{"枯叶市", "港口的汽笛声响起。圣特安努号正在等候持有船票的客人。", "完成6次挑战，登船获得居合斩许可，击败马志士。", 16, 23, []int{50, 51, 52, 56, 58, 60, 81, 100}, []int{52, 56, 58, 60, 81, 100}, "马志士", "橙色徽章", []int{100, 25, 26}, 24},
	{"紫苑镇", "岩山隧道的另一端是安静的紫苑镇。宝可梦塔里有无法辨认的幽灵。", "完成6次挑战、调查宝可梦塔；到玉虹寻找西尔佛检视镜后回来。", 20, 28, []int{41, 42, 74, 75, 92, 93, 96, 104}, []int{41, 92, 96, 104, 93, 42}, "", "", nil, 0},
	{"玉虹市", "花园道馆与游戏厅相邻，地下通道里却传来火箭队的密谈。", "赢6次挑战、击败莉佳，潜入火箭队基地，回紫苑救出富士老人。", 24, 32, []int{37, 43, 44, 48, 49, 58, 69, 70, 133}, []int{43, 69, 48, 37, 44, 70}, "莉佳", "彩虹徽章", []int{71, 114, 45}, 29},
	{"浅红市", "宝可梦之笛让挡路的卡比兽醒来。浅红市的狩猎地带藏着新的旅行技巧。", "赢6次挑战，取得冲浪与金牙，击败阿桔。", 28, 36, []int{84, 102, 111, 113, 115, 123, 127, 128, 147}, []int{84, 102, 111, 123, 48, 49}, "阿桔", "粉红徽章", []int{109, 89, 109, 110}, 43},
	{"金黄市", "都市中央的西尔佛公司被火箭队封锁。超能力道馆的大门也在等待你。", "赢6次挑战，解救西尔佛公司，击败娜姿。", 32, 40, []int{63, 64, 66, 67, 96, 97, 106, 107}, []int{64, 67, 97, 66, 96, 63}, "娜姿", "金色徽章", []int{64, 122, 49, 65}, 43},
	{"红莲岛", "乘着伙伴穿过海面，你看见火山岛和废弃研究宅邸。", "赢6次挑战，在宅邸找到钥匙，击败夏伯。", 36, 44, []int{58, 77, 88, 89, 109, 110, 126, 138, 140, 142}, []int{58, 77, 109, 88, 110, 78}, "夏伯", "深红徽章", []int{58, 77, 78, 59}, 47},
	{"常磐市", "旅程回到最初的路口。常磐道馆终于开门，馆主正是火箭队首领。", "赢6次挑战，击败坂木取得最后一枚徽章。", 40, 48, []int{22, 24, 28, 31, 34, 57, 78, 112, 143}, []int{22, 28, 57, 112, 31, 34}, "坂木", "绿色徽章", []int{111, 51, 31, 34, 112}, 50},
	{"冠军之路·石英高原", "八枚徽章换来通行资格。山路尽头，四天王和劲敌守候着联盟殿堂。", "赢6次挑战，连续击败四天王和冠军。", 44, 52, []int{42, 55, 67, 75, 95, 105, 112, 114, 132}, []int{67, 75, 42, 105, 95, 112}, "", "", nil, 0},
	{"华蓝洞窟·冠军后篇", "联盟记录下你的名字。华蓝洞窟开放，新的调查与收集还在继续。", "探索151种宝可梦，挑战传说精灵，完成图鉴。", 50, 65, []int{64, 65, 82, 85, 97, 101, 112, 113, 115, 123, 125, 126, 127, 128, 131, 143, 148, 149}, nil, "", "", nil, 0},
}
