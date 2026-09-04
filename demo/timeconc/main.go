//go:build js && wasm

// Command timeconc is the real-time + concurrency demo: it launches several
// goroutines that each time.Sleep a different REAL duration, then reports the
// order they finished in and the true wall-clock elapsed time, stamped with
// time.Now(). The elapsed time tracks the longest sleep because the clock is
// real and the goroutines run concurrently on real timers.
//
//	The Go Playground fakes the clock: time is deterministic and frozen, sleeps
//	resolve in a fixed fake order, and long/parallel timing is capped. Here the
//	timestamps advance in real time and the goroutines finish in real order.
//
// Its output goes to stdout, which the websh terminal shows.
package main

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type finish struct {
	id      int
	slept   time.Duration
	elapsed time.Duration
}

func main() {
	start := time.Now()
	fmt.Printf("timeconc: start wall-clock time is %s\n", start.Format("15:04:05.000"))

	// Five concurrent workers, each sleeping a real, distinct duration.
	sleeps := []time.Duration{
		250 * time.Millisecond,
		100 * time.Millisecond,
		400 * time.Millisecond,
		50 * time.Millisecond,
		300 * time.Millisecond,
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var order []finish

	for i, d := range sleeps {
		wg.Add(1)
		go func(id int, dur time.Duration) {
			defer wg.Done()
			time.Sleep(dur)
			mu.Lock()
			order = append(order, finish{id: id, slept: dur, elapsed: time.Since(start)})
			mu.Unlock()
		}(i, d)
	}
	wg.Wait()
	total := time.Since(start)

	// Finish order is by real completion time — shortest sleeps first.
	sort.Slice(order, func(a, b int) bool { return order[a].elapsed < order[b].elapsed })
	for rank, f := range order {
		fmt.Printf("timeconc: #%d worker %d slept %v, done at +%v\n", rank+1, f.id, f.slept, f.elapsed.Round(time.Millisecond))
	}
	end := time.Now()
	fmt.Printf("timeconc: end wall-clock time is %s\n", end.Format("15:04:05.000"))

	// The whole run took about as long as the LONGEST sleep (concurrent), and
	// meaningfully longer than zero — proof the clock is real, not faked.
	realish := total >= 350*time.Millisecond && total < 5*time.Second
	if realish {
		fmt.Printf("SHIPYARD-TIME-MARKER: 5 goroutines finished in real order in %v of real wall-clock time\n", total.Round(time.Millisecond))
	} else {
		fmt.Printf("SHIPYARD-TIME-FAIL: elapsed %v not in the expected real-clock range\n", total)
	}
}
