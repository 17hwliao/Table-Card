package table

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

type Mode string

const (
	LandlordMode     Mode = "landlord"
	LiarBarMode      Mode = "liar_bar"
	MahjongMode      Mode = "mahjong"
	ChessMode        Mode = "chess"
	WesternChessMode Mode = "western_chess"
	GomokuMode       Mode = "gomoku"
	GoMode           Mode = "go"
	UNOMode          Mode = "uno"
	TetrisMode       Mode = "tetris"
)

type RoomPhase uint8

const (
	Waiting RoomPhase = iota
	InProgress
	Closed
)

var (
	ErrRoomFull        = errors.New("房间已满")
	ErrRoomStarted     = errors.New("牌局已经开始")
	ErrRoomNotReady    = errors.New("所有玩家准备后才能开始")
	ErrPlayerMissing   = errors.New("玩家不在房间内")
	ErrDuplicatePlayer = errors.New("玩家已经在房间内")
)

type Player struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
	Bot   bool   `json:"bot,omitempty"`
}

type Snapshot struct {
	Options json.RawMessage `json:"options,omitempty"`
	Code    string          `json:"code"`
	Mode    Mode            `json:"mode"`
	Phase   RoomPhase       `json:"phase"`
	Seats   int             `json:"seats"`
	Players []Player        `json:"players"`
}

type Room struct {
	options json.RawMessage
	mu      sync.RWMutex
	code    string
	mode    Mode
	phase   RoomPhase
	seats   int
	players []Player
}

type RoomManager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

func NewRoomManager() *RoomManager {
	return &RoomManager{rooms: make(map[string]*Room)}
}

func (m *RoomManager) Create(mode Mode, seats int, owner Player) (*Room, error) {
	if seats < 2 || seats > 4 {
		return nil, fmt.Errorf("座位数必须在 2 到 4 之间")
	}
	if owner.ID == "" || owner.Name == "" {
		return nil, fmt.Errorf("创建房间需要有效玩家身份")
	}
	for {
		code, err := newRoomCode()
		if err != nil {
			return nil, err
		}
		room := &Room{code: code, mode: mode, phase: Waiting, seats: seats, players: []Player{owner}}
		m.mu.Lock()
		if _, exists := m.rooms[code]; !exists {
			m.rooms[code] = room
			m.mu.Unlock()
			return room, nil
		}
		m.mu.Unlock()
	}
}

func (m *RoomManager) Find(code string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.rooms[code]
	return room, ok
}

func (m *RoomManager) Remove(code string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rooms, code)
}

func (r *Room) Join(player Player) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.phase != Waiting {
		return ErrRoomStarted
	}
	if player.ID == "" || player.Name == "" {
		return fmt.Errorf("加入房间需要有效玩家身份")
	}
	for _, current := range r.players {
		if current.ID == player.ID {
			return ErrDuplicatePlayer
		}
	}
	if len(r.players) >= r.seats {
		return ErrRoomFull
	}
	player.Ready = false
	r.players = append(r.players, player)
	return nil
}

func (r *Room) Leave(playerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.phase != Waiting {
		return ErrRoomStarted
	}
	for i, player := range r.players {
		if player.ID == playerID {
			r.players = append(r.players[:i], r.players[i+1:]...)
			if len(r.players) == 0 {
				r.phase = Closed
			}
			return nil
		}
	}
	return ErrPlayerMissing
}

func (r *Room) SetReady(playerID string, ready bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.phase != Waiting {
		return ErrRoomStarted
	}
	for i := range r.players {
		if r.players[i].ID == playerID {
			r.players[i].Ready = ready
			return nil
		}
	}
	return ErrPlayerMissing
}

func (r *Room) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.phase != Waiting {
		return ErrRoomStarted
	}
	if len(r.players) != r.seats || len(r.players) < 2 {
		return ErrRoomNotReady
	}
	for _, player := range r.players {
		if !player.Ready {
			return ErrRoomNotReady
		}
	}
	r.phase = InProgress
	return nil
}

func (r *Room) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return Snapshot{
		Options: append(json.RawMessage(nil), r.options...),
		Code:    r.code, Mode: r.mode, Phase: r.phase, Seats: r.seats,
		Players: append([]Player(nil), r.players...),
	}
}

func (m *RoomManager) List() []*Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rooms := make([]*Room, 0, len(m.rooms))
	for _, room := range m.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}

func (r *Room) SetOptions(options json.RawMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.options = append(json.RawMessage(nil), options...)
}

// TakeOver preserves the seat and hand while transferring control to a bot.
func (r *Room) TakeOver(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.players {
		if r.players[i].ID == id {
			r.players[i].Bot = true
			r.players[i].Ready = true
			return nil
		}
	}
	return ErrPlayerMissing
}

func newRoomCode() (string, error) {
	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	const codeSize = 6
	buf := make([]byte, codeSize)
	noise := make([]byte, codeSize)
	if _, err := rand.Read(noise); err != nil {
		return "", fmt.Errorf("生成房间号失败: %w", err)
	}
	for i, b := range noise {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}
