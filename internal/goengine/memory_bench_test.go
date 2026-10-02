package goengine

import (
	"github.com/17hwliao/table-card-independent/internal/table"
	"testing"
)

var benchmarkScore float64

func BenchmarkScoreEmptyBoard(b *testing.B) {
	var board [BoardSize][BoardSize]uint8
	b.ReportAllocs()
	for range b.N {
		x, y := score(board)
		benchmarkScore = x + y
	}
}
func BenchmarkBotAction(b *testing.B) {
	e, err := NewEngine([]table.Player{{ID: "a"}, {ID: "b"}})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		e.BotAction("a")
	}
}
