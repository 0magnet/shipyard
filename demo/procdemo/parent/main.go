//go:build js && wasm

// Command parent is the process-pipe demo: it spawns /work/procchild.wasm as a
// real child process, pipes a string into the child's stdin, collects the
// child's stdout, and reports what came back plus the child's exit code — all
// inside one browser tab, two wasm instances, one shared filesystem, connected
// by real pipes. It prints to stdout, which the websh terminal shows.
//
//	The Go Playground can't spawn processes — os/exec fails, there is no second
//	program, no pipe, no child exit code.
//
// The child is resolved by absolute path in jsfs, so this works no matter the
// terminal's working directory.
package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"demo/proc"
)

func main() {
	const childPath = "/work/procchild.wasm"
	if _, err := os.Stat(childPath); err != nil {
		fmt.Printf("parent: %s missing (%v)\n", childPath, err)
		os.Exit(1)
	}

	var out bytes.Buffer
	c := proc.Command(childPath)
	c.Stdin = strings.NewReader("processes and pipes, in one browser tab\n")
	c.Stdout = &out
	c.Stderr = &out
	fmt.Printf("parent: spawning %s with a piped stdin…\n", childPath)
	code, err := c.Run()
	if err != nil {
		fmt.Println("parent: spawn error:", err)
		os.Exit(1)
	}
	got := strings.TrimRight(out.String(), "\n")
	fmt.Printf("parent: child exited with code %d\n", code)
	fmt.Printf("parent: child stdout = %q\n", got)

	if got == "PROCESSES AND PIPES, IN ONE BROWSER TAB" && code == 3 {
		fmt.Println("SHIPYARD-PROC-MARKER: spawned a child process, piped stdin/stdout, and read its exit code")
	} else {
		fmt.Printf("SHIPYARD-PROC-FAIL: unexpected result (code=%d, out=%q)\n", code, got)
	}
}
