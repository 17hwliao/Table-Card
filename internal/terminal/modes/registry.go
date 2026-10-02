package modes

import (
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/17hwliao/table-card-independent/internal/terminal/modes/board"
	"github.com/17hwliao/table-card-independent/internal/terminal/modes/cards"
	"github.com/17hwliao/table-card-independent/internal/terminal/modes/mahjong"
	"github.com/17hwliao/table-card-independent/internal/terminal/modes/tetris"
	"github.com/17hwliao/table-card-independent/internal/terminal/ui"
)

func New(mode table.Mode) ui.Controller {
	var controller ui.Controller
	switch mode {
	case table.TetrisMode:
		controller = tetris.New()
	case table.LandlordMode:
		controller = cards.NewLandlord()
	case table.LiarBarMode:
		controller = cards.NewLiarBar()
	case table.UNOMode:
		controller = cards.NewUNO()
	case table.MahjongMode:
		controller = mahjong.New()
	case table.ChessMode:
		controller = &board.ChineseChess{}
	case table.WesternChessMode:
		controller = &board.InternationalChess{}
	case table.GomokuMode:
		controller = &board.Gomoku{}
	case table.GoMode:
		controller = &board.Go{}
	}
	if controller != nil {
		controller.Reset()
	}
	return controller
}
