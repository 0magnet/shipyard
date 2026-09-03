//go:build js && wasm

// Command browser is a minimal web browser written in Go/wasm — the shape of a
// netscrape port. Its chrome (address bar, back/forward, reload) is DOM built
// from Go via syscall/js; its engine is an <iframe> Go points at a URL (the
// "direct mode" netscrape uses for trusted origins). That's the whole trick:
// the browser is Go, the rendering is delegated to the platform iframe.
//
// It runs as a shipyard program — it draws into the element named by
// $SHIPYARD_MOUNT — so `run browser.wasm` opens a browser in its own window.
package main

import (
	"os"
	"syscall/js"
)

func main() {
	doc := js.Global().Get("document")
	root := doc.Call("getElementById", os.Getenv("SHIPYARD_MOUNT"))
	if !root.Truthy() {
		return
	}
	root.Get("style").Set("cssText", "position:absolute;inset:0;display:flex;flex-direction:column;background:#15131c")

	mk := func(tag string) js.Value { return doc.Call("createElement", tag) }
	btn := func(label string) js.Value {
		b := mk("button")
		b.Set("textContent", label)
		b.Get("style").Set("cssText", "background:#2a2342;color:#cdd2da;border:1px solid #3a3352;padding:2px 8px;cursor:pointer;font:13px monospace")
		return b
	}

	bar := mk("div")
	bar.Get("style").Set("cssText", "display:flex;gap:4px;padding:4px;background:#100d18;border-bottom:1px solid #2a2342")
	back, fwd, reload := btn("◀"), btn("▶"), btn("⟳")
	addr := mk("input")
	addr.Get("style").Set("cssText", "flex:1;background:#0e0c14;color:#cdd2da;border:1px solid #2a2342;padding:2px 8px;font:13px monospace")
	addr.Set("value", "data:text/html,<body style='font-family:sans-serif;padding:2em'><h1>A browser, in Go</h1><p>The chrome is syscall/js. The page is an iframe. Type a URL above.</p></body>")
	go_ := btn("Go")
	for _, el := range []js.Value{back, fwd, reload, addr, go_} {
		bar.Call("appendChild", el)
	}

	frame := mk("iframe")
	frame.Get("style").Set("cssText", "flex:1;border:0;background:#fff")
	frame.Set("id", "browser-frame") // so a harness can read where it navigated

	root.Call("appendChild", bar)
	root.Call("appendChild", frame)

	// History: a back/forward stack the Go side owns.
	var hist []string
	pos := -1
	load := func(url string) { frame.Set("src", url); addr.Set("value", url) }
	navigate := func(url string) {
		if pos >= 0 && pos < len(hist)-1 {
			hist = hist[:pos+1] // a new navigation truncates the forward history
		}
		hist = append(hist, url)
		pos = len(hist) - 1
		load(url)
	}

	onClick := func(el js.Value, fn func()) {
		el.Call("addEventListener", "click", js.FuncOf(func(js.Value, []js.Value) any { fn(); return nil }))
	}
	onClick(go_, func() { navigate(addr.Get("value").String()) })
	onClick(reload, func() { if pos >= 0 { load(hist[pos]) } })
	onClick(back, func() {
		if pos > 0 {
			pos--
			load(hist[pos])
		}
	})
	onClick(fwd, func() {
		if pos < len(hist)-1 {
			pos++
			load(hist[pos])
		}
	})
	addr.Call("addEventListener", "keydown", js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) > 0 && a[0].Get("key").String() == "Enter" {
			navigate(addr.Get("value").String())
		}
		return nil
	}))

	navigate(addr.Get("value").String()) // load the start page
	select {}
}
