//go:build !(js && wasm)

// Package proc native stub: spawning a page wasm process only exists in the
// browser (proc.js). The stub keeps the API compiling off js/wasm so
// go build ./... works on any host; Run reports the platform mismatch.
package proc

import (
	"errors"
	"io"
)

// Cmd mirrors the js/wasm Cmd shape.
type Cmd struct {
	Path   string
	Args   []string
	Env    []string
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Command builds a Cmd.
func Command(name string, arg ...string) *Cmd {
	return &Cmd{Path: name, Args: append([]string{name}, arg...)}
}

// Run reports that page processes need the js/wasm build.
func (c *Cmd) Run() (int, error) {
	return -1, errors.New("proc: page processes are only available on js/wasm")
}
