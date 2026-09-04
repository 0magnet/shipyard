//go:build js && wasm

// Command netclient is the vnet HTTP CLIENT demo: a real Go net/http client
// that dials the in-tab Go server (run server.wasm, listening on vnet:8080)
// over bottle's virtual loopback and prints the response body to stdout — which
// the websh terminal drains back into the shell. Server AND client are both
// ordinary Go programs, two separate wasm processes in one browser tab, talking
// over a real net.Conn. No browser, no server-side proxy, no network.
//
//	The Go Playground has no network at all — no Listen, no Dial, no client.
//
// Build and run it in the desk:
//
//	cd /work/demo && go build -o /work/netclient.wasm ./netclient
//	run server.wasm          # start the server first
//	./netclient.wasm         # exec it in the terminal to see the body printed
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"demo/vnet"
)

func main() {
	// An ordinary *http.Client, except its transport dials over vnet: the
	// DialContext hook hands net/http a vnet net.Conn instead of a kernel
	// socket. Everything above this — the Client, the Request, reading the
	// Body — is unmodified standard library.
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, network, addr string) (net.Conn, error) {
				return vnet.DialTimeout(network, addr, 8*time.Second)
			},
		},
	}

	const url = "http://127.0.0.1:8080/ping"
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stdout, "netclient: GET %s failed: %v\n(is the server running? try `run server.wasm` first)\n", url, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stdout, "netclient: read body failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "netclient: GET %s -> %s, %d bytes\n", url, resp.Status, len(body))
	fmt.Fprintf(os.Stdout, "netclient: body = %q\n", string(body))
	// The server answers /ping with a line carrying SHIPYARD-VNET-PAGE; echo a
	// self-describing marker the selftest greps for in the terminal.
	fmt.Fprintf(os.Stdout, "SHIPYARD-NETCLIENT-MARKER: a Go net/http client fetched %d bytes from a Go server over vnet\n", len(body))
}
