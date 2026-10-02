package audio

import (
	"io"
	"testing"

	"github.com/gopxl/beep/v2/mp3"
)

func TestEmbeddedTracksCanStreamAndLoop(t *testing.T) {
	for _, name := range []string{"liar-bar.mp3", "mahjong.mp3", "go.mp3", "chess.mp3"} {
		t.Run(name, func(t *testing.T) {
			asset, err := tracks.Open(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := asset.(io.Seeker); !ok {
				_ = asset.Close()
				t.Fatal("embedded file cannot seek for looping playback")
			}
			stream, _, err := mp3.Decode(asset)
			if err != nil {
				_ = asset.Close()
				t.Fatal(err)
			}
			defer stream.Close()
			samples := make([][2]float64, 128)
			if n, _ := stream.Stream(samples); n == 0 {
				t.Fatal("audio stream produced no samples")
			}
			if err := stream.Seek(0); err != nil {
				t.Fatal(err)
			}
		})
	}
}
