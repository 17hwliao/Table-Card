package server

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/table"
)

//go:embed web/index.html
var assets embed.FS

type Server struct {
	rooms    *table.RoomManager
	registry *games.Registry
	mu       sync.RWMutex
	engines  map[string]games.Engine
	hub      *roomHub
}

type modeInfo struct {
	ID       table.Mode `json:"id"`
	Name     string     `json:"name"`
	MinSeats int        `json:"minSeats"`
	MaxSeats int        `json:"maxSeats"`
	Progress string     `json:"progress"`
}

var modes = []modeInfo{
	{table.LandlordMode, "斗地主", 3, 3, "Sunjiajia 人机训练与基础牌桌已接入"},
	{table.LiarBarMode, "骗子酒馆", 4, 4, "规则引擎与基础牌桌已接入"},
	{table.MahjongMode, "四川麻将", 4, 4, "开发中"},
	{table.ChessMode, "中国象棋", 2, 2, "开发中"},
	{table.WesternChessMode, "国际象棋", 2, 2, "开发中"},
	{table.GomokuMode, "五子棋", 2, 2, "开发中"},
	{table.GoMode, "围棋", 2, 2, "开发中"},
	{table.UNOMode, "UNO", 2, 4, "开发中"},
}

func New() *Server {
	return &Server{
		rooms: table.NewRoomManager(), registry: games.NewDefaultRegistry(),
		engines: make(map[string]games.Engine), hub: newRoomHub(),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.home)
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/modes", s.listModes)
	mux.HandleFunc("POST /api/rooms", s.createRoom)
	mux.HandleFunc("GET /api/rooms/{code}/ws", s.roomWebSocket)
	mux.HandleFunc("/api/rooms/", s.roomRoute)
	return mux
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	page, err := assets.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "大厅页面不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listModes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, modes)
}

type createRoomRequest struct {
	Mode     table.Mode `json:"mode"`
	Seats    int        `json:"seats"`
	Bots     int        `json:"bots"`
	PlayerID string     `json:"playerId"`
	Name     string     `json:"name"`
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	var request createRoomRequest
	if !readJSON(w, r, &request) {
		return
	}
	definition, ok := modeByID(request.Mode)
	if !ok {
		writeError(w, http.StatusBadRequest, "未知的游戏模式")
		return
	}
	if request.Seats < definition.MinSeats || request.Seats > definition.MaxSeats {
		writeError(w, http.StatusBadRequest, "该模式的座位数无效")
		return
	}
	if request.Bots < 0 || request.Bots >= request.Seats || request.Bots > 0 && request.Mode != table.LandlordMode {
		writeError(w, http.StatusBadRequest, "当前只有斗地主支持 Sunjiajia 人机训练，机器人数量必须小于座位数")
		return
	}
	room, err := s.rooms.Create(request.Mode, request.Seats, table.Player{ID: request.PlayerID, Name: request.Name})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for i := 0; i < request.Bots; i++ {
		bot := table.Player{ID: room.Snapshot().Code + "-sunjiajia-" + string(rune('1'+i)), Name: "Sunjiajia " + string(rune('A'+i)), Bot: true}
		if err := room.Join(bot); err != nil {
			s.rooms.Remove(room.Snapshot().Code)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := room.SetReady(bot.ID, true); err != nil {
			s.rooms.Remove(room.Snapshot().Code)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusCreated, room.Snapshot())
}

func (s *Server) roomRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "rooms" {
		http.NotFound(w, r)
		return
	}
	room, ok := s.rooms.Find(parts[2])
	if !ok {
		writeError(w, http.StatusNotFound, "没有找到这个房间")
		return
	}
	if len(parts) == 3 && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, room.Snapshot())
		return
	}
	if len(parts) == 4 && parts[3] == "state" && r.Method == http.MethodGet {
		engine, ok := s.findEngine(parts[2])
		if !ok {
			writeError(w, http.StatusConflict, "房间尚未开始，或该模式的对局尚未接入")
			return
		}
		playerID := r.URL.Query().Get("playerId")
		if !isRoomPlayer(room.Snapshot(), playerID) {
			writeError(w, http.StatusForbidden, "玩家不在这个房间中")
			return
		}
		writeJSON(w, http.StatusOK, engine.View(playerID))
		return
	}
	if len(parts) != 4 || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	switch parts[3] {
	case "join":
		var player table.Player
		if !readJSON(w, r, &player) {
			return
		}
		if err := room.Join(player); err != nil {
			writeRoomError(w, err)
			return
		}
		s.broadcastRoom(room)
	case "ready":
		var request struct {
			PlayerID string `json:"playerId"`
			Ready    bool   `json:"ready"`
		}
		if !readJSON(w, r, &request) {
			return
		}
		if err := room.SetReady(request.PlayerID, request.Ready); err != nil {
			writeRoomError(w, err)
			return
		}
		s.broadcastRoom(room)
	case "start":
		snapshot := room.Snapshot()
		engine, err := s.registry.New(snapshot.Mode, snapshot.Players)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		if err := room.Start(); err != nil {
			writeRoomError(w, err)
			return
		}
		s.mu.Lock()
		s.engines[parts[2]] = engine
		s.mu.Unlock()
		s.broadcastGameState(parts[2], room, engine)
	case "action":
		engine, ok := s.findEngine(parts[2])
		if !ok {
			writeError(w, http.StatusConflict, "房间尚未开始，或该模式的对局尚未接入")
			return
		}
		var request struct {
			PlayerID string          `json:"playerId"`
			Action   json.RawMessage `json:"action"`
		}
		if !readJSON(w, r, &request) {
			return
		}
		if !isRoomPlayer(room.Snapshot(), request.PlayerID) {
			writeError(w, http.StatusForbidden, "玩家不在这个房间中")
			return
		}
		result, err := engine.Apply(request.PlayerID, request.Action)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.broadcastGameState(parts[2], room, engine)
		writeJSON(w, http.StatusOK, result)
		return
	default:
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, room.Snapshot())
}

func (s *Server) findEngine(code string) (games.Engine, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	engine, ok := s.engines[code]
	return engine, ok
}

func isRoomPlayer(snapshot table.Snapshot, playerID string) bool {
	if playerID == "" {
		return false
	}
	for _, player := range snapshot.Players {
		if player.ID == playerID {
			return true
		}
	}
	return false
}

func modeByID(id table.Mode) (modeInfo, bool) {
	for _, mode := range modes {
		if mode.ID == id {
			return mode, true
		}
	}
	return modeInfo{}, false
}

func readJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "请求内容无效: "+err.Error())
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "请求只能包含一个 JSON 对象")
		return false
	}
	return true
}

func writeRoomError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, table.ErrPlayerMissing) {
		status = http.StatusNotFound
	}
	writeError(w, status, err.Error())
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
