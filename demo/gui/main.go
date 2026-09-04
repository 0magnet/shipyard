//go:build js && wasm

// Command gui is the graphical demo: a Go program that draws an animated
// widget into its window with syscall/js — a live analog clock face plus a
// bouncing dot, redrawn every animation frame on an HTML <canvas>, with the
// real time under it. It draws into the element whose id arrives in
// $SHIPYARD_MOUNT (set by the desk's `run` command).
//
//	The Go Playground is text-stdout only — no window, no canvas, no drawing,
//	no animation, no interaction.
package main

import (
	"math"
	"os"
	"syscall/js"
	"time"
)

func main() {
	doc := js.Global().Get("document")
	el := doc.Call("getElementById", os.Getenv("SHIPYARD_MOUNT"))
	if !el.Truthy() {
		return
	}
	el.Get("style").Set("cssText", "background:#0e0c14;height:100%;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:.6em;font:14px ui-monospace,monospace;color:#9d7cff")

	const size = 240
	canvas := doc.Call("createElement", "canvas")
	canvas.Set("width", size)
	canvas.Set("height", size)
	canvas.Get("style").Set("cssText", "background:#141020;border-radius:12px")
	el.Call("appendChild", canvas)

	status := doc.Call("createElement", "div")
	el.Call("appendChild", status)

	ctx := canvas.Call("getContext", "2d")
	cx, cy, r := float64(size)/2, float64(size)/2, float64(size)/2-18

	var frame js.Func
	frame = js.FuncOf(func(js.Value, []js.Value) any {
		now := time.Now()

		ctx.Set("fillStyle", "#141020")
		ctx.Call("fillRect", 0, 0, size, size)

		// clock face
		ctx.Set("strokeStyle", "#3a3350")
		ctx.Set("lineWidth", 2)
		ctx.Call("beginPath")
		ctx.Call("arc", cx, cy, r, 0, 2*math.Pi)
		ctx.Call("stroke")

		// hands, from the REAL local time
		h := float64(now.Hour()%12) + float64(now.Minute())/60
		m := float64(now.Minute()) + float64(now.Second())/60
		s := float64(now.Second()) + float64(now.Nanosecond())/1e9
		hand := func(frac, length float64, color string, width float64) {
			ang := frac*2*math.Pi - math.Pi/2
			ctx.Set("strokeStyle", color)
			ctx.Set("lineWidth", width)
			ctx.Call("beginPath")
			ctx.Call("moveTo", cx, cy)
			ctx.Call("lineTo", cx+length*math.Cos(ang), cy+length*math.Sin(ang))
			ctx.Call("stroke")
		}
		hand(h/12, r*0.5, "#cdd2da", 4)
		hand(m/60, r*0.72, "#9d7cff", 3)
		hand(s/60, r*0.86, "#7ce0b0", 1.5)

		// a dot bouncing on the second hand's tip, for obvious animation
		ang := s/60*2*math.Pi - math.Pi/2
		ctx.Set("fillStyle", "#7ce0b0")
		ctx.Call("beginPath")
		ctx.Call("arc", cx+r*0.86*math.Cos(ang), cy+r*0.86*math.Sin(ang), 5, 0, 2*math.Pi)
		ctx.Call("fill")

		status.Set("textContent", now.Format("15:04:05"))
		js.Global().Call("requestAnimationFrame", frame)
		return nil
	})
	js.Global().Call("requestAnimationFrame", frame)
	select {}
}
