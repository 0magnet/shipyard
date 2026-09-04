//go:build js && wasm

// Command browser builds netscrape (github.com/0magnet/netscrape) as a
// standalone Go/wasm module for the desk: a thin wrapper that resolves the
// window's mount element and hands off to netscrape.Open, then blocks. shipyard
// no longer carries its own copy of the browser — it imports netscrape, and
// this binary exists only so the desk can DEMONSTRATE `run browser.wasm`:
// compiling a Go program and running it in its own window (the M3 goal).
//
// A page that already runs a Go/wasm binary would import netscrape.Open and
// share that runtime instead of running this as a second module — see
// netscrape.Open. The desk runs it standalone on purpose: that IS the demo.
package main

import (
	"os"
	"syscall/js"

	"github.com/0magnet/netscrape"
)

func main() {
	doc := js.Global().Get("document")
	// shipyard's proc layer opens a fresh winbox window and passes its mount
	// element id in $SHIPYARD_MOUNT; a direct host may set globalThis.__shipyardMount
	// (an element id, or the element).
	root := doc.Call("getElementById", os.Getenv("SHIPYARD_MOUNT"))
	if !root.Truthy() {
		if m := js.Global().Get("__shipyardMount"); m.Truthy() {
			if m.Type() == js.TypeString {
				root = doc.Call("getElementById", m.String())
			} else {
				root = m
			}
		}
	}
	if !root.Truthy() {
		return
	}
	netscrape.Open(root)
	select {}
}
