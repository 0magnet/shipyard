//go:build js && wasm

// Command server is the vnet demo payload: a REAL Go net/http server that runs
// inside the browser tab. It listens on bottle's virtual loopback (vnet) via
// the vnet subpackage's net.Listen drop-in, so a SEPARATE wasm instance in the
// same tab — the netscrape browser — can dial it and fetch its page entirely
// client-side, with no server-side proxy. That is the thing the Go Playground
// fundamentally cannot do, and it works on static hosting (GitHub Pages).
//
// Build and run it in the desk:
//
//	cd /work/server && go build -o server.wasm . && run server.wasm
//
// then open http://127.0.0.1:8080/ in the netscrape browser (run browser.wasm).
//
// The vnet/ subpackage is a self-contained copy of github.com/0magnet/bottle's
// vnet net.* drop-ins, so this module builds offline from the seeded standard
// library alone — no external dependency to fetch.
package main

import (
	"fmt"
	"net/http"
	"os"
	"syscall/js"
	"time"

	"srv/vnet"
)

// page is what the server serves. The SHIPYARD-VNET-PAGE token is the headless
// marker the selftest greps for once netscrape has rendered this over vnet.
const page = `<!doctype html><html><head><meta charset="utf-8">
<title>served over vnet</title>
<style>body{font:15px/1.6 ui-monospace,monospace;background:#0e0c14;color:#cdd2da;padding:2.4em;max-width:40em;margin:auto}
h1{color:#9d7cff;font-size:1.5em}code{color:#7ce0b0}a{color:#9d7cff}</style></head>
<body>
<h1>Hello from inside the tab.</h1>
<p>This page came from a real Go <code>net/http</code> server running as a wasm
process in <em>this same browser tab</em>. The netscrape browser fetched it over
bottle's <code>vnet</code> virtual loopback — no server, no <code>/fetch</code>
proxy, no network round-trip. <!-- SHIPYARD-VNET-PAGE --></p>
<p>Served at <code>127.0.0.1:8080</code> · request #%d · %s</p>
<p><a href="/ping">/ping</a> answers plain text over the same vnet pipe.</p>
</body></html>`

var hits int

func status(msg string) {
	id := os.Getenv("SHIPYARD_MOUNT")
	if id == "" {
		return
	}
	el := js.Global().Get("document").Call("getElementById", id)
	if !el.Truthy() {
		return
	}
	el.Get("style").Set("cssText", "font:14px/1.6 ui-monospace,monospace;color:#7ce0b0;background:#0e0c14;padding:1.4em;height:100%;box-sizing:border-box;white-space:pre-wrap")
	el.Set("textContent", msg)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		fmt.Fprintf(w, "pong over vnet — SHIPYARD-VNET-PAGE\n")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, page, hits, time.Now().Format("15:04:05"))
	})

	// vnet.Listen is net.Listen with a browser escape hatch: on js/wasm a
	// loopback address is bound in the page's vnet port table, so another wasm
	// instance in the tab can reach it. Everything above this line is ordinary,
	// unmodified net/http server code.
	ln, err := vnet.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		status("vnet listen failed: " + err.Error())
		js.Global().Get("console").Call("error", "server: "+err.Error())
		return
	}
	status("Go net/http server listening on vnet 127.0.0.1:8080\n\nBrowse to http://127.0.0.1:8080/ in the netscrape\nbrowser (run browser.wasm) — the request is dialed\nover vnet, entirely inside this tab.")
	js.Global().Get("console").Call("log", "server: net/http listening on vnet:8080")
	if err := http.Serve(ln, mux); err != nil {
		js.Global().Get("console").Call("error", "server: "+err.Error())
	}
}
