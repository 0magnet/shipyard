//go:build js && wasm

// Command shipyard boots a browser workstation: a websh terminal in a desk
// window, its shell running over bottle's jsfs — the same in-memory filesystem
// the Go toolchain (shipwright) reads and writes — with `go` on its PATH.
//
// Type `go build` in the window and the toolchain runs as child wasm processes
// via bottle's proc layer. Type `run ./thing.wasm` and the program you just
// built opens in its own window: `run` spawns it as a child that draws into a
// fresh window (its element id is passed in $SHIPYARD_MOUNT). So the full loop
// — edit, build, run a UI — happens in the tab.
//
// github.com/0magnet/desk supplies the window management: a panel with a task
// button per window and an Applications menu, and a registry of what can be
// opened. Everything shipyard puts on screen is a registered app, so a window
// that is closed can be opened again — before the desk, closing the terminal
// left no way to get another.
//
// window.__shipyardSubmit(line) feeds a command into the oldest surviving
// terminal without a keyboard, so the wiring can be exercised headlessly.
package main

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/0magnet/afero"
	"github.com/0magnet/desk"
	"github.com/0magnet/sh/v3/expand"
	"github.com/0magnet/sh/v3/interp"
	"github.com/0magnet/websh/shell"
	"github.com/0magnet/websh/web"
	winbox "github.com/0magnet/winbox-go"

	"github.com/0magnet/shipyard/internal/wasmgc"
)

// shellEnv is what every shell starts with, and what a program launched from
// the Applications menu — rather than from a prompt — inherits.
var shellEnv []string

const greeting = "shipyard — go is on your PATH.\r\n  go version   |   cd /work && go build .   |   run ./hello.wasm\r\n"

// ---- the terminal ---------------------------------------------------------

// termPane is a websh shell as a desk pane.
//
// shipyard brings its own rather than using desk/panes/term because all three
// of its options matter here: the shell has to run over afero.NewOsFs (which on
// js/wasm IS bottle's jsfs, the filesystem the toolchain reads), it has to
// carry the toolchain environment, and it has to use the DOM renderer so a
// headless check can read the terminal buffer back out of the DOM. Bringing a
// pane is what desk's Pane interface is for.
type termPane struct{ sess *web.Session }

// terms is every live terminal, oldest first. __shipyardSubmit targets the
// oldest so the gallery keeps typing into the same window it reads back — it
// finds the terminal as the first window in the desk.
var terms []*termPane

func (t *termPane) Mount(el js.Value) error {
	s, err := web.NewSession(el, web.Options{
		FS:       afero.NewOsFs(), // == bottle's jsfs on js/wasm
		Host:     "user@shipyard",
		Greeting: greeting,
		Env:      shellEnv,
		NoWebGL:  true, // DOM renderer, so a headless check can read the buffer
	})
	if err != nil {
		return err
	}
	t.sess = s
	terms = append(terms, t)
	return nil
}

func (t *termPane) Close() {
	for i, x := range terms {
		if x == t {
			terms = append(terms[:i], terms[i+1:]...)
			break
		}
	}
	if t.sess != nil {
		t.sess.Close()
		t.sess = nil
	}
}

// frontTerm is the terminal __shipyardSubmit types into.
func frontTerm() *web.Session {
	for _, t := range terms {
		if t.sess != nil {
			return t.sess
		}
	}
	return nil
}

// ---- programs -------------------------------------------------------------

var appN int // unique mount-element counter

// progPane is a wasm program from the filesystem, drawing into a desk window.
// The program finds the element to draw into via $SHIPYARD_MOUNT.
type progPane struct {
	prog, cwd string
	env       js.Value
	args      []string
	handle    js.Value // proc.spawn's {pid, exited, kill}
}

func (p *progPane) Mount(el js.Value) error {
	appN++
	mountID := "shipyard-app-" + strconv.Itoa(appN)
	doc := js.Global().Get("document")
	mount := doc.Call("createElement", "div")
	mount.Set("id", mountID)
	mount.Get("style").Set("cssText", "position:absolute;inset:0")
	el.Call("appendChild", mount)

	// A fresh env per launch: the mount id differs, and the caller's object is
	// reused by every later launch of the same program.
	env := js.Global().Get("Object").Call("assign", js.Global().Get("Object").New(), p.env)
	env.Set("SHIPYARD_MOUNT", mountID)
	argv := js.Global().Get("Array").New()
	argv.Call("push", p.prog)
	for _, a := range p.args {
		argv.Call("push", a)
	}
	opts := js.Global().Get("Object").New()
	opts.Set("argv", argv)
	opts.Set("cwd", p.cwd)
	opts.Set("env", env)

	// Spawn detached — a UI program runs until its window closes, so the prompt
	// must not wait on it. Its stdio inherits the page defaults.
	p.handle = js.Global().Get("proc").Call("spawn", opts)
	return nil
}

// Close interrupts the program, when the page's proc can. kill() delivers to a
// handler the child registered, so a program that installed none keeps running
// with its window gone — and an older proc.js has no kill on the handle at all
// (shipwright still vendors one), hence the check rather than a Call that
// panics the whole page. Either way this is a floor: before the desk owned
// these windows, closing one left the program running regardless.
func (p *progPane) Close() {
	if p.handle.Truthy() && p.handle.Get("kill").Type() == js.TypeFunction {
		p.handle.Call("kill")
	}
	p.handle = js.Value{}
}

