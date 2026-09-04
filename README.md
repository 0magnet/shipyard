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
- **✓ A browser, in Go.** `cmd/browser` is a web browser written in Go/wasm —
  a netscrape port. Chrome (tabs, address bar, back/forward, reload) is DOM from
  `syscall/js`; each tab is a sandboxed `<iframe>`. It **fetches** pages over a
  pluggable transport (`fetchVia`), **transcodes** them (renders in a sandboxed
  srcdoc, inlines stylesheets as `<style>` and images as `data:` URIs via a
  parent/sandbox relay), and **relays navigation** (a sandboxed page's link and
  form submits drive the browser). `run /work/browser.wasm` opens it in a
  window. Each of fetch, sandbox, navigation, and resource-inlining is
  headless-verified.
  The transport is pluggable (`globalThis.__shipyardBrowserFetch`), so a host
  can route it through its own network; that's an exploration, not a commitment
  to live anywhere in particular — if the shipyard/wasm-visor combination is
  worth shipping, it belongs in its own repo, not folded into either side.
- **✓ Instant reloads.** `index.html` restores the whole workstation —
  toolchain, standard library, and your `/work` — from IndexedDB via
  `jsfs.persist`, so a reload skips the ~100 MB cold seed entirely. The module
  cache persists the same way, so a dependency fetched once stays fetched.
  Verified: a reload restores from cache with no re-seed.
- **Richer transcoding.** Fonts, nested `@import`, and XHR layer onto the same
  resource relay the browser already uses for stylesheets and images.
- **Lazy std.** Seeding is all-of-std up front today; fetching each std package
  only when a build first imports it would shrink the cold seed further.

## Build

    ./build.sh          # builds shipyard.wasm + pulls shipwright's toolchain into web/
    go run ./serve      # http://localhost:8931, static + the /goproxy passthrough

Then open `http://localhost:8931/demo.html`.
