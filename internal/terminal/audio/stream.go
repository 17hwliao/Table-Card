package audio

import (
	"errors"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"io"
)

// Hide seeking from the decoder: indexed random access scans and allocates for
// the whole MP3. Background playback only needs sequential reads and restart.
type sequentialReader struct{ io.ReadCloser }
type loopingTrack struct {
	file       string
	current    beep.StreamSeekCloser
	format     beep.Format
	err        error
	pendingErr error
	closed     bool
}

func decodeSequential(file string) (beep.StreamSeekCloser, beep.Format, error) {
	asset, err := tracks.Open(file)
	if err != nil {
		return nil, beep.Format{}, err
	}
	stream, format, err := mp3.Decode(sequentialReader{asset})
	if err != nil {
		_ = asset.Close()
	}
	return stream, format, err
}
func openLoopingTrack(file string) (*loopingTrack, beep.Format, error) {
	s, f, e := decodeSequential(file)
	if e != nil {
		return nil, f, e
	}
	return &loopingTrack{file: file, current: s, format: f}, f, nil
}
func (l *loopingTrack) Stream(samples [][2]float64) (int, bool) {
	if l.pendingErr != nil {
		l.err = l.pendingErr
		l.pendingErr = nil
	}
	if l.closed || l.err != nil {
		return 0, false
	}
	count := 0
	reopenedWithoutProgress := false
	for count < len(samples) {
		n, ok := l.current.Stream(samples[count:])
		count += n
		if n > 0 {
			reopenedWithoutProgress = false
		}
		if ok {
			continue
		}
		if err := l.current.Err(); err != nil {
			return l.fail(count, err)
		}
		if reopenedWithoutProgress {
			return l.fail(count, io.ErrNoProgress)
		}
		_ = l.current.Close()
		l.current = nil
		s, f, err := decodeSequential(l.file)
		if err != nil {
			return l.fail(count, err)
		}
		if f != l.format {
			_ = s.Close()
			return l.fail(count, errors.New("背景曲循环时格式发生变化"))
		}
		l.current = s
		reopenedWithoutProgress = true
	}
	return count, true
}
func (l *loopingTrack) fail(n int, err error) (int, bool) {
	if n > 0 {
		l.pendingErr = err
		return n, true
	}
	l.err = err
	return 0, false
}
func (l *loopingTrack) Err() error { return l.err }
func (l *loopingTrack) Close() error {
	l.closed = true
	if l.current != nil {
		err := l.current.Close()
		l.current = nil
		return err
	}
	return nil
}
