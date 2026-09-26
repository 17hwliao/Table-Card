package server

import (
	"encoding/json"
	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/mahjong"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/uno"
)

func (s *Server) configuredEngine(room table.Snapshot) (games.Engine, error) {
	switch room.Mode {
	case table.MahjongMode:
		options := mahjong.DefaultOptions()
		if len(room.Options) > 0 {
			if err := json.Unmarshal(room.Options, &options); err != nil {
				return nil, err
			}
		}
		return mahjong.NewEngineWithOptions(room.Players, options)
	case table.UNOMode:
		options := uno.DefaultOptions()
		if len(room.Options) > 0 {
			if err := json.Unmarshal(room.Options, &options); err != nil {
				return nil, err
			}
		}
		return uno.NewEngineWithOptions(room.Players, options)
	default:
		return s.registry.New(room.Mode, room.Players)
	}
}
