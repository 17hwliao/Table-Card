package ui

import (
	"encoding/json"

	"github.com/17hwliao/table-card-independent/internal/table"
)

// Snapshot is the read-only state passed to a mode-specific terminal controller.
type Snapshot struct {
	Mode     table.Mode
	Room     table.Snapshot
	PlayerID string
	Game     json.RawMessage
	Width    int
	Height   int
}

// Result describes what a key did. Action is sent through the room action API.
type Result struct {
	Handled bool
	Action  json.RawMessage
	Status  string
	Exit    bool
}

// Controller owns mode-specific selection state, rendering, and key bindings.
type Controller interface {
	Mode() table.Mode
	Reset()
	View(Snapshot) string
	Key(Snapshot, string) Result
}

func Action(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}
