//go:build js && wasm

// Command shipyard boots a browser workstation: a websh terminal in a winbox
// window, its shell running over bottle's jsfs — the same in-memory filesystem
// the Go toolchain (shipwright) reads and writes — with `go` on its PATH.
//
// Type `go build` in the window and the toolchain runs as child wasm processes
// via bottle's proc layer. Type `run ./thing.wasm` and the program you just
// built opens in its own window: `run` spawns it as a child that draws into a
// fresh winbox window (its element id is passed in $SHIPYARD_MOUNT). So the
// full loop — edit, build, run a UI — happens in the tab.
//
// window.__shipyardSubmit(line) feeds a command in without a keyboard, so the
// wiring can be exercised headlessly.
package main

import (
	"context"
	"path"
	"path/filepath"
	"strconv"
	"syscall/js"

	"github.com/0magnet/afero"
	"github.com/0magnet/sh/v3/expand"
	"github.com/0magnet/sh/v3/interp"
	"github.com/0magnet/websh/shell"
	"github.com/0magnet/websh/web"
	winbox "github.com/0magnet/winbox-go"
)

var appN int // unique mount-element counter

// runApplet launches a wasm program from the filesystem into its own winbox
// window. The program finds the element to draw into via $SHIPYARD_MOUNT.
func runApplet(_ context.Context, s *shell.Shell, hc *interp.HandlerContext, args []string) int {
	fprintf := func(w interface{ Write([]byte) (int, error) }, format string, a ...string) {
		msg := format
		for _, x := range a {
			msg = replaceFirst(msg, "%s", x)
		}
		w.Write([]byte(msg))
	}
	// args excludes the command name: args[0] is the program, args[1:] its args.
	if len(args) < 1 {
		fprintf(hc.Stderr, "usage: run <program.wasm> [args...]\n")
		return 2
	}
	p := args[0]
	if !filepath.IsAbs(p) {
		p = path.Join(hc.Dir, p)
	}
	if fi, err := s.FS.Stat(p); err != nil || fi.IsDir() {
		fprintf(hc.Stderr, "run: %s: not a program\n", p)
		return 1
	}

	appN++
	mountID := "shipyard-app-" + strconv.Itoa(appN)
	doc := js.Global().Get("document")
	opts := &winbox.Options{Title: path.Base(p), Width: winbox.Px(560), Height: winbox.Px(400),
		X: winbox.Px(float64(120 + appN*24)), Y: winbox.Px(float64(120 + appN*24))}
	if root := doc.Call("getElementById", "desktop"); root.Truthy() {
		opts.Root = root
	}
	w := winbox.New(opts)
	mount := doc.Call("createElement", "div")
	mount.Set("id", mountID)
	mount.Get("style").Set("cssText", "position:absolute;inset:0")
	w.Body.Call("appendChild", mount)

	// Spawn detached — a UI program runs until its window closes, so the prompt
	// must not wait on it. Its stdio inherits the page defaults.
	env := js.Global().Get("Object").New()
	hc.Env.Each(func(name string, vr expand.Variable) bool {
		env.Set(name, vr.String())
		return true
	})
	env.Set("SHIPYARD_MOUNT", mountID)
	argv := js.Global().Get("Array").New()
	argv.Call("push", p)
	for _, a := range args[1:] {
		argv.Call("push", a)
	}
	spawnOpts := js.Global().Get("Object").New()
	spawnOpts.Set("argv", argv)
	spawnOpts.Set("cwd", hc.Dir)
	spawnOpts.Set("env", env)
	js.Global().Get("proc").Call("spawn", spawnOpts)

	fprintf(hc.Stdout, "launched %s in a window\n", path.Base(p))
	return 0
}

func replaceFirst(s, old, new string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}

func main() {
	doc := js.Global().Get("document")
	winbox.InjectCSS()

	shell.RegisterApplet("run", "run a wasm program in its own window", runApplet)

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
		Greeting: "shipyard — go is on your PATH.\r\n  go version   |   cd /work && go build .   |   run ./hello.wasm\r\n",
		Env:      env,
		NoWebGL:  true, // DOM renderer, so a headless check can read the buffer
	})
	if err != nil {
		js.Global().Get("console").Call("error", "shipyard: "+err.Error())
		return
	}

	js.Global().Set("__shipyardSubmit", js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) > 0 {
			sess.Submit(a[0].String())
		}
		return nil
	}))

	js.Global().Get("console").Call("log", "shipyard: terminal window open — go is on PATH, run launches windows")
	if r := js.Global().Get("__shipyardReady"); r.Truthy() {
		r.Invoke()
	}
	select {}
}
