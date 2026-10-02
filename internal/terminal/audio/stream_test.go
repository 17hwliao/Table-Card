package audio

import (
	"github.com/gopxl/beep/v2/mp3"
	"testing"
)

func TestSequentialAudioMatchesIndexedDecoderAndRestarts(t *testing.T) {
	for _, file := range []string{"liar-bar.mp3", "mahjong.mp3", "go.mp3", "chess.mp3"} {
		t.Run(file, func(t *testing.T) {
			asset, _ := tracks.Open(file)
			reference, format, err := mp3.Decode(asset)
			if err != nil {
				t.Fatal(err)
			}
			defer reference.Close()
			loop, gotFormat, err := openLoopingTrack(file)
			if err != nil {
				t.Fatal(err)
			}
			defer loop.Close()
			if gotFormat != format || loop.current.Len() != 0 {
				t.Fatal("format changed or whole-file index still built")
			}
			var expected, actual [2048][2]float64
			reference.Stream(expected[:])
			loop.Stream(actual[:])
			if expected != actual {
				t.Fatal("decoded audio samples changed")
			}
			_ = loop.current.Close()
			loop.current = &drainedTrack{}
			reference.Seek(0)
			reference.Stream(expected[:])
			n, ok := loop.Stream(actual[:])
			if !ok || n != len(actual) || expected != actual {
				t.Fatal("loop did not restart with identical audio")
			}
			loop.Close()
			if n, ok := loop.Stream(actual[:]); n != 0 || ok {
				t.Fatal("closed track kept playing")
			}
		})
	}
}

type drainedTrack struct{}

func (*drainedTrack) Stream([][2]float64) (int, bool) { return 0, false }
func (*drainedTrack) Err() error                      { return nil }
func (*drainedTrack) Close() error                    { return nil }
func (*drainedTrack) Len() int                        { return 0 }
func (*drainedTrack) Position() int                   { return 0 }
func (*drainedTrack) Seek(int) error                  { return nil }
func BenchmarkOpenMusic(b *testing.B) {
	b.Run("indexed-reference", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			asset, _ := tracks.Open("liar-bar.mp3")
			s, _, err := mp3.Decode(asset)
			if err != nil {
				b.Fatal(err)
			}
			s.Close()
		}
	})
	b.Run("sequential", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			s, _, err := openLoopingTrack("liar-bar.mp3")
			if err != nil {
				b.Fatal(err)
			}
			s.Close()
		}
	})
}
