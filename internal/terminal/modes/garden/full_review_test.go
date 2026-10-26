package garden

import (
	"encoding/json"
	"strings"
	"testing"

	game "github.com/17hwliao/table-card-independent/internal/garden"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
	"github.com/charmbracelet/x/ansi"
)

func TestReviewMouseCardsAndLawnMatchRenderedRows(t *testing.T) {
	for _, width := range []int{40, 48, 60, 68, 89, 120, 160} {
		for _, height := range []int{25, 30, 40, 60} {
			v := game.Snapshot{Started: true, Players: []game.Lawn{{ID: "a", Sun: 999}, {ID: "b", Sun: 999}}}
			raw, _ := json.Marshal(v)
			s := ui.Snapshot{PlayerID: "a", Game: raw, Width: width, Height: height}
			c := New()
			g := geometry(s)
			lines := strings.Split(ansi.Strip(c.View(s)), "\n")
			for n := 1; n <= 10; n++ {
				x, y := ((n-1)%g.cardColumns)*cardStride, 2+(n-1)/g.cardColumns
				if !strings.Contains(lines[y], abbreviations[n]) {
					t.Fatalf("card row mismatch %dx%d card%d", width, height, n)
				}
				c.Mouse(s, x, y)
				if int(c.selected) != n {
					t.Fatal("card click missed")
				}
			}
			if !strings.HasPrefix(lines[g.boardTop], "│") {
				t.Fatal("board row mismatch")
			}
			for row := 0; row < game.Rows; row++ {
				for col := 0; col < game.Cols; col++ {
					r := c.Mouse(s, 1+col*g.cellWidth, g.boardTop+row*g.cellHeight)
					var a game.Action
					if json.Unmarshal(r.Action, &a) != nil || a.Row != row || a.Col != col || a.Plant != game.Squash {
						t.Fatalf("wrong click %dx%d [%d,%d]: %s", width, height, row, col, r.Action)
					}
				}
			}
			c.Key(s, "tab")
			if len(c.Mouse(s, 1, g.boardTop).Action) != 0 {
				t.Fatal("opponent lawn is writable")
			}
		}
	}
}
