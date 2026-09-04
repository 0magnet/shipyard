//go:build js && wasm

// Command fs is the filesystem demo: an ordinary Go program using os and
// io/ioutil-era std calls to create a directory, write a file, read it back,
// stat it, and list the directory — all on bottle's in-tab jsfs, the same
// filesystem the compiler reads. It prints what it did to stdout, which the
// websh terminal shows.
//
//	The Go Playground has no filesystem — os.Create, os.WriteFile, os.ReadDir
//	all fail; there is nowhere to write.
//
// Persistence (a file surviving a page reload via jsfs.persist → IndexedDB) is
// proved separately, at the page level, in the selftest.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func main() {
	dir := "/work/fsdemo"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Println("fs: mkdir failed:", err)
		os.Exit(1)
	}
	path := filepath.Join(dir, "note.txt")
	content := []byte("written to /work by a real Go program, on a real filesystem, in a browser tab.\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		fmt.Println("fs: write failed:", err)
		os.Exit(1)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("fs: read failed:", err)
		os.Exit(1)
	}
	fi, err := os.Stat(path)
	if err != nil {
		fmt.Println("fs: stat failed:", err)
		os.Exit(1)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("fs: readdir failed:", err)
		os.Exit(1)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	fmt.Printf("fs: wrote %s (%d bytes, mode %v)\n", path, fi.Size(), fi.Mode().Perm())
	fmt.Printf("fs: read it back: %q\n", string(got))
	fmt.Printf("fs: %s contains %v\n", dir, names)
	if string(got) == string(content) {
		fmt.Printf("SHIPYARD-FS-MARKER: wrote and read %d bytes at %s on the in-tab filesystem\n", fi.Size(), path)
	} else {
		fmt.Println("SHIPYARD-FS-FAIL: read back did not match what was written")
	}
}
