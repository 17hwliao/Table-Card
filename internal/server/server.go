package server

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/17hwliao/table-card-independent/internal/table"
)

//go:embed web/index.html
var assets embed.FS

type Server struct {
	rooms *table.RoomManager
}

type modeInfo struct {
	ID       table.Mode `json:"id"`
	Name     string     `json:"name"`
	MinSeats int        `json:"minSeats"`
	MaxSeats int        `json:"maxSeats"`
	Progress string     `json:"progress"`
}

var modes = []modeInfo{
	{table.LandlordMode, "斗地主", 3, 3, "牌型规则与 Sunjiajia 机器人基础"},
	{table.LiarBarMode, "骗子酒馆", 4, 4, "对局规则引擎"},
	{table.MahjongMode, "四川麻将", 4, 4, "开发中"},
	{table.ChessMode, "中国象棋", 2, 2, "开发中"},
	{table.WesternChessMode, "国际象棋", 2, 2, "开发中"},
	{table.GomokuMode, "五子棋", 2, 2, "开发中"},
	{table.GoMode, "围棋", 2, 2, "开发中"},
	{table.UNOMode, "UNO", 2, 4, "开发中"},
}

func New() *Server {
	return &Server{rooms: table.NewRoomManager()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.home)
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/modes", s.listModes)
	mux.HandleFunc("POST /api/rooms", s.createRoom)
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
	room, err := s.rooms.Create(request.Mode, request.Seats, table.Player{ID: request.PlayerID, Name: request.Name})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
	case "start":
		if err := room.Start(); err != nil {
			writeRoomError(w, err)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, room.Snapshot())
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
