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

## What works today (milestones 1–3)

**A terminal window where you type `go build`.** shipyard opens a websh terminal
in a [winbox](https://github.com/0magnet/winbox-go) window, its shell running
over `afero.NewOsFs()` — which on js/wasm *is* bottle's jsfs — with `go` on its
PATH. Because the shell and the toolchain share one filesystem, and websh execs
a filesystem program as a child wasm process through bottle's proc, the
toolchain runs as an ordinary command you type at the prompt:

    $ go version
    go version go1.27.0 js/wasm
    $ go env GOROOT
    /goroot
    $ cd /work && go build -o hello.wasm .    # compiles fmt/os/runtime from source
    $ go install github.com/0magnet/websh/cmd/websh@main   # fetches + builds a whole tool

The last line is not hypothetical — shipwright already compiles websh from its
whole module graph, fetched over a `/goproxy` passthrough. In shipyard you type
it at a prompt.

`index.html` is the desk: it seeds the toolchain into jsfs, opens the terminal
window, and lets you type. Open it with `#selftest` and it drives `go version`
and `go build` in and checks the artifact — the headless proof. `demo.html` is
the same wiring without a window (the shell driven through a JS call), kept as
the minimal read. Build and serve with `./build.sh` (see below).

## How the shell execs the toolchain

websh's exec handler, on js/wasm, resolves an unknown command against the
filesystem PATH and — if it finds a program — spawns it as a child wasm
instance via `globalThis.proc.spawn`, sharing this tab's jsfs and vnet. The
child's stdout/stderr cross through jsfs pipes (plain JS sinks, drained into
the shell after the child exits), so nothing re-enters the shell's Go runtime
mid-execution. `go` is just `go-proc.wasm` parked at `/bin/go`; running it is
instantiating another wasm module.

## Roadmap

- **✓ Milestone 2 — the terminal window.** websh's `web.Session` in a winbox
  window, `go build` typed at the prompt.
- **✓ Milestone 3 — run what you build.** The `run` command spawns a compiled
  wasm program into its own winbox window (its mount element id arrives in
  `$SHIPYARD_MOUNT`), so `go build -o clock.wasm . && run clock.wasm` opens a
  live UI. Headless-verified: build a GUI program in the terminal, run it, and
  a second window draws its output.
- **✓ A browser, in Go.** `cmd/browser` is a minimal web browser written in
  Go/wasm — the shape of a netscrape port: its chrome (tabs, address bar, back/forward, reload) is DOM built from Go via
  `syscall/js`, and each tab is an `<iframe>` Go navigates. `run /work/browser.wasm` opens it in a window.
  Verified: the browser comes up and its iframe engine navigates.
- **Next — full netscrape parity.** The browser has tabs and direct-mode
  navigation now; parity needs the transcoding engine that inlines a fetched
  page into a sandboxed iframe (the bulk of netscrape's 3.6k-line browse.js)
  and the clearnet/dmsg transports feeding it — the `fetchPage` seam is marked
  in `cmd/browser`. Then swap the Go browser in for the JS netscrape, adapting
  skywire's wasm visor at the same time.
- **Lazy std + persistence.** Seed the standard library on demand and cache it
  (with the module cache) in IndexedDB via jsfs, so a reload is instant.

## Build

    ./build.sh          # builds shipyard.wasm + pulls shipwright's toolchain into web/
    go run ./serve      # http://localhost:8931, static + the /goproxy passthrough

Then open `http://localhost:8931/demo.html`.
