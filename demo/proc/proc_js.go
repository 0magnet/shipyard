//go:build js && wasm

// Package proc is the Go adapter for bottle's process layer (proc.js): spawn
// another wasm program from the page filesystem as a child that shares this
// tab's fs and vnet, wire its stdio, and wait for it to exit. It is a
// self-contained copy of github.com/0magnet/bottle/proc so this demo module
// builds offline in the desk from the seeded standard library alone.
//
// os/exec on js/wasm fails in syscall.StartProcess (ENOSYS); Cmd here is the
// honest tab primitive — proc.Command(name, args...).Run() — shaped like
// os/exec so the spawn-a-real-child story reads the same as on a host.
package proc

import (
	"errors"
	"io"
	"syscall/js"
)

// Cmd is one child process: a program in the page filesystem plus the argv,
// env, cwd and stdio to run it with. Zero value is not useful; use Command.
type Cmd struct {
	Path   string   // argv[0]: resolved against jsfs (abs, cwd-relative, or PATH)
	Args   []string // argv, including Args[0] == the program name
	Env    []string // "KEY=value"
	Dir    string   // working directory; empty means the page cwd
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Command builds a Cmd, mirroring os/exec.Command's shape.
func Command(name string, arg ...string) *Cmd {
	return &Cmd{Path: name, Args: append([]string{name}, arg...)}
}

// Run spawns the child and blocks until it exits, returning its exit code and
// any spawn error.
func (c *Cmd) Run() (int, error) {
	proc := js.Global().Get("proc")
	if !proc.Truthy() {
		return -1, errors.New("proc: proc.js not loaded on this page")
	}

	opts := js.Global().Get("Object").New()
	argv := js.Global().Get("Array").New()
	for _, a := range c.Args {
		argv.Call("push", a)
	}
	opts.Set("argv", argv)
	if c.Dir != "" {
		opts.Set("cwd", c.Dir)
	}
	env := js.Global().Get("Object").New()
	for _, kv := range c.Env {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				env.Set(kv[:i], kv[i+1:])
				break
			}
		}
	}
	opts.Set("env", env)

	if c.Stdout != nil {
		opts.Set("stdout", sinkFunc(c.Stdout))
	}
	if c.Stderr != nil {
		opts.Set("stderr", sinkFunc(c.Stderr))
	}
	if c.Stdin != nil {
		opts.Set("stdin", sourceFunc(c.Stdin))
	}

	res := proc.Call("spawn", opts)
	code, err := await(res.Get("exited"))
	if err != nil {
		return -1, err
	}
	return code.Int(), nil
}

// sinkFunc adapts an io.Writer to proc.js's stdout/stderr callback.
func sinkFunc(w io.Writer) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		b := make([]byte, args[0].Get("length").Int())
		js.CopyBytesToGo(b, args[0])
		w.Write(b) //nolint:errcheck
		return nil
	})
}

// sourceFunc adapts an io.Reader to proc.js's stdin puller.
func sourceFunc(r io.Reader) js.Func {
	buf := make([]byte, 4096)
	return js.FuncOf(func(_ js.Value, _ []js.Value) any {
		n, err := r.Read(buf)
		if n == 0 && err != nil {
			return js.Null() // EOF
		}
		out := js.Global().Get("Uint8Array").New(n)
		js.CopyBytesToJS(out, buf[:n])
		return out
	})
}

// await blocks a goroutine on a JS promise.
func await(promise js.Value) (js.Value, error) {
	type result struct {
		v   js.Value
		err error
	}
	ch := make(chan result, 1)
	then := js.FuncOf(func(_ js.Value, args []js.Value) any {
		var v js.Value
		if len(args) > 0 {
			v = args[0]
		}
		ch <- result{v: v}
		return nil
	})
	defer then.Release()
	catch := js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := "promise rejected"
		if len(args) > 0 {
			msg = args[0].Call("toString").String()
		}
		ch <- result{err: errors.New(msg)}
		return nil
	})
	defer catch.Release()
	promise.Call("then", then).Call("catch", catch)
	r := <-ch
	return r.v, r.err
}
