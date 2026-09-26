package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/table"
	"net/http"
	"time"
)

type roomRuntime struct {
	nextBot        time.Time
	pauseUntil     time.Time
	matchAt        time.Time
	idleSince      time.Time
	recorded       map[string]bool
	previousScores map[string]int
	missing        map[string]time.Time
}

func newRoomRuntime() *roomRuntime {
	return &roomRuntime{nextBot: time.Now().Add(3 * time.Second), recorded: map[string]bool{}, previousScores: map[string]int{}, missing: map[string]time.Time{}}
}
func (s *Server) Close() { s.stopOnce.Do(func() { close(s.stop) }) }
func (s *Server) tickLoop() {
	timer := time.NewTicker(500 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-s.stop:
			return
		case now := <-timer.C:
			s.tick(now)
		}
	}
}
func (s *Server) tick(now time.Time) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	for _, room := range s.rooms.List() {
		snapshot := room.Snapshot()
		rt := s.runtime[snapshot.Code]
		if rt == nil {
			rt = newRoomRuntime()
			s.runtime[snapshot.Code] = rt
		}
		if !rt.matchAt.IsZero() && !now.Before(rt.matchAt) && snapshot.Phase == table.Waiting {
			for len(room.Snapshot().Players) < snapshot.Seats {
				i := len(room.Snapshot().Players)
				p := table.Player{ID: fmt.Sprintf("%s-bot-%d", snapshot.Code, i), Name: fmt.Sprintf("Sunjiajia %d", i), Bot: true}
				_ = room.Join(p)
				_ = room.SetReady(p.ID, true)
			}
			if err := s.startMatched(room); err == nil {
				rt.matchAt = time.Time{}
				if engine, ok := s.findEngine(snapshot.Code); ok {
					s.broadcastGameState(snapshot.Code, room, engine)
				}
			}
			snapshot = room.Snapshot()
		}
		connected := map[string]bool{}
		for _, p := range s.hub.roomPeers(snapshot.Code) {
			connected[p.playerID] = true
		}
		if len(connected) == 0 {
			if rt.idleSince.IsZero() {
				rt.idleSince = now
			}
			if now.Sub(rt.idleSince) > 2*time.Minute {
				s.removeRoom(snapshot.Code)
				continue
			}
		} else {
			rt.idleSince = time.Time{}
		}
		engine, ok := s.findEngine(snapshot.Code)
		if !ok {
			continue
		}
		s.recordResult(snapshot, engine, rt)
		if now.Before(rt.nextBot) || now.Before(rt.pauseUntil) || gameFinished(engine) {
			continue
		}
		for _, p := range snapshot.Players {
			if connected[p.ID] {
				delete(rt.missing, p.ID)
			}
			bot := p.Bot
			if !bot && !connected[p.ID] {
				if rt.missing[p.ID].IsZero() {
					rt.missing[p.ID] = now
				}
				bot = now.Sub(rt.missing[p.ID]) >= 30*time.Second
			}
			if !bot {
				continue
			}
			chooser, ok := engine.(interface{ BotAction(string) json.RawMessage })
			if !ok {
				continue
			}
			action := chooser.BotAction(p.ID)
			if len(action) == 0 {
				continue
			}
			if _, err := s.applyLocked(snapshot.Code, room, engine, p.ID, action); err == nil {
				s.broadcastGameState(snapshot.Code, room, engine)
			}
			rt.nextBot = now.Add(3 * time.Second)
			break
		}
	}
}
func (s *Server) applyLocked(code string, room *table.Room, engine games.Engine, id string, action json.RawMessage) (any, error) {
	rt := s.runtime[code]
	if rt == nil {
		rt = newRoomRuntime()
		s.runtime[code] = rt
	}
	if time.Now().Before(rt.pauseUntil) {
		return nil, errors.New("正在公示质疑与枪决结果，请稍候")
	}
	value, err := engine.Apply(id, action)
	if err != nil {
		return nil, err
	}
	rt.nextBot = time.Now().Add(3 * time.Second)
	var a struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(action, &a)
	if engine.Mode() == table.LiarBarMode && a.Type == "challenge" {
		rt.pauseUntil = time.Now().Add(10 * time.Second)
		rt.nextBot = rt.pauseUntil.Add(3 * time.Second)
	}
	s.recordResult(room.Snapshot(), engine, rt)
	return value, nil
}
func (s *Server) removeRoom(code string) {
	s.rooms.Remove(code)
	s.mu.Lock()
	delete(s.engines, code)
	s.mu.Unlock()
	delete(s.runtime, code)
}
func (s *Server) leaveRoom(room *table.Room, id string) error {
	snap := room.Snapshot()
	if !isHumanRoomPlayer(snap, id) {
		return table.ErrPlayerMissing
	}
	if snap.Phase == table.Waiting {
		if err := room.Leave(id); err != nil {
			return err
		}
	} else {
		if e, ok := s.findEngine(snap.Code); ok && !gameFinished(e) {
			for _, p := range snap.Players {
				if p.ID == id {
					s.stats.record(snap.Mode, p, 0, 1, 0, 0)
				}
			}
		}
		if err := room.TakeOver(id); err != nil {
			return err
		}
	}
	s.hub.disconnectPlayer(snap.Code, id)
	humans := 0
	for _, p := range room.Snapshot().Players {
		if !p.Bot {
			humans++
		}
	}
	if humans == 0 {
		s.removeRoom(snap.Code)
	} else {
		s.broadcastRoom(room)
	}
	return nil
}
func (s *Server) listRooms(w http.ResponseWriter, r *http.Request) {
	rooms := []table.Snapshot{}
	mode := table.Mode(r.URL.Query().Get("mode"))
	for _, room := range s.rooms.List() {
		v := room.Snapshot()
		if v.Phase == table.Waiting && (mode == "" || mode == v.Mode) {
			rooms = append(rooms, v)
		}
	}
	writeJSON(w, 200, rooms)
}
func (s *Server) quickMatch(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest
	if !readJSON(w, r, &req) {
		return
	}
	if !authorize(w, r, req.PlayerID) {
		return
	}
	info, ok := modeByID(req.Mode)
	if !ok {
		writeError(w, 400, "未知模式")
		return
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	player := table.Player{ID: req.PlayerID, Name: req.Name}
	for _, room := range s.rooms.List() {
		v := room.Snapshot()
		rt := s.runtime[v.Code]
		if v.Mode != req.Mode || v.Phase != table.Waiting || len(v.Players) >= v.Seats || rt == nil || rt.matchAt.IsZero() {
			continue
		}
		if err := room.Join(player); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		_ = room.SetReady(player.ID, true)
		if len(room.Snapshot().Players) == v.Seats {
			if err := s.startMatched(room); err != nil {
				writeError(w, 400, err.Error())
				return
			}
			rt.matchAt = time.Time{}
		}
		s.broadcastRoom(room)
		writeJSON(w, 200, room.Snapshot())
		return
	}
	room, err := s.rooms.Create(req.Mode, info.MaxSeats, player)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	_ = room.SetReady(player.ID, true)
	rt := newRoomRuntime()
	rt.matchAt = time.Now().Add(15 * time.Second)
	s.runtime[room.Snapshot().Code] = rt
	writeJSON(w, 201, room.Snapshot())
}
func (s *Server) startMatched(room *table.Room) error {
	v := room.Snapshot()
	engine, err := s.configuredEngine(v)
	if err != nil {
		return err
	}
	if err = room.Start(); err != nil {
		return err
	}
	s.mu.Lock()
	s.engines[v.Code] = engine
	s.mu.Unlock()
	s.runtime[v.Code].nextBot = time.Now().Add(3 * time.Second)
	return nil
}
func gameMap(engine games.Engine) map[string]any {
	b, _ := json.Marshal(engine.View(""))
	var v map[string]any
	_ = json.Unmarshal(b, &v)
	return v
}
func (s *Server) stateEnvelope(room *table.Room, engine games.Engine, id string) map[string]any {
	v := map[string]any{"type": "state", "room": room.Snapshot(), "game": engine.View(id)}
	if rt := s.runtime[room.Snapshot().Code]; rt != nil && !rt.pauseUntil.IsZero() {
		v["pauseUntil"] = rt.pauseUntil.Unix()
	}
	return v
}
func gameFinished(engine games.Engine) bool {
	v := gameMap(engine)
	if b, _ := v["finished"].(bool); b {
		return true
	}
	if b, _ := v["draw"].(bool); b {
		return true
	}
	if engine.Mode() == table.LandlordMode {
		return v["phase"] == float64(3)
	}
	winner, _ := v["winner"].(string)
	return winner != ""
}
