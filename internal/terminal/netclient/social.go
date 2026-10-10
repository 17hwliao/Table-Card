package netclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/coder/websocket"
)

type OnlineUser struct {
	PlayerID  string     `json:"playerId"`
	Name      string     `json:"name"`
	Mode      table.Mode `json:"mode"`
	RoomCode  string     `json:"roomCode,omitempty"`
	Status    string     `json:"status"`
	Ready     bool       `json:"ready"`
	Host      bool       `json:"host"`
	Connected bool       `json:"connected"`
}

type ConnectionInfo struct {
	Addresses []struct {
		Address   string `json:"address"`
		Interface string `json:"interface"`
	} `json:"addresses"`
}

func (c *Client) ConnectionInfo(ctx context.Context) (ConnectionInfo, error) {
	var info ConnectionInfo
	err := c.request(ctx, http.MethodGet, "/api/connection-info", nil, &info)
	return info, err
}

// SocialConnect maintains a separate stream from room state. Close only leaves
// the room stream; SocialClose must be called when the application exits.
func (c *Client) SocialConnect(ctx context.Context, playerID, name string, mode table.Mode) error {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	if base.Scheme == "https" {
		base.Scheme = "wss"
	} else {
		base.Scheme = "ws"
	}
	base.Path = "/api/social/ws"
	query := base.Query()
	query.Set("playerId", playerID)
	query.Set("name", name)
	query.Set("mode", string(mode))
	base.RawQuery = query.Encode()
	header := http.Header{}
	header.Set("X-Player-Token", c.token)
	conn, _, err := websocket.Dial(ctx, base.String(), &websocket.DialOptions{HTTPHeader: header, HTTPClient: c.http})
	if err != nil {
		return err
	}
	conn.SetReadLimit((2 << 20) + (16 << 10))
	c.stateMu.Lock()
	oldConn, oldCancel := c.socialSocket, c.socialCancel
	c.socialCtx, c.socialCancel = context.WithCancel(context.Background())
	c.socialSocket = conn
	c.stateMu.Unlock()
	if oldCancel != nil {
		oldCancel()
	}
	if oldConn != nil {
		_ = oldConn.CloseNow()
	}
	return nil
}

func (c *Client) SocialRead() (Envelope, error) {
	var envelope Envelope
	c.stateMu.RLock()
	conn, ctx := c.socialSocket, c.socialCtx
	c.stateMu.RUnlock()
	if conn == nil || ctx == nil {
		return envelope, fmt.Errorf("尚未连接服务器社交频道")
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		return envelope, err
	}
	err = json.Unmarshal(data, &envelope)
	return envelope, err
}

func (c *Client) SocialSend(value any) error {
	c.stateMu.RLock()
	conn, connectionContext := c.socialSocket, c.socialCtx
	c.stateMu.RUnlock()
	if conn == nil || connectionContext == nil {
		return fmt.Errorf("尚未连接服务器社交频道")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(connectionContext, 5*time.Second)
	defer cancel()
	c.socialWriteMu.Lock()
	defer c.socialWriteMu.Unlock()
	return conn.Write(ctx, websocket.MessageText, data)
}

func (c *Client) SocialClose() {
	c.stateMu.Lock()
	conn, cancel := c.socialSocket, c.socialCancel
	c.socialSocket, c.socialCtx, c.socialCancel = nil, nil, nil
	c.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.CloseNow()
	}
}
