package games

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/17hwliao/table-card-independent/internal/chinesechess"
	"github.com/17hwliao/table-card-independent/internal/goengine"
	"github.com/17hwliao/table-card-independent/internal/gomoku"
	"github.com/17hwliao/table-card-independent/internal/internationalchess"
	"github.com/17hwliao/table-card-independent/internal/mahjong"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/tetris"
	"github.com/17hwliao/table-card-independent/internal/uno"
)

var ErrModeUnavailable = errors.New("该游戏模式的对局引擎尚未接入")

type Engine interface {
	Mode() table.Mode
	View(viewerID string) any
	Apply(playerID string, action json.RawMessage) (any, error)
}

type Factory func(players []table.Player) (Engine, error)

type Registry struct {
	mu        sync.RWMutex
	factories map[table.Mode]Factory
}

func NewRegistry() *Registry {
	return &Registry{factories: make(map[table.Mode]Factory)}
}

func (r *Registry) Register(mode table.Mode, factory Factory) error {
	if mode == "" || factory == nil {
		return errors.New("注册游戏模式需要模式编号和创建器")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.factories[mode]; exists {
		return fmt.Errorf("游戏模式 %q 已注册", mode)
	}
	r.factories[mode] = factory
	return nil
}

func (r *Registry) New(mode table.Mode, players []table.Player) (Engine, error) {
	r.mu.RLock()
	factory, exists := r.factories[mode]
	r.mu.RUnlock()
	if !exists {
		return nil, ErrModeUnavailable
	}
	return factory(append([]table.Player(nil), players...))
}

func (r *Registry) Available(mode table.Mode) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.factories[mode]
	return exists
}

func NewDefaultRegistry() *Registry {
	registry := NewRegistry()
	_ = registry.Register(table.TetrisMode, func(players []table.Player) (Engine, error) {
		return tetris.NewEngine(players)
	})
	_ = registry.Register(table.LandlordMode, newLandlordEngine)
	_ = registry.Register(table.LiarBarMode, newLiarBarEngine)
	_ = registry.Register(table.GomokuMode, func(players []table.Player) (Engine, error) {
		return gomoku.NewEngine(players)
	})
	_ = registry.Register(table.ChessMode, func(players []table.Player) (Engine, error) {
		return chinesechess.NewEngine(players)
	})
	_ = registry.Register(table.WesternChessMode, func(players []table.Player) (Engine, error) {
		return internationalchess.NewEngine(players)
	})
	_ = registry.Register(table.GoMode, func(players []table.Player) (Engine, error) {
		return goengine.NewEngine(players)
	})
	_ = registry.Register(table.UNOMode, func(players []table.Player) (Engine, error) {
		return uno.NewEngine(players)
	})
	_ = registry.Register(table.MahjongMode, func(players []table.Player) (Engine, error) {
		return mahjong.NewEngine(players)
	})
	return registry
}
