//go:build js && wasm

// Command browser is a web browser written in Go/wasm — the shape of a
// netscrape port. Its chrome (tab strip, address bar, back/forward, reload) is
// DOM built from Go via syscall/js; each tab's engine is an <iframe> Go points
// at a URL (the "direct mode" netscrape uses for trusted origins). That's the
// trick: the browser is Go, the rendering is delegated to the platform iframe.
//
// It runs as a shipyard program — it draws into the element named by
// $SHIPYARD_MOUNT — so `run browser.wasm` opens a browser in its own window.
//
// What a full netscrape port still needs on top of this: the transcoding
// engine that inlines a fetched page's resources into a sandboxed iframe (the
// bulk of browse.js), and the clearnet/dmsg transports feeding it. The
// transport seam is `fetchPage` below.
package main

import (
	"os"
	"strconv"
	"strings"
	"syscall/js"
)

const startPage = "data:text/html,<body style='font-family:sans-serif;padding:2em'>" +
	"<h1>A browser, in Go</h1><p>The chrome is syscall/js. Each tab is an iframe. " +
	"Use + for a new tab, or type a URL above.</p></body>"

var (
	doc    js.Value
	strip  js.Value // tab strip
	views  js.Value // stacked iframe area
	addr   js.Value // shared address bar
	tabs   []*tab
	active = -1
	nextID int
)

type tab struct {
	btn, lbl, frame js.Value
	hist            []string
	pos             int
}

func mk(tag string) js.Value { return doc.Call("createElement", tag) }

func btn(label, style string) js.Value {
	b := mk("button")
	b.Set("textContent", label)
	b.Get("style").Set("cssText", "background:#2a2342;color:#cdd2da;border:1px solid #3a3352;cursor:pointer;font:13px monospace;"+style)
	return b
}

// load renders a URL into a tab's iframe. An http(s) page goes through the
// transport (fetchPage): fetched same-origin and rendered sandboxed. Anything
// else (data:, about:, a relative path) is handed straight to the iframe.
func load(t *tab, url string) {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		fetchPage(t, url)
	} else {
		t.frame.Call("removeAttribute", "srcdoc")
		t.frame.Set("src", url)
	}
	if active >= 0 && tabs[active] == t {
		addr.Set("value", url)
	}
}

// fetchPage is the clearnet transport + first transcoding pass. It fetches the
// page over the same-origin /fetch proxy (the tab can't reach it cross-origin),
// then renders it as a sandboxed srcdoc with a <base> so the page's own
// relative URLs resolve. The heavier transcoding — inlining stylesheets and
// images, and the shims that relay the sandboxed page's navigation and fetches
// back through the transport — layers on top of this seam.
func fetchPage(t *tab, url string) {
	g := js.Global()
	enc := g.Get("encodeURIComponent").Invoke(url).String()
	fail := func(msg string) {
		t.frame.Call("removeAttribute", "src")
		t.frame.Set("srcdoc", "<body style='font:14px sans-serif;padding:2em;color:#a33'>"+msg+"</body>")
	}
	var onResp, onText, onErr js.Func
	onErr = js.FuncOf(func(_ js.Value, a []js.Value) any {
		m := "fetch failed"
		if len(a) > 0 {
			m = a[0].Call("toString").String()
		}
		fail(m)
		onResp.Release()
		onText.Release()
		onErr.Release()
		return nil
	})
	onText = js.FuncOf(func(_ js.Value, a []js.Value) any {
		html := a[0].String()
		t.frame.Call("removeAttribute", "src")
		t.frame.Call("setAttribute", "sandbox", "allow-scripts") // sandboxed: no allow-same-origin
		t.frame.Set("srcdoc", "<base href=\""+url+"\">"+html)
		onResp.Release()
		onText.Release()
		onErr.Release()
		return nil
	})
	onResp = js.FuncOf(func(_ js.Value, a []js.Value) any { return a[0].Call("text") })
	g.Call("fetch", "/fetch?url="+enc).Call("then", onResp).Call("then", onText).Call("catch", onErr)
}

func navigate(t *tab, url string) {
	if t.pos >= 0 && t.pos < len(t.hist)-1 {
		t.hist = t.hist[:t.pos+1]
	}
	t.hist = append(t.hist, url)
	t.pos = len(t.hist) - 1
	load(t, url)
}

func activate(i int) {
	active = i
	for j, t := range tabs {
		on := j == i
		if on {
			t.frame.Get("style").Set("display", "block")
			t.btn.Get("style").Set("background", "#2a2342")
		} else {
			t.frame.Get("style").Set("display", "none")
			t.btn.Get("style").Set("background", "transparent")
		}
	}
	addr.Set("value", tabs[i].hist[tabs[i].pos])
}

func onClick(el js.Value, fn func()) {
	el.Call("addEventListener", "click", js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) > 0 {
			a[0].Call("stopPropagation")
		}
		fn()
		return nil
	}))
}

