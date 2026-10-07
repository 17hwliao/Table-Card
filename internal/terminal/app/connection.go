package app

import (
	"context"
	"time"

	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/netclient"
)

// Cover the HTTP setup and WebSocket handshake with one cancelable deadline.
func (m *Model) beginRoomConnection(client *netclient.Client) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	m.connectionClient = client
	m.connectionCancel = cancel
	m.connectionCanceled = false
	m.pending = true
	return ctx, cancel
}

func finishRoomConnection(ctx context.Context, client *netclient.Client, room table.Snapshot, id string, err error) connected {
	if err == nil {
		err = client.Connect(ctx, room.Code, id)
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		cleanupRoomConnection(client, room, id)
	}
	return connected{client: client, room: room, err: err}
}

// Finish cleanup before allowing a retry with the same identity. Otherwise a
// late leave request could remove the player's newly rejoined seat.
func cleanupRoomConnection(client *netclient.Client, room table.Snapshot, id string) {
	if room.Code != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = client.Leave(ctx, room.Code, id)
		cancel()
	}
	client.Close()
}
