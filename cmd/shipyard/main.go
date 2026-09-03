//go:build js && wasm

// Command shipyard boots a browser workstation: a websh terminal in a winbox
// window, its shell running over bottle's jsfs — the same in-memory filesystem
// the Go toolchain (shipwright) reads and writes — with `go` on its PATH.
//
// Type `go build` in the window and the toolchain runs as child wasm processes
// via bottle's proc layer: the terminal drives the compiler, all in the tab.
// window.__shipyardSubmit(line) feeds a command in without a keyboard, so the
// wiring can be exercised headlessly.
package main

import (
	"syscall/js"

	"github.com/0magnet/afero"
	"github.com/0magnet/websh/web"
	winbox "github.com/0magnet/winbox-go"
)

func main() {
	doc := js.Global().Get("document")
	winbox.InjectCSS()

	// The environment the toolchain needs; the shell passes it to `go`, which
	// passes it on to compile/asm/link. The page sets __shipyardGOPROXY when a
	// module-proxy passthrough is available.
	env := []string{
		"GOROOT=/goroot", "GOPATH=/gopath", "HOME=/root", "TMPDIR=/tmp",
		"GOCACHE=/root/.cache/go-build", "PATH=/bin:/goroot/pkg/tool/js_wasm",
		"GOOS=js", "GOARCH=wasm", "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GO111MODULE=on",
	}
	if p := js.Global().Get("__shipyardGOPROXY"); p.Truthy() {
		env = append(env, "GOPROXY="+p.String(), "GOSUMDB=sum.golang.org")
	} else {
		env = append(env, "GOPROXY=off", "GOSUMDB=off")
	}

	opts := &winbox.Options{
		Title:  "shell — the Go toolchain is on your PATH",
		Width:  winbox.Px(780),
		Height: winbox.Px(480),
		X:      winbox.Px(60),
		Y:      winbox.Px(60),
	}
	if root := doc.Call("getElementById", "desktop"); root.Truthy() {
		opts.Root = root
	}
	w := winbox.New(opts)

	sess, err := web.NewSession(w.Body, web.Options{
		FS:       afero.NewOsFs(), // == bottle's jsfs on js/wasm
		Host:     "user@shipyard",
		Greeting: "shipyard — go is on your PATH. Try:  go version   |   cd /work && go build .\r\n",
		Env:      env,
		NoWebGL:  true, // DOM renderer, so a headless check can read the buffer
	})
	if err != nil {
		js.Global().Get("console").Call("error", "shipyard: "+err.Error())
		return
	}

	// Feed a command as if typed — for headless verification and scripting.
	js.Global().Set("__shipyardSubmit", js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) > 0 {
			sess.Submit(a[0].String())
		}
		return nil
	}))

	js.Global().Get("console").Call("log", "shipyard: terminal window open — go is on PATH")
	if r := js.Global().Get("__shipyardReady"); r.Truthy() {
		r.Invoke()
	}
	select {}
}
