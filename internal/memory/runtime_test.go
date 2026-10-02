package memory

import (
	"runtime"
	"runtime/debug"
	"testing"
)

func TestDefaultsAndExplicitEnvironment(t *testing.T) {
	previousGC := debug.SetGCPercent(100)
	previousLimit := debug.SetMemoryLimit(1 << 40)
	previousCores := runtime.GOMAXPROCS(1)
	t.Cleanup(func() {
		debug.SetGCPercent(previousGC)
		debug.SetMemoryLimit(previousLimit)
		runtime.GOMAXPROCS(previousCores)
	})
	for _, key := range []string{"GOGC", "GOMEMLIMIT", "GOMAXPROCS"} {
		t.Setenv(key, "")
	}
	Client()
	if got := debug.SetGCPercent(50); got != 50 {
		t.Fatalf("client GOGC: %d", got)
	}
	if debug.SetMemoryLimit(-1) != 64<<20 || runtime.GOMAXPROCS(0) != min(2, runtime.NumCPU()) {
		t.Fatal("client defaults not applied")
	}
	Server()
	if debug.SetMemoryLimit(-1) != 128<<20 || runtime.GOMAXPROCS(0) != min(4, runtime.NumCPU()) {
		t.Fatal("server defaults not applied")
	}
	debug.SetGCPercent(75)
	debug.SetMemoryLimit(256 << 20)
	runtime.GOMAXPROCS(1)
	t.Setenv("GOGC", "75")
	t.Setenv("GOMEMLIMIT", "256MiB")
	t.Setenv("GOMAXPROCS", "1")
	Client()
	Server()
	if debug.SetGCPercent(75) != 75 || debug.SetMemoryLimit(-1) != 256<<20 || runtime.GOMAXPROCS(0) != 1 {
		t.Fatal("explicit runtime configuration was overwritten")
	}
}
