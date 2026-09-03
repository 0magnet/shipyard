# shipyard — a Go workstation in a browser tab

[bottle](https://github.com/0magnet/bottle) gives a tab an operating system —
a filesystem, a network, processes. [shipwright](https://github.com/0magnet/shipwright)
gives it the Go toolchain. shipyard is where you sit down and use them: a shell
where you type `go build`, on the same in-memory filesystem the compiler reads,
with the browser to view what you make.

The pieces, and who does what:

- **bottle** — jsfs (files), vnet (loopback network), proc (processes).
- **shipwright** — `cmd/go` + `cmd/compile`/`asm`/`link`, built for js/wasm.
- **websh** — the terminal (a Go/wasm shell on xterm).
- **netscrape** — the browser, for viewing what you serve on the vnet.
- **winbox-go** — the windowing the desk is made of.

## What works today (milestone 1)

**The shell runs `go`.** shipyard boots a websh shell over `afero.NewOsFs()` —
which on js/wasm *is* bottle's jsfs — and puts `go` on its PATH. Because the
shell and the toolchain share one filesystem, and websh execs a filesystem
program as a child wasm process through bottle's proc, the toolchain runs as an
ordinary command:

    $ go version
    go version go1.27.0 js/wasm
    $ go env GOROOT
    /goroot
    $ cd /work && go build -o hello.wasm .    # compiles fmt/os/runtime from source
    $ go install github.com/0magnet/websh/cmd/websh@main   # fetches + builds a whole tool

The last line is not hypothetical — shipwright already compiles websh from its
whole module graph, fetched over a `/goproxy` passthrough. In shipyard you type
it at a prompt.

`demo.html` is the headless-verified proof: it seeds the toolchain into jsfs,
boots shipyard, and drives `go version` / `go env` / `go build` through the
shell, capturing their output. Build and serve it with `./build.sh` (see below).

## How the shell execs the toolchain

websh's exec handler, on js/wasm, resolves an unknown command against the
filesystem PATH and — if it finds a program — spawns it as a child wasm
instance via `globalThis.proc.spawn`, sharing this tab's jsfs and vnet. The
child's stdout/stderr cross through jsfs pipes (plain JS sinks, drained into
the shell after the child exits), so nothing re-enters the shell's Go runtime
mid-execution. `go` is just `go-proc.wasm` parked at `/bin/go`; running it is
instantiating another wasm module.

## Roadmap

- **Milestone 2 — the desk.** Mount websh's `web.Session` terminal in a
  winbox window; open netscrape alongside. Type `go build`, then view a
  vnet-served result in the browser tab. A real windowed workstation.
- **Milestone 3 — run what you build.** Launch a compiled DOM/wasm program in
  its own window (instantiate `/work/<bin>` against a window's element), so
  `go build && ./thing` opens a UI.
- **Lazy std + persistence.** Seed the standard library on demand and cache it
  (with the module cache) in IndexedDB via jsfs, so a reload is instant.
- **netscrape in Go.** Its chrome is DOM work, its transport is Go (dmsg is
  already Go); porting it means shipwright compiles the browser too.

## Build

    ./build.sh          # builds shipyard.wasm + pulls shipwright's toolchain into web/
    go run ./serve      # http://localhost:8931, static + the /goproxy passthrough

Then open `http://localhost:8931/demo.html`.
