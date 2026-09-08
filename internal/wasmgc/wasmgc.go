// Package wasmgc sizes the Go heap for a runtime that can never give memory
// back.
//
// A wasm module's linear memory only ever GROWS: WebAssembly.Memory.grow() is
// one-way and the Go runtime never returns pages to the host. So the heap's
// PEAK is not a transient — it is a permanent cost for the life of the tab, and
// a single spike is paid for until the page is closed.
//
// GOGC decides how big that peak is. It sizes the heap at roughly
// (1 + GOGC/100) x live-after-GC, so the default 100 reserves about twice the
// live set forever. Measured on skywire's browser visor, which has the same
// shape of problem: heapSys/live was 4.30 at GOGC=100 against 1.56 at GOGC=50.
// The cost is more frequent collections; the saving is memory that would
// otherwise never come back.
//
// Which runtimes this is for: the ones that live as long as the page. In
// shipyard that is the shell (cmd/shipyard) and the browser window
// (cmd/browser). It is deliberately NOT pushed onto the toolchain, which is by
// far the biggest allocator here — `go build` runs as a CHILD wasm instance
// through bottle's proc, and a child's whole linear memory is reclaimed when it
// exits (proc.js goes out of its way to keep no closure that would pin it), so
// a compile's peak is transient rather than a ratchet. Halving GOGC there would
// buy nothing and make every build collect twice as often.
//
// What it is worth here, measured rather than assumed: after two `go build`s in
// the desk, the shell's OWN heap peaked at heapSys 1.9 MB having run no GC at
// all, and lowering GOGC changed nothing — the tab's several hundred megabytes
// are in the child instances and in JS, not here. So this is a bound on what a
// page-lifetime runtime can ratchet to if it ever does grow, not a saving
// already banked. __shipyardMem() in cmd/shipyard reports the numbers, so the
// next person can check rather than believe.
package wasmgc

import (
	"os"
	"runtime/debug"
)

// Tune lowers GOGC unless the operator has already chosen a value.
//
// The environment is read directly rather than inferring from what
// SetGCPercent returns: that reports the previous percentage, and an explicit
// GOGC=100 is indistinguishable from the default that way.
func Tune() {
	if os.Getenv("GOGC") != "" {
		return
	}
	debug.SetGCPercent(50)
}
