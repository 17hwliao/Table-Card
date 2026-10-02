// Package memory sets conservative Go runtime defaults for local multi-client
// use. Explicit GOGC, GOMEMLIMIT and GOMAXPROCS settings take precedence.
package memory

import (
	"os"
	"runtime"
	"runtime/debug"
)

func Client() { configure(64<<20, 2) }
func Server() { configure(128<<20, 4) }

// Release returns unused heap pages to the OS after a server becomes idle.
// It is deliberately not called by render loops or active room handlers.
func Release() { debug.FreeOSMemory() }
func configure(budget int64, cores int) {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(50)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(budget)
	}
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(min(cores, runtime.NumCPU()))
	}
}
