//go:build js && wasm

// Command shipyard boots a websh shell over bottle's jsfs — the same in-memory
// filesystem the Go toolchain (shipwright) reads and writes — and puts `go` on
// its PATH. From that shell, `go build` / `go install` run as child wasm
// processes via bottle's proc layer: the terminal drives the toolchain, all in
// the tab.
//
// This is milestone one: the shell and the toolchain share a filesystem and
// exec across it. The interactive terminal window (websh's web.Session in a
// winbox frame) and netscrape for viewing vnet-served output come next; here
// the shell is driven through window.shipyardRun(cmd) so the wiring can be
// exercised headlessly.
package main

import (
	"bytes"
	"context"
	"strings"
	"syscall/js"

	"github.com/0magnet/afero"
	"github.com/0magnet/websh/shell"
)

func main() {
	// afero.OsFs on js/wasm goes through the os package to syscall/fs_js.go to
	// globalThis.fs — bottle's jsfs. So the shell and every process it spawns
	// share one filesystem.
	var out bytes.Buffer
	sh, err := shell.New(afero.NewOsFs(), strings.NewReader(""), &out, &out)
	if err != nil {
		js.Global().Get("console").Call("error", "shipyard: "+err.Error())
		return
	}
	ctx := context.Background()

	// The environment the toolchain needs; the shell passes it to `go`, which
	// passes it on to compile/asm/link. GOPROXY is set by the page if a
	// passthrough is available.
	env := "export GOROOT=/goroot GOPATH=/gopath HOME=/root TMPDIR=/tmp " +
		"GOCACHE=/root/.cache/go-build PATH=/bin:/goroot/pkg/tool/js_wasm " +
		"GOOS=js GOARCH=wasm GOFLAGS=-mod=mod GOTOOLCHAIN=local GO111MODULE=on"
	if p := js.Global().Get("__shipyardGOPROXY"); p.Truthy() {
		env += " GOPROXY=" + p.String() + " GOSUMDB=sum.golang.org"
	} else {
		env += " GOPROXY=off GOSUMDB=off"
	}
	if _, err := sh.Run(ctx, env); err != nil {
		js.Global().Get("console").Call("error", "shipyard env: "+err.Error())
	}

	// Run a shell line and resolve with its combined output. Runs on its own
	// goroutine so a blocking build (which drives child wasm processes on the
	// JS event loop) never blocks the caller's synchronous JS frame.
	run := js.FuncOf(func(_ js.Value, args []js.Value) any {
		line := args[0].String()
		promise := js.Global().Get("Promise")
		return promise.New(js.FuncOf(func(_ js.Value, pa []js.Value) any {
			resolve := pa[0]
			go func() {
				out.Reset()
				_, runErr := sh.Run(ctx, line)
				text := out.String()
				if runErr != nil {
					text += "\n[shipyard: " + runErr.Error() + "]"
				}
				resolve.Invoke(text)
			}()
			return nil
		}))
	})
	js.Global().Set("shipyardRun", run)

	js.Global().Get("console").Call("log", "shipyard: shell ready on jsfs — go is on PATH")
	if r := js.Global().Get("__shipyardReady"); r.Truthy() {
		r.Invoke()
	}
	select {}
}
