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
	closed   bool
	playerID string
	token    string
	baseURL  string
	http     *http.Client
	stateMu  sync.RWMutex
	socket   *websocket.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	writeMu  sync.Mutex
}

func (c *Client) Identity(id, token string) { c.playerID = id; c.token = token }
func (c *Client) Leave(ctx context.Context, code, id string) error {
	return c.request(ctx, http.MethodPost, "/api/rooms/"+url.PathEscape(code)+"/leave", map[string]string{"playerId": id}, nil)
}
func (c *Client) Rematch(ctx context.Context, code, id string) error {
	return c.request(ctx, http.MethodPost, "/api/rooms/"+url.PathEscape(code)+"/rematch", map[string]string{"playerId": id}, nil)
}
func (c *Client) Stats(ctx context.Context, mode table.Mode, id string) (json.RawMessage, error) {
	var data json.RawMessage
	err := c.request(ctx, http.MethodGet, "/api/stats?mode="+url.QueryEscape(string(mode))+"&playerId="+url.QueryEscape(id), nil, &data)
	return data, err
}
func (c *Client) Rooms(ctx context.Context, mode table.Mode) ([]table.Snapshot, error) {
	var data []table.Snapshot
	err := c.request(ctx, http.MethodGet, "/api/rooms?mode="+url.QueryEscape(string(mode)), nil, &data)
	return data, err
}
func (c *Client) Match(ctx context.Context, mode table.Mode, p table.Player, seats ...int) (table.Snapshot, error) {
	var room table.Snapshot
	request := map[string]any{"mode": mode, "playerId": p.ID, "name": p.Name}
	if len(seats) > 0 {
		request["seats"] = seats[0]
	}
	err := c.request(ctx, http.MethodPost, "/api/match", request, &room)
	return room, err
}

type Envelope struct {
	PauseUntil int64           `json:"pauseUntil"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data"`
	Room       json.RawMessage `json:"room"`
	Game       json.RawMessage `json:"game"`
	Text       string          `json:"text"`
	Name       string          `json:"name"`
	PlayerID   string          `json:"playerId"`
	Error      string          `json:"error"`
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
	var health struct{ Service, Protocol, RulesVersion string }
	if err := c.request(ctx, http.MethodGet, "/api/health", nil, &health); err != nil {
		return nil, err
	}
	if health.Service != "table-card" || health.Protocol != "2" || health.RulesVersion != table.RulesVersion {
		return nil, fmt.Errorf("服务端版本与当前客户端不匹配，请房主更新本次维护版并重启服务端")
	}
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

func (c *Client) CreateRoom(ctx context.Context, mode table.Mode, seats, bots int, player table.Player, options ...json.RawMessage) (table.Snapshot, error) {
	var room table.Snapshot
	var config json.RawMessage
	if len(options) > 0 {
		config = options[0]
	}
	err := c.request(ctx, http.MethodPost, "/api/rooms", map[string]any{
		"mode": mode, "seats": seats, "bots": bots, "playerId": player.ID, "name": player.Name, "options": config,
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
	header := http.Header{}
	header.Set("X-Player-Token", c.token)
	conn, _, err := websocket.Dial(ctx, base.String(), &websocket.DialOptions{HTTPHeader: header, HTTPClient: c.http})
	if err != nil {
		return err
	}
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.closed {
		_ = conn.CloseNow()
		return fmt.Errorf("连接已关闭")
	}
	if c.cancel != nil {
		c.cancel()
	}
	if c.socket != nil {
		_ = c.socket.Close(websocket.StatusNormalClosure, "reconnect")
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
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
	c.closed = true
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
	req.Header.Set("X-Player-Token", c.token)
	req.Header.Set("X-Player-ID", c.playerID)
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
