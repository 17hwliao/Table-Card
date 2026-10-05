package audio

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/17hwliao/table-card-independent/internal/table"
)

type Track struct {
	Mode     table.Mode `json:"mode"`
	File     string     `json:"file"`
	Title    string     `json:"title"`
	Author   string     `json:"author"`
	License  string     `json:"license"`
	Source   string     `json:"source"`
	Download string     `json:"download"`
}

//go:embed catalog.json
var catalogJSON []byte

var catalog map[table.Mode]Track

func init() {
	var entries []Track
	if err := json.Unmarshal(catalogJSON, &entries); err != nil {
		panic(err)
	}
	catalog = make(map[table.Mode]Track, len(entries))
	files := make(map[string]bool, len(entries))
	for _, track := range entries {
		if _, duplicate := catalog[track.Mode]; duplicate || files[track.File] || track.Mode == "" ||
			track.Title == "" || track.Author == "" || track.License != "CC0-1.0" ||
			strings.ContainsAny(track.File, `/\`) || !strings.HasSuffix(track.File, ".mp3") {
			panic("invalid or duplicate mode music")
		}
		asset, err := tracks.Open(track.File)
		if err != nil {
			panic(err)
		}
		info, err := asset.Stat()
		_ = asset.Close()
		if err != nil || info.IsDir() || info.Size() == 0 {
			panic("empty or invalid music asset: " + track.File)
		}
		catalog[track.Mode] = track
		files[track.File] = true
	}
	for _, mode := range []table.Mode{table.LandlordMode, table.LiarBarMode, table.MahjongMode,
		table.ChessMode, table.WesternChessMode, table.GomokuMode, table.GoMode,
		table.UNOMode, table.TetrisMode, table.SnakeMode, table.PokemonMode} {
		if _, ok := catalog[mode]; !ok {
			panic("missing mode music: " + string(mode))
		}
	}
}

// TrackForMode keeps the game chooser on the tavern theme and gives every
// actual mode a separate recording. It does not open or decode any audio.
func TrackForMode(mode table.Mode) Track {
	if track, ok := catalog[mode]; ok {
		return track
	}
	return catalog[table.LiarBarMode]
}

func (p *Player) Track() Track {
	p.mu.Lock()
	defer p.mu.Unlock()
	return TrackForMode(p.mode)
}
