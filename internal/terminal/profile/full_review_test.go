package profile

import (
	"sync"
	"testing"
)

func TestReviewConcurrentFirstIdentityLoad(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	for round := 0; round < 30; round++ {
		start := make(chan struct{})
		errors := make(chan error, 16)
		var wg sync.WaitGroup
		for worker := 0; worker < 16; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if _, err := Load("review:1781", string(rune('A'+round))); err != nil {
					errors <- err
				}
			}()
		}
		close(start)
		wg.Wait()
		close(errors)
		if len(errors) > 0 {
			t.Fatalf("%d concurrent readers rejected an identity during creation: %v", len(errors), <-errors)
		}
	}
}
