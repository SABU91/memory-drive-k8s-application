package simulate

import (
	"math"
	"sync"
	"time"
)

// RunCPULoad spins up `workers` goroutines that perform busy math for the given
// duration, driving up CPU utilisation. It blocks until the burst completes.
// The work is bounded by time, so it can never run away or crash the process.
func RunCPULoad(workers int, duration time.Duration) {
	if workers <= 0 {
		workers = 1
	}
	if duration <= 0 {
		duration = time.Second
	}

	deadline := time.Now().Add(duration)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			x := 1.0001
			for time.Now().Before(deadline) {
				// A tight, non-trivial loop the compiler cannot elide.
				for j := 0; j < 1_000_000; j++ {
					x = math.Sqrt(x*1.0000001) + 1.0
				}
				// Keep the result observable so it is not optimised away.
				if x > math.MaxFloat64/2 {
					x = 1.0001
				}
			}
		}()
	}
	wg.Wait()
}
