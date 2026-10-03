package pokemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

func SavePath(name string) (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(name)))
	return filepath.Join(root, "Table-Card", "pokemon", hex.EncodeToString(sum[:12])+".json"), nil
}
func Load(name string) (*Game, error) {
	g := New(name)
	path, err := SavePath(name)
	if err != nil {
		return g, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return g, nil
	}
	if err != nil {
		return g, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 2*1024*1024+1))
	if err != nil {
		return g, err
	}
	if len(data) > 2*1024*1024 {
		return g, errors.New("存档过大")
	}
	var s State
	if err = json.Unmarshal(data, &s); err != nil {
		return g, fmt.Errorf("无法解析存档：%w", err)
	}
	if err = validate(s); err != nil {
		return g, err
	}
	if s.Name != name {
		return g, errors.New("存档昵称不匹配")
	}
	g.State = s
	g.Version = 2
	return g, nil
}
func (g *Game) Save() error {
	if err := validate(g.State); err != nil {
		return err
	}
	path, err := SavePath(g.Name)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(g.State)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".pokemon-*.tmp")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
func validate(s State) error {
	// Version 1 saves migrate with optional navigation/teaching fields at zero.
	if s.Location < 0 || s.Location >= 8 {
		return errors.New("存档地点编号不合法")
	}
	bad := func() error {
		return errors.New("存档结构不合法；请备份存档后再决定是否输入“new yes”")
	}
	if (s.Version != 1 && s.Version != 2) || s.Area < 0 || s.Area >= len(Areas) || s.Unlocked < s.Area || s.Unlocked >= len(Areas) || s.Money < 0 || s.Money > 1000000000 || s.Elite < 0 || s.Elite > 5 || len(s.Party) > 6 || len(s.Box) > 600 || len(s.TrainerWins) != len(Areas) || len(s.Badges) > 8 || len(s.Log) > 120 || len(s.Name) > 200 {
		return bad()
	}
	if s.Items == nil || s.Flags == nil || s.Seen == nil || s.Caught == nil {
		return bad()
	}
	if s.Completed && (s.Elite != 5 || s.Unlocked != 11) {
		return bad()
	}
	if !s.Completed && s.Elite == 5 {
		return bad()
	}
	for _, n := range s.TrainerWins {
		if n < 0 || n > 6 {
			return bad()
		}
	}
	for item, n := range s.Items {
		if n < 0 || n > 999 || len(item) > 80 {
			return bad()
		}
	}
	for id := range s.Seen {
		if id < 1 || id > 151 {
			return bad()
		}
	}
	for id := range s.Caught {
		if id < 1 || id > 151 {
			return bad()
		}
	}
	checkMon := func(m Monster) bool {
		if m.Species < 1 || m.Species > 151 || m.Level < 1 || m.Level > 100 || m.Exp < m.Level*m.Level*m.Level || m.Exp > 1000000 || m.HP < 0 || m.HP > m.MaxHP() || m.Sleep < 0 || m.Sleep > 4 {
			return false
		}
		switch m.Status {
		case "", "中毒", "麻痹", "睡眠", "灼伤", "冰冻":
		default:
			return false
		}
		for i, id := range m.Moves {
			if id < 0 || id >= len(Moves) || m.PP[i] < 0 || m.PP[i] > Moves[id].PP {
				return false
			}
		}
		return true
	}
	for _, m := range s.Party {
		if !checkMon(m) {
			return bad()
		}
	}
	for _, m := range s.Box {
		if !checkMon(m) {
			return bad()
		}
	}
	if b := s.Battle; b != nil {
		if b.Area < 0 || b.Area > 11 || b.Area != s.Area || b.Active < 0 || b.Active >= len(s.Party) || b.Enemy < 0 || b.Enemy >= len(b.Foes) || len(b.Foes) > 6 || b.Turns < 0 || b.Challenge < 0 || b.Challenge > 6 || s.Party[b.Active].HP <= 0 {
			return bad()
		}
		switch b.Kind {
		case "wild", "trainer", "gym", "event", "league":
		default:
			return bad()
		}
		for _, m := range b.Foes {
			if !checkMon(m) {
				return bad()
			}
		}
		if b.Foes[b.Enemy].HP <= 0 {
			return bad()
		}
	}
	if c := s.Capture; c != nil {
		if s.Battle == nil || s.Battle.Kind != "wild" || c.Step < 0 || c.Step > 2 || math.IsNaN(c.Probability) || c.Probability < 0 || c.Probability > 1 {
			return bad()
		}
		for i := 0; i < c.Step; i++ {
			if !c.Passed[i] {
				return bad()
			}
		}
		switch c.Ball {
		case "精灵球", "超级球", "高级球", "大师球":
		default:
			return bad()
		}
	}
	return nil
}
