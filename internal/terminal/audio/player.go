package audio

import (
	"embed"
	"errors"
	"github.com/17hwliao/table-card-independent/internal/table"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"io"
	"math"
	"sync"
	"time"
)

//go:embed *.mp3
var tracks embed.FS

const outputRate beep.SampleRate = 44100

type Player struct {
	mu          sync.Mutex
	mode        table.Mode
	muted       bool
	initialized bool
	stream      beep.StreamSeekCloser
}

func New() *Player            { return &Player{muted: true} }
func (p *Player) Muted() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.muted }
func (p *Player) SetMode(mode table.Mode) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if mode == p.mode {
		return nil
	}
	p.mode = mode
	if !p.muted {
		return p.play()
	}
	return nil
}
func (p *Player) Toggle() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.initialized {
		if err := speaker.Init(outputRate, outputRate.N(80*time.Millisecond)); err != nil {
			return true, err
		}
		p.initialized = true
	}
	p.muted = !p.muted
	if p.muted {
		p.stop()
		return true, nil
	}
	if err := p.play(); err != nil {
		p.muted = true
		return true, err
	}
	return false, nil
}
func (p *Player) stop() {
	if !p.initialized {
		return
	}
	speaker.Clear()
	if p.stream != nil {
		_ = p.stream.Close()
		p.stream = nil
	}
}
func (p *Player) play() error {
	p.stop()
	file := "liar-bar.mp3"
	switch p.mode {
	case table.MahjongMode:
		file = "mahjong.mp3"
	case table.GoMode:
		file = "go.mp3"
	case table.ChessMode, table.WesternChessMode:
		file = "chess.mp3"
	}
	asset, err := tracks.Open(file)
	if err != nil {
		return err
	}
	if _, ok := asset.(io.Seeker); !ok {
		_ = asset.Close()
		return errors.New("内置音乐不支持循环播放")
	}
	// Open reads directly from the embedded file instead of allocating a
	// separate multi-megabyte byte slice in every client process.
	stream, format, err := mp3.Decode(asset)
	if err != nil {
		_ = asset.Close()
		return err
	}
	p.stream = stream
	loop := beep.Loop(-1, stream)
	speaker.Play(&effects.Volume{Streamer: beep.Resample(4, format.SampleRate, outputRate, loop), Base: 2, Volume: -2})
	return nil
}

// Effect synthesizes a short tile or move sound; seed gives every tile a distinct tone.
func (p *Player) Effect(seed int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.muted || !p.initialized {
		return
	}
	n, total := 0, outputRate.N(75*time.Millisecond)
	frequency := 360 + float64(seed%27)*24
	speaker.Play(beep.StreamerFunc(func(samples [][2]float64) (int, bool) {
		count := 0
		for i := range samples {
			if n >= total {
				return count, false
			}
			t := float64(n) / float64(outputRate)
			decay := math.Exp(-float64(n) * 8 / float64(total))
			v := .12 * decay * (math.Sin(2*math.Pi*frequency*t) + .35*math.Sin(2*math.Pi*frequency*2.7*t))
			samples[i] = [2]float64{v, v}
			n++
			count++
		}
		return count, true
	}))
}
func (p *Player) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stop()
	if p.initialized {
		speaker.Close()
		p.initialized = false
	}
}
