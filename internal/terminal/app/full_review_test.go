package app

import (
	"encoding/json"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/17hwliao/table-card-independent/internal/garden"
	"github.com/17hwliao/table-card-independent/internal/pokemon"
	"github.com/17hwliao/table-card-independent/internal/table"
	modeui "github.com/17hwliao/table-card-independent/internal/terminal/ui"
	"github.com/charmbracelet/x/ansi"
)

type gardenMouseCapture struct {
	modeui.Controller
	action json.RawMessage
}

func (c *gardenMouseCapture) Mouse(s modeui.Snapshot, x, y int) modeui.Result {
	r := c.Controller.(interface {
		Mouse(modeui.Snapshot, int, int) modeui.Result
	}).Mouse(s, x, y)
	c.action = r.Action
	return r
}

func TestGardenMousePlantingThroughFullFrame(t *testing.T) {
	for _, width := range []int{60, 119, 120, 160} {
		for _, height := range []int{30, 40, 50} {
			m := memoryScene(t, "garden_relay")
			var state garden.Snapshot
			if err := json.Unmarshal(m.game, &state); err != nil {
				t.Fatal(err)
			}
			state.Started = true
			state.Players[0].Sun = 999
			m.game, _ = json.Marshal(state)
			m.width, m.height = width, height
			capture := &gardenMouseCapture{Controller: m.control}
			m.control = capture
			rows := strings.Split(ansi.Strip(m.View().Content), "\n")
			cardY, cardX, top, bottom, boardWidth := -1, -1, -1, -1, 0
			for y, line := range rows {
				if i := strings.Index(line, "[2:PEA "); i >= 0 {
					cardY, cardX = y, ansi.StringWidth(line[:i])
				}
				if strings.HasPrefix(line, "┌─") {
					top = y + 1
					boardWidth = strings.Count(strings.SplitN(line, "┐", 2)[0], "─")
				}
				if strings.HasPrefix(line, "└─") {
					bottom = y
				}
			}
			if cardY < 0 || top < 0 || bottom <= top || boardWidth%garden.Cols != 0 || (bottom-top)%garden.Rows != 0 {
				t.Fatalf("missing garden geometry at %dx%d", width, height)
			}
			m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: cardX, Y: cardY})
			if !strings.Contains(m.status, "已选择豌豆射手") {
				t.Fatalf("card selection missed at %dx%d: %s", width, height, m.status)
			}
			row, col := 4, 6
			m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 1 + col*(boardWidth/garden.Cols), Y: top + row*((bottom-top)/garden.Rows)})
			var action garden.Action
			if json.Unmarshal(capture.action, &action) != nil || action.Type != "plant" || action.Plant != garden.Peashooter || action.Row != row || action.Col != col {
				t.Fatalf("wrong full-frame planting at %dx%d: %s", width, height, capture.action)
			}
			m.Close()
		}
	}
}

func TestReviewGardenFrameFitsTerminal(t *testing.T) {
	for _, width := range []int{60, 80, 90, 120, 160} {
		for _, height := range []int{30, 40, 50} {
			m := memoryScene(t, "garden_relay")
			m.width, m.height = width, height
			body := ansi.Strip(m.View().Content)
			rows := strings.Split(body, "\n")
			if len(rows) > height {
				t.Errorf("garden frame %dx%d uses %d rows", width, height, len(rows))
			}
			for i, row := range rows {
				if ansi.StringWidth(row) > width {
					t.Errorf("garden frame %dx%d row %d uses %d columns", width, height, i, ansi.StringWidth(row))
					break
				}
			}
			m.Close()
		}
	}
}

func TestAllModeFrameWidths(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	for _, mode := range []string{"home", "landlord", "liar_bar", "mahjong", "chess", "western_chess", "gomoku", "go", "uno", "tetris", "garden_relay", "snake", "pokemon"} {
		m := memoryScene(t, mode)
		for _, width := range []int{60, 80, 119, 120, 160} {
			m.width, m.height = width, 40
			for _, row := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(row) > width {
					t.Fatalf("%s frame exceeds %d columns: %d", mode, width, ansi.StringWidth(row))
				}
			}
		}
		m.Close()
	}
}

func TestSmallTerminalShowsSizeWarningAndRestoresFullGame(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	for _, mode := range []string{"landlord", "liar_bar", "mahjong", "chess", "western_chess", "gomoku", "go", "uno", "tetris", "garden_relay", "snake"} {
		m := memoryScene(t, mode)
		for _, size := range [][2]int{{60, 24}, {80, 30}, {90, 30}, {120, 30}} {
			m.width, m.height = size[0], size[1]
			view := m.View()
			if rows := strings.Count(view.Content, "\n") + 1; rows > m.height {
				t.Fatalf("%s exceeds %dx%d: %d rows", mode, m.width, m.height, rows)
			}
			if m.frameTooSmall && (!strings.Contains(view.Content, "完整界面需要至少") || view.MouseMode != tea.MouseModeNone) {
				t.Fatalf("%s hides content without size warning or mouse protection", mode)
			}
		}
		m.width, m.height = 160, 80
		view := m.View()
		if m.frameTooSmall || strings.Contains(view.Content, "终端空间不足") {
			t.Fatalf("%s did not restore after enlargement", mode)
		}
		m.Close()
	}
}

