package app

import (
	"encoding/json"
	"os"
	"runtime"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/17hwliao/table-card-independent/internal/games"
	"github.com/17hwliao/table-card-independent/internal/memory"
	"github.com/17hwliao/table-card-independent/internal/pokemon"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/modes"
)

var benchmarkView tea.View

func memoryScene(t testing.TB, mode string) *Model {
	t.Helper()
	m := New("", "memory benchmark")
	m.width, m.height = 120, 40
	switch mode {
	case "home":
		return m
	case "snake":
		m.startSnake()
		return m
	case "pokemon":
		m.startPokemon()
		for i := 0; i < 120; i++ {
			m.pokemonGame.Notice("远行的训练者回到了城镇，与伙伴一起整理背包和旅行记录，准备迎接下一场挑战。")
		}
		return m
	}
	registry := games.NewDefaultRegistry()
	id := table.Mode(mode)
	seats := 2
	if id == table.LandlordMode {
		seats = 3
	}
	if id == table.MahjongMode || id == table.LiarBarMode || id == table.UNOMode || id == table.TetrisMode {
		seats = 4
	}
	players := make([]table.Player, seats)
	for i := range players {
		players[i] = table.Player{ID: string(rune('a' + i)), Name: "训练者"}
	}
	engine, err := registry.New(id, players)
	if err != nil {
		t.Fatal(err)
	}
	m.page = gameScreen
	m.player = players[0]
	m.room = table.Snapshot{Mode: id, Code: "MEMORY", Seats: seats, Players: players, Phase: table.InProgress}
	m.control = modes.New(id)
	m.game, err = json.Marshal(engine.View("a"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func BenchmarkRenderMode(b *testing.B) {
	b.Setenv("APPDATA", b.TempDir())
	for _, mode := range []string{"home", "landlord", "liar_bar", "mahjong", "chess", "western_chess", "gomoku", "go", "uno", "tetris", "snake", "pokemon"} {
		b.Run(mode, func(b *testing.B) {
			m := memoryScene(b, mode)
			defer m.Close()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkView = m.View()
			}
		})
	}
}

// The PowerShell harness observes this process from outside, without a terminal
// host, race instrumentation, or Go's compiler in the working-set measurement.
func TestMemoryProcessHelper(t *testing.T) {
	mode := os.Getenv("TABLE_CARD_MEMORY_SCENE")
	if mode == "" {
		t.Skip("used only by scripts/measure-memory.ps1")
	}
	t.Setenv("APPDATA", t.TempDir())
	memory.Client()
	m := memoryScene(t, mode)
	defer m.Close()
	for i := 0; i < 20; i++ {
		benchmarkView = m.View()
	}
	runtime.GC()
	if err := os.WriteFile(os.Getenv("TABLE_CARD_MEMORY_READY"), []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	end := time.Now().Add(8 * time.Second)
	for time.Now().Before(end) {
		benchmarkView = m.View()
		time.Sleep(20 * time.Millisecond)
	}
	runtime.KeepAlive(m)
}
func TestAllTerminalModesRenderAndExit(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	for _, mode := range []string{"home", "landlord", "liar_bar", "mahjong", "chess", "western_chess", "gomoku", "go", "uno", "tetris", "snake", "pokemon"} {
		t.Run(mode, func(t *testing.T) {
			m := memoryScene(t, mode)
			defer m.Close()
			for _, size := range [][2]int{{120, 40}, {80, 30}, {60, 24}} {
				m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				if m.View().Content == "" {
					t.Fatal("empty terminal view")
				}
			}
			if mode != "home" {
				m.key(tea.KeyPressMsg(tea.Key{Code: tea.KeyDelete}))
				if m.page != homeScreen || m.game != nil || m.control != nil || m.room.Code != "" || m.snakeGame != nil || m.pokemonGame != nil || m.pokemonCache.game != nil {
					t.Fatal("Del retained the previous game state")
				}
			}
		})
	}
}
func TestPokemonInputIsolationAndStaleTimer(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	m := NewPokemon("input test")
	defer m.Close()
	m.key(tea.KeyPressMsg(tea.Key{Code: 'm', Text: "m"}))
	if m.pokemonInput.Value() != "m" || !m.sound.Muted() {
		t.Fatal("typing m toggled music or failed to enter text")
	}
	m.pokemonGame.Party = []pokemon.Monster{}
	if err := m.pokemonGame.Command("选择 杰尼龟"); err != nil {
		t.Fatal(err)
	}
	if err := m.pokemonGame.Command("进草丛"); err != nil {
		t.Fatal(err)
	}
	if err := m.pokemonGame.Command("捕捉 精灵球"); err != nil {
		t.Fatal(err)
	}
	m.pokemonTick(pokemonTickMsg{m.pokemonGeneration - 1, 0})
	if m.pokemonGame.Capture.Step != 0 {
		t.Fatal("stale timer advanced capture")
	}
	m.key(tea.KeyPressMsg(tea.Key{Code: tea.KeyDelete}))
	if m.page != homeScreen || m.pokemonGame != nil {
		t.Fatal("Del did not leave local mode")
	}
	m.pokemonTick(pokemonTickMsg{m.pokemonGeneration - 1, 0})
}
func TestPokemonRenderCacheInvalidatesAndReleasesGame(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	m := NewPokemon("cache test")
	defer m.Close()
	before := m.pokemonView()
	m.pokemonGame.Notice("缓存失效检查")
	after := m.pokemonView()
	if before == after {
		t.Fatal("new log entry hidden by render cache")
	}
	oldWidth := m.pokemonCache.width
	m.Update(tea.WindowSizeMsg{Width: 70, Height: 24})
	m.pokemonView()
	if m.pokemonCache.width == oldWidth {
		t.Fatal("resize retained old wrapping")
	}
	m.key(tea.KeyPressMsg(tea.Key{Code: tea.KeyDelete}))
	if m.pokemonCache.game != nil || m.pokemonCache.lines != nil {
		t.Fatal("closed adventure retained in cache")
	}
}
