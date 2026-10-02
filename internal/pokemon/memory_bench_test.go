package pokemon

import "testing"

var benchmarkMonster Monster

func BenchmarkNewMonster(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		benchmarkMonster = newMonster(25, 30)
	}
}
func BenchmarkBoundedLog(b *testing.B) {
	g := New("log")
	for range 120 {
		g.Notice("训练记录")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		g.Notice("训练记录")
	}
}
