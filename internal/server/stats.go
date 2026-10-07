package server

import (
	"encoding/json"
	"fmt"
	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/table"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type statistic struct {
	PlayerID string     `json:"playerId"`
	Name     string     `json:"name"`
	Mode     table.Mode `json:"mode"`
	Wins     int        `json:"wins"`
	Losses   int        `json:"losses"`
	Draws    int        `json:"draws"`
	Score    int        `json:"score"`
}
type statStore struct {
	mu      sync.Mutex
	path    string
	entries map[string]statistic
	loadErr error
}

func newStatStore() *statStore {
	dir := os.Getenv("TABLE_CARD_DATA_DIR")
	if dir == "" {
		dir, _ = os.UserConfigDir()
		dir = filepath.Join(dir, "Table-Card", "server")
	}
	s := &statStore{path: filepath.Join(dir, "stats.json"), entries: map[string]statistic{}}
	if data, err := os.ReadFile(s.path); err == nil {
		if err = json.Unmarshal(data, &s.entries); err != nil || s.entries == nil {
			if err == nil {
				err = fmt.Errorf("战绩必须是 JSON 对象")
			}
			log.Printf("读取战绩失败：%v", err)
			s.loadErr = err
			s.entries = map[string]statistic{}
		}
	} else if !os.IsNotExist(err) {
		s.loadErr = err
		log.Printf("读取战绩失败，保留原文件：%v", err)
	}
	return s
}
func (s *statStore) record(mode table.Mode, p table.Player, w, l, d, score int) {
	if p.Bot {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := string(mode) + "/" + p.ID
	v := s.entries[key]
	v.PlayerID = p.ID
	v.Name = p.Name
	v.Mode = mode
	v.Wins += w
	v.Losses += l
	v.Draws += d
	v.Score += score
	s.entries[key] = v
	if s.loadErr != nil {
		// Keep session results in memory, without replacing a damaged history.
		return
	}
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(s.path), 0700)
	}
	if err == nil {
		err = os.WriteFile(s.path+".tmp", data, 0600)
	}
	if err == nil {
		err = os.Rename(s.path+".tmp", s.path)
	}
	if err != nil {
		log.Printf("保存战绩失败：%v", err)
	}
}
func (s *Server) playerStats(w http.ResponseWriter, r *http.Request) {
	mode := table.Mode(r.URL.Query().Get("mode"))
	id := r.URL.Query().Get("playerId")
	if !authorize(w, r, id) {
		return
	}
	s.stats.mu.Lock()
	defer s.stats.mu.Unlock()
	rows := []statistic{}
	for _, v := range s.stats.entries {
		if v.Mode == mode {
			rows = append(rows, v)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].Wins > rows[j].Wins
	})
	if len(rows) > 50 {
		rows = rows[:50]
	}
	warning := ""
	if s.stats.loadErr != nil {
		warning = "原战绩文件读取失败，已保留；本次运行结果仅在内存中，请恢复文件后重启服务端"
	}
	writeJSON(w, 200, map[string]any{"leaderboard": rows, "mine": s.stats.entries[string(mode)+"/"+id], "warning": warning})
}
func (s *Server) recordResult(room table.Snapshot, engine games.Engine, rt *roomRuntime) bool {
	if !gameFinished(engine) {
		return false
	}
	v := gameMap(engine)
	key := fmt.Sprint(v["round"], "/", v["winner"], "/", v["phase"])
	if rt.recorded[key] {
		return true
	}
	rt.recorded[key] = true
	winner, _ := v["winner"].(string)
	landlord, _ := v["landlord"].(float64)
	seatWinner, _ := v["winner"].(float64)
	isDraw, _ := v["draw"].(bool)
	scores := map[string]int{}
	if players, ok := v["players"].([]any); ok {
		for _, value := range players {
			p, ok := value.(map[string]any)
			if !ok {
				continue
			}
			id, _ := p["id"].(string)
			score, _ := p["score"].(float64)
			scores[id] = int(score)
		}
	}
	for i, p := range room.Players {
		if p.Bot {
			continue
		}
		won := winner == p.ID
		draw := isDraw
		if room.Mode == table.LandlordMode {
			won = (int(seatWinner) == int(landlord)) == (i == int(landlord))
		}
		score := 0
		if room.Mode == table.MahjongMode {
			score = scores[p.ID] - rt.previousScores[p.ID]
			rt.previousScores[p.ID] = scores[p.ID]
			won = score > 0
			draw = score == 0
		} else if room.Mode == table.LandlordMode {
			multiplier, _ := v["multiplier"].(float64)
			score = max(1, int(multiplier))
			if i == int(landlord) {
				score *= 2
			}
			if !won {
				score = -score
			}
		} else if room.Mode == table.UNOMode {
			score = scores[p.ID] - rt.previousScores[p.ID]
			rt.previousScores[p.ID] = scores[p.ID]
		} else {
			if won {
				score = 20
			} else if draw {
				score = 5
			} else {
				score = -10
			}
		}
		w, l, d := 0, 0, 0
		if draw {
			d = 1
		} else if won {
			w = 1
		} else {
			l = 1
		}
		s.stats.record(room.Mode, p, w, l, d, score)
	}
	return true
}