// registerProgram makes a filesystem program a desk app: it gets a launcher
// entry, and closing its window is no longer the end of it.
func registerProgram(prog, cwd string, env js.Value, title, help string) {
	desk.Register(desk.App{
		Name:   prog, // the path — unique, and what `run` was given
		Title:  title,
		Help:   help,
		Width:  560,
		Height: 400,
		Open: func(args []string) (desk.Pane, error) {
			return &progPane{prog: prog, cwd: cwd, env: env, args: args}, nil
		},
	})
}

// runApplet launches a wasm program from the filesystem into its own window.
func runApplet(_ context.Context, s *shell.Shell, hc *interp.HandlerContext, args []string) int {
	fprintf := func(w interface{ Write([]byte) (int, error) }, format string, a ...string) {
		msg := format
		for _, x := range a {
			msg = strings.Replace(msg, "%s", x, 1)
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

	// The shell's environment and working directory, captured so the launcher
	// can start the same program again later on the same terms.
	env := js.Global().Get("Object").New()
	hc.Env.Each(func(name string, vr expand.Variable) bool {
		env.Set(name, vr.String())
		return true
	})
	registerProgram(p, hc.Dir, env, path.Base(p), "run "+p)

	if _, err := desk.Launch(p, args[1:]...); err != nil {
		fprintf(hc.Stderr, "run: %s\n", err.Error())
		return 1
	}
	fprintf(hc.Stdout, "launched %s in a window\n", path.Base(p))
	return 0
}

// envObject turns a "NAME=value" list into the object proc.spawn wants.
func envObject(kv []string) js.Value {
	o := js.Global().Get("Object").New()
	for _, e := range kv {
		if name, val, ok := strings.Cut(e, "="); ok {
			o.Set(name, val)
		}
	}
	return o
}

func main() {
	wasmgc.Tune()

	doc := js.Global().Get("document")
	winbox.InjectCSS()

	// The desk owns the windows. SetRoot keeps them inside #desktop, and the
	// panel gives each one a task button — so a minimized or buried window has
	// somewhere to be got back from — plus an Applications menu to start one
	// from. The gallery dock in index.html is unaffected: it is a page element,
	// not a desk window.
	if root := doc.Call("getElementById", "desktop"); root.Truthy() {
		desk.SetRoot(root)
	}
	desk.NewPanel()

	shell.RegisterApplet("run", "run a wasm program in its own window", runApplet)

	shellEnv = []string{
		"GOROOT=/goroot", "GOPATH=/gopath", "HOME=/root", "TMPDIR=/tmp",
		"GOCACHE=/root/.cache/go-build", "PATH=/bin:/goroot/pkg/tool/js_wasm",
		"GOOS=js", "GOARCH=wasm", "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GO111MODULE=on",
	}
	if p := js.Global().Get("__shipyardGOPROXY"); p.Truthy() {
		shellEnv = append(shellEnv, "GOPROXY="+p.String(), "GOSUMDB=sum.golang.org")
	} else {
		shellEnv = append(shellEnv, "GOPROXY=off", "GOSUMDB=off")
	}

	desk.Register(desk.App{
		Name:   "shell",
		Title:  "shell — the Go toolchain is on your PATH",
		Help:   "a websh shell over jsfs, with go on its PATH",
		Width:  780,
		Height: 480,
		Open:   func([]string) (desk.Pane, error) { return &termPane{}, nil },
	})

	// The prebuilt programs index.html seeds, in the launcher before anyone has
	// typed `run`. Absent on a page that seeded neither, so they are checked.
	pageEnv := envObject(shellEnv)
	for _, prog := range []struct{ path, title, help string }{
		{"/work/browser.wasm", "browser", "netscrape — a web browser written in Go"},
		{"/work/server.wasm", "server", "a Go net/http server on the vnet loopback"},
	} {
		if fi, err := os.Stat(prog.path); err == nil && !fi.IsDir() {
			registerProgram(prog.path, "/work", pageEnv, prog.title, prog.help)
		}
	}

	if _, err := desk.LaunchOpts("shell", desk.Options{X: 60, Y: 60}); err != nil {
		js.Global().Get("console").Call("error", "shipyard: "+err.Error())
		return
	}

	// runtime.MemStats for the page, so the heap-sizing claim in
	// internal/wasmgc can be checked on a real workstation rather than taken on
	// trust: read __shipyardMem() after a build and compare heapSys with the
	// live set.
	js.Global().Set("__shipyardMem", js.FuncOf(func(js.Value, []js.Value) any {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return js.ValueOf(map[string]any{
			"heapSys": m.HeapSys, "heapAlloc": m.HeapAlloc, "heapInuse": m.HeapInuse,
			"heapIdle": m.HeapIdle, "heapReleased": m.HeapReleased,
			"sys": m.Sys, "numGC": m.NumGC, "gogc": os.Getenv("GOGC"),
		})
	}))

	js.Global().Set("__shipyardSubmit", js.FuncOf(func(_ js.Value, a []js.Value) any {
		if s := frontTerm(); s != nil && len(a) > 0 {
			s.Submit(a[0].String())
		}
		return nil
	}))

	js.Global().Get("console").Call("log", "shipyard: terminal window open — go is on PATH, run launches windows")
	if r := js.Global().Get("__shipyardReady"); r.Truthy() {
		r.Invoke()
	}
	select {}
}
