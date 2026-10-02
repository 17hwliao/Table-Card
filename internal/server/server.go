package server

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/table"
)

type Server struct {
	listenAddress net.Addr
	opMu          sync.Mutex
	runtime       map[string]*roomRuntime
	stats         *statStore
	stop          chan struct{}
	stopOnce      sync.Once
	rooms         *table.RoomManager
	registry      *games.Registry
	mu            sync.RWMutex
	engines       map[string]games.Engine
	hub           *roomHub
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
	{table.MahjongMode, "四川麻将", 4, 4, "血战规则引擎与定缺流程已接入"},
	{table.ChessMode, "中国象棋", 2, 2, "合法走子规则与终端棋盘已接入"},
	{table.WesternChessMode, "国际象棋", 2, 2, "规则引擎与终端棋盘已接入"},
	{table.GomokuMode, "五子棋", 2, 2, "规则引擎与终端棋盘已接入"},
	{table.GoMode, "围棋", 2, 2, "19 路规则引擎与终端棋盘已接入"},
	{table.UNOMode, "UNO", 2, 4, "108 张牌规则引擎已接入"},
	{table.TetrisMode, "俄罗斯方块", 2, 4, "双人 / 四人实时生存对战 · 人机练习"},
}

func New() *Server {
	s := &Server{
		rooms: table.NewRoomManager(), registry: games.NewDefaultRegistry(),
		engines: make(map[string]games.Engine), hub: newRoomHub(),
		runtime: make(map[string]*roomRuntime), stats: newStatStore(), stop: make(chan struct{}),
	}
	go s.tickLoop()
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/connection-info", s.connectionInfo)
	mux.HandleFunc("GET /api/modes", s.listModes)
	mux.HandleFunc("POST /api/rooms", s.createRoom)
	mux.HandleFunc("GET /api/rooms", s.listRooms)
	mux.HandleFunc("POST /api/match", s.quickMatch)
	mux.HandleFunc("GET /api/stats", s.playerStats)
	mux.HandleFunc("GET /api/rooms/{code}/ws", s.roomWebSocket)
	mux.HandleFunc("/api/rooms/", s.roomRoute)
	return mux
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("牌桌终端服务正在运行。请在终端运行 table-card 客户端连接此服务。\n"))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "table-card", "protocol": "2"})
}

func (s *Server) listModes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, modes)
}

type createRoomRequest struct {
	Options  json.RawMessage `json:"options,omitempty"`
	Mode     table.Mode      `json:"mode"`
	Seats    int             `json:"seats"`
	Bots     int             `json:"bots"`
	PlayerID string          `json:"playerId"`
	Name     string          `json:"name"`
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	var request createRoomRequest
	if !readJSON(w, r, &request) {
		return
	}
	if !authorize(w, r, request.PlayerID) {
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
	if request.Bots < 0 || request.Bots >= request.Seats {
		writeError(w, http.StatusBadRequest, "机器人数量必须小于座位数")
		return
	}
	if request.Mode == table.TetrisMode && request.Seats != 2 && request.Seats != 4 {
		writeError(w, http.StatusBadRequest, "俄罗斯方块只支持双人或四人对战")
		return
	}
	room, err := s.rooms.Create(request.Mode, request.Seats, table.Player{ID: request.PlayerID, Name: request.Name})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	room.SetOptions(request.Options)
	s.runtime[room.Snapshot().Code] = newRoomRuntime()
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
	s.opMu.Lock()
	defer s.opMu.Unlock()
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
		if !authorize(w, r, playerID) {
			return
		}
		if !isHumanRoomPlayer(room.Snapshot(), playerID) {
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
	case "leave":
		var request struct {
			PlayerID string `json:"playerId"`
		}
		if !readJSON(w, r, &request) {
			return
		}
		if !authorize(w, r, request.PlayerID) {
			return
		}
		if err := s.leaveRoom(room, request.PlayerID); err != nil {
			writeRoomError(w, err)
			return
		}
	case "rematch":
		var request struct {
			PlayerID string `json:"playerId"`
		}
		if !readJSON(w, r, &request) {
			return
		}
		if !authorize(w, r, request.PlayerID) {
			return
		}
		if !isHumanRoomPlayer(room.Snapshot(), request.PlayerID) {
			writeError(w, 403, "玩家不在房间内")
			return
		}
		old, exists := s.findEngine(parts[2])
		if !exists || !gameFinished(old) {
			writeError(w, 409, "当前对局尚未结束")
			return
		}
		engine, err := s.configuredEngine(room.Snapshot())
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		s.mu.Lock()
		s.engines[parts[2]] = engine
		s.mu.Unlock()
		s.runtime[parts[2]] = newRoomRuntime()
		s.broadcastGameState(parts[2], room, engine)
	case "join":
		var player table.Player
		if !readJSON(w, r, &player) {
			return
		}
		player.Bot = false
		if !authorize(w, r, player.ID) {
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
		if !authorize(w, r, request.PlayerID) {
			return
		}
		if err := room.SetReady(request.PlayerID, request.Ready); err != nil {
			writeRoomError(w, err)
			return
		}
		s.broadcastRoom(room)
	case "start":
		snapshot := room.Snapshot()
		id := r.Header.Get("X-Player-ID")
		if !authorize(w, r, id) {
			return
		}
		if !isHumanRoomPlayer(snapshot, id) {
			writeError(w, 403, "玩家不在此房间")
			return
		}
		engine, err := s.configuredEngine(snapshot)
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
		s.runtime[parts[2]].nextBot = time.Now().Add(3 * time.Second)
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
		if !isHumanRoomPlayer(room.Snapshot(), request.PlayerID) {
			writeError(w, http.StatusForbidden, "玩家不在这个房间中")
			return
		}
		if !authorize(w, r, request.PlayerID) {
			return
		}
		result, err := s.applyLocked(parts[2], room, engine, request.PlayerID, request.Action)
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

func isHumanRoomPlayer(snapshot table.Snapshot, playerID string) bool {
	if playerID == "" {
		return false
	}
	for _, player := range snapshot.Players {
		if player.ID == playerID {
			return !player.Bot
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