func TestSizeWarningCoversFullLobbyAndWaitingRoom(t *testing.T) {
	for _, page := range []screen{homeScreen, roomScreen} {
		m := memoryScene(t, "garden_relay")
		m.page = page
		m.width, m.height = 80, 12
		m.modes = nil
		for _, mode := range []table.Mode{table.LandlordMode, table.LiarBarMode, table.MahjongMode, table.ChessMode, table.WesternChessMode, table.GomokuMode, table.GoMode, table.UNOMode, table.TetrisMode, table.GardenMode, table.SnakeMode, table.PokemonMode} {
			m.modes = append(m.modes, modeInfo{ID: mode, Name: string(mode)})
		}
		body := m.View().Content
		if !m.frameTooSmall || !strings.Contains(body, "终端空间不足") || strings.Count(body, "\n")+1 > m.height {
			t.Fatalf("page %d has no bounded size warning", page)
		}
		m.width, m.height = 120, 50
		m.View()
		if m.frameTooSmall {
			t.Fatalf("page %d did not recover", page)
		}
		m.Close()
	}
}

func TestSizeWarningBlocksHiddenBoardMouseAndAllowsLeaving(t *testing.T) {
	m := memoryScene(t, "garden_relay")
	defer m.Close()
	probe := &mouseProbe{}
	m.control = probe
	m.width, m.height = 90, 7
	if !strings.Contains(m.View().Content, "终端空间不足") {
		t.Fatal("expected size warning")
	}
	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 3, Y: 6})
	if probe.calls != 0 {
		t.Fatal("hidden board received a click")
	}
	m.key(tea.KeyPressMsg(tea.Key{Code: tea.KeyDelete}))
	if m.page != homeScreen || m.control != nil || m.game != nil {
		t.Fatal("Del cannot leave while size warning is active")
	}
}

type mouseProbe struct {
	snapshot modeui.Snapshot
	x, y     int
	calls    int
}

func (*mouseProbe) Mode() table.Mode                          { return table.GardenMode }
func (*mouseProbe) Reset()                                    {}
func (p *mouseProbe) View(s modeui.Snapshot) string           { p.snapshot = s; return "board" }
func (*mouseProbe) Key(modeui.Snapshot, string) modeui.Result { return modeui.Result{} }
func (p *mouseProbe) Mouse(s modeui.Snapshot, x, y int) modeui.Result {
	p.snapshot = s
	p.x = x
	p.y = y
	p.calls++
	return modeui.Result{Handled: true}
}

func TestMouseUsesRenderedDimensionsAndIgnoresSidebar(t *testing.T) {
	m := memoryScene(t, "garden_relay")
	defer m.Close()
	probe := &mouseProbe{}
	m.control = probe
	for _, width := range []int{60, 119, 120, 160} {
		m.width, m.height = width, 40
		m.View()
		rendered := probe.snapshot
		m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 3, Y: 8})
		if probe.snapshot.Width != rendered.Width || probe.snapshot.Height != rendered.Height || probe.x != 3 || probe.y != 6 {
			t.Fatal("pointer geometry differs from rendered board")
		}
		before := probe.calls
		m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: rendered.Width, Y: 8})
		if probe.calls != before {
			t.Fatal("sidebar click reached the game controller")
		}
	}
}

func TestReviewPokemonDuelFrameFitsTerminal(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Full skill descriptions contain embedded newlines; exercise that view,
	// rather than assuming len(rows) equals terminal height.
	m := memoryScene(t, "pokemon")
	defer m.Close()
	m.width, m.height = 90, 30
	if err := m.pokemonGame.Command("starter 1"); err != nil {
		t.Fatal(err)
	}
	// Construct the same view data that a normal network duel sends.
	mon := m.pokemonGame.Party[0]
	mon.Level = 50
	mon.Exp = pokemon.ExperienceForSpecies(mon.Species, mon.Level)
	pokemon.SanitizeMonster(&mon)
	mon.HP = mon.MaxHP()
	d := &pokemon.DuelView{Names: [2]string{"A", "B"}, Team: []pokemon.Monster{mon, mon, mon, mon, mon, mon}, Foe: mon, Turn: 1, FoeTotal: 6, FoeRemaining: 6, Log: []string{"A派出妙蛙种子", "B使用藤鞭", "造成20伤害"}}
	m.pokemonDuel = d
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	if len(rows) > m.height {
		t.Fatalf("duel exceeds height: %d/%d", len(rows), m.height)
	}
}
