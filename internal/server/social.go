package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/coder/websocket"
)

// Social traffic has its own connection, so changing games or leaving a room
// does not interrupt private messages and the server-wide player roster.
type socialHub struct {
	mu           sync.RWMutex
	peers        map[*socketPeer]struct{}
	modes        map[string]table.Mode
	sessions     map[string]bool
	activities   map[string]string
	lastPresence string // Accessed only under Server.opMu.
}

type socialRequest struct {
	Type     string          `json:"type"`
	Scope    string          `json:"scope,omitempty"`
	TargetID string          `json:"targetId,omitempty"`
	Text     string          `json:"text,omitempty"`
	Mode     table.Mode      `json:"mode,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

type onlineUser struct {
	PlayerID  string     `json:"playerId"`
	Name      string     `json:"name"`
	Mode      table.Mode `json:"mode"`
	RoomCode  string     `json:"roomCode,omitempty"`
	Status    string     `json:"status"`
	Ready     bool       `json:"ready"`
	Host      bool       `json:"host"`
	Connected bool       `json:"connected"`
}

func newSocialHub() *socialHub {
	return &socialHub{peers: map[*socketPeer]struct{}{}, modes: map[string]table.Mode{}, sessions: map[string]bool{}, activities: map[string]string{}}
}

func (s *Server) socialPeers() []*socketPeer {
	s.social.mu.RLock()
	defer s.social.mu.RUnlock()
	peers := make([]*socketPeer, 0, len(s.social.peers))
	for p := range s.social.peers {
		peers = append(peers, p)
	}
	return peers
}

func (s *Server) sendToSocial(id string, value any) bool {
	delivered := false
	for _, p := range s.socialPeers() {
		if p.playerID == id && p.send(value) == nil {
			delivered = true
		}
	}
	return delivered
}

func (s *Server) socialMode(id string) table.Mode {
	s.social.mu.RLock()
	defer s.social.mu.RUnlock()
	return s.social.modes[id]
}

func validSocialMode(mode table.Mode) bool {
	if mode == "" || mode == table.PokemonMode || mode == table.SnakeMode {
		return true
	}
	_, ok := modeByID(mode)
	return ok
}

func (s *Server) socialWebSocket(w http.ResponseWriter, r *http.Request) {
	id, name := r.URL.Query().Get("playerId"), strings.TrimSpace(r.URL.Query().Get("name"))
	mode := table.Mode(r.URL.Query().Get("mode"))
	if !authorize(w, r, id) {
		return
	}
	if !table.ValidVisibleText(name, 32) || !validSocialMode(mode) {
		writeError(w, 400, "昵称或游戏模式无效")
		return
	}
	s.social.mu.Lock()
	if s.social.sessions[id] {
		s.social.mu.Unlock()
		writeError(w, http.StatusConflict, "同一玩家身份已在另一窗口在线，请关闭原窗口后重连")
		return
	}
	if len(s.social.sessions) >= 256 {
		s.social.mu.Unlock()
		writeError(w, http.StatusServiceUnavailable, "服务器在线人数已满，请稍后连接")
		return
	}
	s.social.sessions[id] = true
	s.social.mu.Unlock()
	defer func() { s.social.mu.Lock(); delete(s.social.sessions, id); s.social.mu.Unlock() }()
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit((2 << 20) + (16 << 10)) // Match the bounded adventure-save import.
	ctx, cancel := context.WithCancel(r.Context())
	peer := &socketPeer{playerID: id, name: name, conn: conn, out: make(chan []byte, 32), ctx: ctx, cancel: cancel}
	go peer.writeLoop()
	s.opMu.Lock()
	s.social.mu.Lock()
	s.social.peers[peer] = struct{}{}
	s.social.modes[id] = mode
	s.social.mu.Unlock()
	s.broadcastPresenceLocked()
	// Existing participants may make the cached roster unchanged on reconnect.
	_ = peer.send(map[string]any{"type": "presence", "data": s.presenceLocked()})
	s.opMu.Unlock()
	defer func() {
		cancel()
		_ = conn.CloseNow()
		s.opMu.Lock()
		s.social.mu.Lock()
		delete(s.social.peers, peer)
		remaining := false
		for p := range s.social.peers {
			if p.playerID == id {
				remaining = true
				break
			}
		}
		if !remaining {
			delete(s.social.modes, id)
		}
		s.social.mu.Unlock()
		s.broadcastPresenceLocked()
		s.opMu.Unlock()
		if !remaining {
			s.pokemonDisconnected(id)
		}
	}()
	lastMessage := time.Time{}
	pokemonWindow := time.Now()
	pokemonRequests := 0
	for {
		kind, payload, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if kind != websocket.MessageText {
			_ = peer.send(map[string]string{"type": "error", "error": "只接受 JSON 文本消息"})
			continue
		}
		var request socialRequest
		if json.Unmarshal(payload, &request) != nil {
			_ = peer.send(map[string]string{"type": "error", "error": "消息必须是合法 JSON"})
			continue
		}
		if strings.HasPrefix(request.Type, "pokemon_") {
			if time.Since(pokemonWindow) >= time.Second {
				pokemonWindow = time.Now()
				pokemonRequests = 0
			}
			pokemonRequests++
			if pokemonRequests > 12 {
				_ = peer.send(map[string]string{"type": "pokemon_error", "error": "联机操作过快，请稍后重试"})
				continue
			}
		}
		if request.Type == "chat" && time.Since(lastMessage) < 300*time.Millisecond {
			_ = peer.send(map[string]string{"type": "error", "error": "发送过快，请稍候"})
			continue
		}
		if request.Type == "chat" {
			lastMessage = time.Now()
		}
		s.handleSocialMessage(peer, request)
	}
}

func (s *Server) handleSocialMessage(peer *socketPeer, request socialRequest) {
	if strings.HasPrefix(request.Type, "pokemon_") {
		if !s.handlePokemonSocial(peer, request) {
			_ = peer.send(map[string]string{"type": "error", "error": "未知的宝可梦交互操作"})
		}
		return
	}
	if request.Type == "status" {
		if !validSocialMode(request.Mode) {
			_ = peer.send(map[string]string{"type": "error", "error": "未知的游戏模式"})
			return
		}
		s.opMu.Lock()
		s.social.mu.Lock()
		oldMode := s.social.modes[peer.playerID]
		s.social.modes[peer.playerID] = request.Mode
		s.social.mu.Unlock()
		s.broadcastPresenceLocked()
		s.opMu.Unlock()
		if oldMode == table.PokemonMode && request.Mode != table.PokemonMode {
			s.pokemonDisconnected(peer.playerID)
		}
		return
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	switch request.Type {
	case "chat":
		text := strings.TrimSpace(request.Text)
		if !table.ValidVisibleText(text, 300) {
			_ = peer.send(map[string]string{"type": "error", "error": "聊天需为1到300个可见字符，不能包含终端控制符"})
			return
		}
		scope := request.Scope
		if scope == "" {
			scope = "server"
		}
		message := map[string]any{"type": "chat", "scope": scope, "playerId": peer.playerID, "name": peer.name, "text": text, "targetId": request.TargetID}
		switch scope {
		case "server":
			for _, p := range s.socialPeers() {
				_ = p.send(message)
			}
		case "direct":
			if request.TargetID == "" || request.TargetID == peer.playerID {
				_ = peer.send(map[string]string{"type": "error", "error": "请选择另一位在线玩家"})
				return
			}
			if !s.sendToSocial(request.TargetID, message) {
				_ = peer.send(map[string]string{"type": "error", "error": "该玩家已离线，消息未送达"})
				return
			}
			s.sendToSocial(peer.playerID, message)
		case "room":
			room := s.socialRoomLocked(peer.playerID)
			if room == nil {
				_ = peer.send(map[string]string{"type": "error", "error": "当前不在房间中，请选择全服或私聊"})
				return
			}
			snapshot := room.Snapshot()
			message["roomCode"] = snapshot.Code
			for _, player := range snapshot.Players {
				if !player.Bot {
					s.sendToSocial(player.ID, message)
				}
			}
			// Legacy room-only clients can still see social room messages.
			for _, p := range s.hub.roomPeers(snapshot.Code) {
				if !s.hasSocialPlayer(p.playerID) {
					_ = p.send(message)
				}
			}
		default:
			_ = peer.send(map[string]string{"type": "error", "error": "聊天范围必须是 server、room 或 direct"})
		}
	default:
		_ = peer.send(map[string]string{"type": "error", "error": "未知的社交消息类型"})
	}
}

func (s *Server) hasSocialPlayer(id string) bool {
	for _, peer := range s.socialPeers() {
		if peer.playerID == id && peer.ctx.Err() == nil {
			return true
		}
	}
	return false
}

func (s *Server) socialRoomLocked(id string) *table.Room {
	for _, room := range s.rooms.List() {
		if isHumanRoomPlayer(room.Snapshot(), id) {
			return room
		}
	}
	return nil
}

// Call while opMu is held: engine status and room membership are coherent.
func (s *Server) presenceLocked() []onlineUser {
	byID := map[string]onlineUser{}
	for _, p := range s.socialPeers() {
		if p.ctx.Err() == nil {
			mode := s.socialMode(p.playerID)
			status := "lobby"
			if mode == table.PokemonMode {
				status = "adventuring"
				s.social.mu.RLock()
				if activity := s.social.activities[p.playerID]; activity != "" {
					status = activity
				}
				s.social.mu.RUnlock()
			} else if mode == table.SnakeMode {
				status = "playing"
			}
			byID[p.playerID] = onlineUser{PlayerID: p.playerID, Name: p.name, Mode: mode, Status: status, Connected: true}
		}
	}
	for _, room := range s.rooms.List() {
		snapshot := room.Snapshot()
		connected := map[string]bool{}
		for _, p := range s.hub.roomPeers(snapshot.Code) {
			if p.ctx.Err() == nil {
				connected[p.playerID] = true
			}
		}
		for i, p := range snapshot.Players {
			if p.Bot {
				continue
			}
			user := byID[p.ID]
			user.PlayerID, user.Name, user.Mode, user.RoomCode = p.ID, p.Name, snapshot.Mode, snapshot.Code
			user.Ready, user.Host = p.Ready, i == 0
			user.Connected = user.Connected || connected[p.ID]
			user.Status = "waiting"
			if snapshot.Phase == table.InProgress {
				user.Status = "playing"
				if runtime := s.runtime[snapshot.Code]; runtime != nil && runtime.finished {
					user.Status = "finished"
				}
			}
			if !user.Connected {
				user.Status = "offlineRoom"
			}
			byID[p.ID] = user
		}
	}
	users := make([]onlineUser, 0, len(byID))
	for _, user := range byID {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].Name == users[j].Name {
			return users[i].PlayerID < users[j].PlayerID
		}
		return users[i].Name < users[j].Name
	})
	return users
}

func (s *Server) broadcastPresenceLocked() {
	users := s.presenceLocked()
	payload, _ := json.Marshal(users)
	if string(payload) == s.social.lastPresence {
		return
	}
	s.social.lastPresence = string(payload)
	for _, peer := range s.socialPeers() {
		_ = peer.send(map[string]any{"type": "presence", "data": users})
	}
}
