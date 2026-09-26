package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/coder/websocket"
)

type roomHub struct {
	mu    sync.RWMutex
	peers map[string]map[*socketPeer]struct{}
}

type socketPeer struct {
	roomCode string
	playerID string
	name     string
	conn     *websocket.Conn
	writeMu  sync.Mutex
}

type socketRequest struct {
	Type   string          `json:"type"`
	Text   string          `json:"text,omitempty"`
	Action json.RawMessage `json:"action,omitempty"`
}

func newRoomHub() *roomHub {
	return &roomHub{peers: make(map[string]map[*socketPeer]struct{})}
}

func (h *roomHub) add(peer *socketPeer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.peers[peer.roomCode] == nil {
		h.peers[peer.roomCode] = make(map[*socketPeer]struct{})
	}
	h.peers[peer.roomCode][peer] = struct{}{}
}

func (h *roomHub) remove(peer *socketPeer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.peers[peer.roomCode], peer)
	if len(h.peers[peer.roomCode]) == 0 {
		delete(h.peers, peer.roomCode)
	}
}

func (h *roomHub) disconnectPlayer(roomCode, playerID string) {
	h.mu.Lock()
	var peers []*socketPeer
	for peer := range h.peers[roomCode] {
		if peer.playerID == playerID {
			delete(h.peers[roomCode], peer)
			peers = append(peers, peer)
		}
	}
	if len(h.peers[roomCode]) == 0 {
		delete(h.peers, roomCode)
	}
	h.mu.Unlock()
	for _, peer := range peers {
		_ = peer.conn.CloseNow()
	}
}

func (h *roomHub) roomPeers(roomCode string) []*socketPeer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	peers := make([]*socketPeer, 0, len(h.peers[roomCode]))
	for peer := range h.peers[roomCode] {
		peers = append(peers, peer)
	}
	return peers
}

func (h *roomHub) broadcast(roomCode string, value any) {
	for _, peer := range h.roomPeers(roomCode) {
		if err := peer.send(value); err != nil {
			_ = peer.conn.CloseNow()
			h.remove(peer)
		}
	}
}

func (p *socketPeer) send(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.conn.Write(ctx, websocket.MessageText, payload)
}

func (s *Server) roomWebSocket(w http.ResponseWriter, r *http.Request) {
	roomCode := r.PathValue("code")
	room, ok := s.rooms.Find(roomCode)
	if !ok {
		http.Error(w, "没有找到这个房间", http.StatusNotFound)
		return
	}
	playerID := r.URL.Query().Get("playerId")
	if !authorize(w, r, playerID) {
		return
	}
	snapshot := room.Snapshot()
	if !isHumanRoomPlayer(snapshot, playerID) {
		http.Error(w, "玩家不在这个房间中", http.StatusForbidden)
		return
	}
	var name string
	for _, player := range snapshot.Players {
		if player.ID == playerID {
			name = player.Name
			break
		}
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(16 << 10)
	peer := &socketPeer{roomCode: roomCode, playerID: playerID, name: name, conn: conn}
	s.hub.add(peer)
	defer func() {
		s.hub.remove(peer)
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()
	s.opMu.Lock()
	s.sendRoomState(peer, room)
	s.opMu.Unlock()

	for {
		messageType, payload, err := conn.Read(context.Background())
		if err != nil {
			return
		}
		if messageType != websocket.MessageText {
			_ = conn.Close(websocket.StatusUnsupportedData, "只接受 JSON 文本消息")
			return
		}
		var request socketRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			_ = peer.send(map[string]string{"type": "error", "error": "消息必须是合法 JSON"})
			continue
		}
		s.handleSocketMessage(peer, room, request)
	}
}

func (s *Server) handleSocketMessage(peer *socketPeer, room *table.Room, request socketRequest) {
	switch request.Type {
	case "chat":
		if !isHumanRoomPlayer(room.Snapshot(), peer.playerID) {
			_ = peer.send(map[string]string{"type": "error", "error": "你已离开此房间"})
			return
		}
		text := strings.TrimSpace(request.Text)
		if text == "" || utf8.RuneCountInString(text) > 300 {
			_ = peer.send(map[string]string{"type": "error", "error": "聊天内容需要为 1 到 300 个字符"})
			return
		}
		s.hub.broadcast(peer.roomCode, map[string]any{
			"type": "chat", "playerId": peer.playerID, "name": peer.name, "text": text,
		})
	case "action":
		s.opMu.Lock()
		defer s.opMu.Unlock()
		if len(request.Action) == 0 || !json.Valid(request.Action) {
			_ = peer.send(map[string]string{"type": "error", "error": "action 必须是一个 JSON 对象"})
			return
		}
		engine, ok := s.findEngine(peer.roomCode)
		if !ok {
			_ = peer.send(map[string]string{"type": "error", "error": "该房间的对局尚未开始"})
			return
		}
		if !isHumanRoomPlayer(room.Snapshot(), peer.playerID) {
			_ = peer.send(map[string]string{"type": "error", "error": "你已离开此房间"})
			return
		}
		if _, err := s.applyLocked(peer.roomCode, room, engine, peer.playerID, request.Action); err != nil {
			_ = peer.send(map[string]string{"type": "error", "error": err.Error()})
			return
		}
		s.broadcastGameState(peer.roomCode, room, engine)
	default:
		_ = peer.send(map[string]string{"type": "error", "error": "消息类型必须是 chat 或 action"})
	}
}

func (s *Server) sendRoomState(peer *socketPeer, room *table.Room) {
	if engine, ok := s.findEngine(peer.roomCode); ok {
		_ = peer.send(s.stateEnvelope(room, engine, peer.playerID))
		return
	}
	_ = peer.send(map[string]any{"type": "room", "data": room.Snapshot()})
}

func (s *Server) broadcastRoom(room *table.Room) {
	for _, peer := range s.hub.roomPeers(room.Snapshot().Code) {
		s.sendRoomState(peer, room)
	}
}

func (s *Server) broadcastGameState(roomCode string, room *table.Room, engine games.Engine) {
	for _, peer := range s.hub.roomPeers(roomCode) {
		_ = peer.send(s.stateEnvelope(room, engine, peer.playerID))
	}
}
