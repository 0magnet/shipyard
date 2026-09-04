//go:build js && wasm

// Command child is the downstream half of the process-pipe demo: it reads all
// of stdin, writes it back upper-cased to stdout, and exits with code 3 —
// proof that a spawned wasm program has a working stdin, stdout, and a real
// process exit code.
package main

import (
	"io"
	"os"
	"strings"
)

func main() {
	b, _ := io.ReadAll(os.Stdin)
	os.Stdout.Write([]byte(strings.ToUpper(string(b))))
	os.Exit(3)
}
