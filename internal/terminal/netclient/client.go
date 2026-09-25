package netclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/coder/websocket"
)

type Client struct {
	baseURL string
	http    *http.Client
	stateMu sync.RWMutex
	socket  *websocket.Conn
	ctx     context.Context
	cancel  context.CancelFunc
	writeMu sync.Mutex
}

type Envelope struct {
	Type     string          `json:"type"`
	Data     json.RawMessage `json:"data"`
	Room     json.RawMessage `json:"room"`
	Game     json.RawMessage `json:"game"`
	Text     string          `json:"text"`
	Name     string          `json:"name"`
	PlayerID string          `json:"playerId"`
	Error    string          `json:"error"`
}

func New(address string) *Client {
	address = strings.TrimSpace(address)
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	return &Client{baseURL: strings.TrimRight(address, "/"), http: &http.Client{Timeout: 8 * time.Second}}
}

func (c *Client) Modes(ctx context.Context) ([]struct {
	ID       table.Mode `json:"id"`
	Name     string     `json:"name"`
	MinSeats int        `json:"minSeats"`
	MaxSeats int        `json:"maxSeats"`
	Progress string     `json:"progress"`
}, error) {
	var modes []struct {
		ID       table.Mode `json:"id"`
		Name     string     `json:"name"`
		MinSeats int        `json:"minSeats"`
		MaxSeats int        `json:"maxSeats"`
		Progress string     `json:"progress"`
	}
	err := c.request(ctx, http.MethodGet, "/api/modes", nil, &modes)
	return modes, err
}

func (c *Client) CreateRoom(ctx context.Context, mode table.Mode, seats, bots int, player table.Player) (table.Snapshot, error) {
	var room table.Snapshot
	err := c.request(ctx, http.MethodPost, "/api/rooms", map[string]any{
		"mode": mode, "seats": seats, "bots": bots, "playerId": player.ID, "name": player.Name,
	}, &room)
	return room, err
}

func (c *Client) JoinRoom(ctx context.Context, code string, player table.Player) (table.Snapshot, error) {
	var room table.Snapshot
	err := c.request(ctx, http.MethodPost, "/api/rooms/"+url.PathEscape(code)+"/join", player, &room)
	return room, err
}

func (c *Client) SetReady(ctx context.Context, code, playerID string, ready bool) (table.Snapshot, error) {
	var room table.Snapshot
	err := c.request(ctx, http.MethodPost, "/api/rooms/"+url.PathEscape(code)+"/ready", map[string]any{
		"playerId": playerID, "ready": ready,
	}, &room)
	return room, err
}

func (c *Client) StartRoom(ctx context.Context, code string) (table.Snapshot, error) {
	var room table.Snapshot
	err := c.request(ctx, http.MethodPost, "/api/rooms/"+url.PathEscape(code)+"/start", map[string]any{}, &room)
	return room, err
}

func (c *Client) Connect(ctx context.Context, code, playerID string) error {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	if base.Scheme == "https" {
		base.Scheme = "wss"
	} else {
		base.Scheme = "ws"
	}
	base.Path = "/api/rooms/" + url.PathEscape(code) + "/ws"
	query := base.Query()
	query.Set("playerId", playerID)
	base.RawQuery = query.Encode()
	conn, _, err := websocket.Dial(ctx, base.String(), nil)
	if err != nil {
		return err
	}
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
	if c.socket != nil {
		_ = c.socket.Close(websocket.StatusNormalClosure, "reconnect")
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.socket = conn
	return nil
}

func (c *Client) Read() (Envelope, error) {
	var envelope Envelope
	c.stateMu.RLock()
	conn, connectionContext := c.socket, c.ctx
	c.stateMu.RUnlock()
	if conn == nil || connectionContext == nil {
		return envelope, fmt.Errorf("尚未连接实时房间")
	}
	_, data, err := conn.Read(connectionContext)
	if err != nil {
		return envelope, err
	}
	err = json.Unmarshal(data, &envelope)
	return envelope, err
}

func (c *Client) Send(value any) error {
	c.stateMu.RLock()
	conn, connectionContext := c.socket, c.ctx
	c.stateMu.RUnlock()
	if conn == nil || connectionContext == nil {
		return fmt.Errorf("尚未连接实时房间")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(connectionContext, 5*time.Second)
	defer cancel()
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.Write(ctx, websocket.MessageText, data)
}

func (c *Client) Close() {
	c.stateMu.Lock()
	cancel, conn := c.cancel, c.socket
	c.cancel, c.socket, c.ctx = nil, nil, nil
	c.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "client left room")
	}
}

func (c *Client) request(ctx context.Context, method, path string, body, target any) error {
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var message struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(response.Body).Decode(&message)
		if message.Error == "" {
			message.Error = response.Status
		}
		return fmt.Errorf("%s", message.Error)
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(target)
}