func addTab(url string) {
	nextID++
	t := &tab{}
	t.btn = mk("div")
	t.btn.Get("style").Set("cssText", "display:flex;align-items:center;gap:.4em;max-width:12em;padding:.25em .6em;cursor:pointer;font:11px monospace;color:#cdd2da;border:1px solid #2a2342;border-bottom:0;border-radius:5px 5px 0 0;white-space:nowrap")
	t.lbl = mk("span")
	t.lbl.Set("textContent", "tab "+strconv.Itoa(nextID))
	x := mk("span")
	x.Set("textContent", "×")
	x.Get("style").Set("cssText", "opacity:.6")
	t.btn.Call("appendChild", t.lbl)
	t.btn.Call("appendChild", x)

	t.frame = mk("iframe")
	t.frame.Get("style").Set("cssText", "position:absolute;inset:0;width:100%;height:100%;border:0;background:#fff;display:none")
	if len(tabs) == 0 {
		t.frame.Set("id", "browser-frame") // the first tab's frame, for harnesses
	}

	idx := len(tabs)
	onClick(t.btn, func() { activate(idx) })
	onClick(x, func() { closeTab(idx) })

	strip.Call("insertBefore", t.btn, strip.Get("lastChild")) // before the + button
	views.Call("appendChild", t.frame)
	tabs = append(tabs, t)
	navigate(t, url)
	activate(idx)
}

func closeTab(i int) {
	if len(tabs) <= 1 {
		return // keep at least one tab
	}
	t := tabs[i]
	t.btn.Call("remove")
	t.frame.Call("remove")
	tabs = append(tabs[:i], tabs[i+1:]...)
	// rebind indices by reactivating the neighbour
	if active >= len(tabs) {
		active = len(tabs) - 1
	}
	rebindClicks()
	activate(active)
}

// rebindClicks re-points each tab button at its current index after a removal.
func rebindClicks() {
	for i, t := range tabs {
		idx := i
		nb := t.btn.Call("cloneNode", true) // drop old listeners
		t.btn.Get("parentNode").Call("replaceChild", nb, t.btn)
		t.btn = nb
		onClick(t.btn, func() { activate(idx) })
		onClick(t.btn.Get("lastChild"), func() { closeTab(idx) })
	}
}

func main() {
	doc = js.Global().Get("document")
	root := doc.Call("getElementById", os.Getenv("SHIPYARD_MOUNT"))
	if !root.Truthy() {
		return
	}
	root.Get("style").Set("cssText", "position:absolute;inset:0;display:flex;flex-direction:column;background:#15131c")

	strip = mk("div")
	strip.Get("style").Set("cssText", "display:flex;gap:2px;align-items:flex-end;background:#100d18;border-bottom:1px solid #2a2342;padding:3px 3px 0;min-height:24px")
	plus := btn("+", "padding:.2em .55em;border-radius:5px 5px 0 0")
	onClick(plus, func() { addTab(startPage) })
	strip.Call("appendChild", plus)

	bar := mk("div")
	bar.Get("style").Set("cssText", "display:flex;gap:4px;padding:4px;background:#100d18;border-bottom:1px solid #2a2342")
	back, fwd, reload := btn("◀", "padding:2px 8px"), btn("▶", "padding:2px 8px"), btn("⟳", "padding:2px 8px")
	addr = mk("input")
	addr.Get("style").Set("cssText", "flex:1;background:#0e0c14;color:#cdd2da;border:1px solid #2a2342;padding:2px 8px;font:13px monospace")
	goBtn := btn("Go", "padding:2px 8px")
	for _, el := range []js.Value{back, fwd, reload, addr, goBtn} {
		bar.Call("appendChild", el)
	}

	views = mk("div")
	views.Get("style").Set("cssText", "position:relative;flex:1;min-height:0")

	root.Call("appendChild", strip)
	root.Call("appendChild", bar)
	root.Call("appendChild", views)

	cur := func() *tab {
		if active >= 0 {
			return tabs[active]
		}
		return nil
	}
	onClick(goBtn, func() { navigate(cur(), addr.Get("value").String()) })
	onClick(reload, func() { t := cur(); load(t, t.hist[t.pos]) })
	onClick(back, func() {
		if t := cur(); t.pos > 0 {
			t.pos--
			load(t, t.hist[t.pos])
		}
	})
	onClick(fwd, func() {
		if t := cur(); t.pos < len(t.hist)-1 {
			t.pos++
			load(t, t.hist[t.pos])
		}
	})
	addr.Call("addEventListener", "keydown", js.FuncOf(func(_ js.Value, a []js.Value) any {
		if len(a) > 0 && a[0].Get("key").String() == "Enter" {
			navigate(cur(), addr.Get("value").String())
		}
		return nil
	}))

	addTab(startPage)
	select {}
}
