// serve hosts the shipwright page and the network egress the tab's cmd/go
// needs. cmd/go's downloads go through net/http, which on js/wasm is the
// browser Fetch API: a tab can call its own origin but not proxy.golang.org
// or sum.golang.org (CORS). serve stands in for both, same-origin:
//
//   /goproxy/                       → the Go module proxy (proxy.golang.org)
//   /goproxy/sumdb/sum.golang.org/  → the checksum database (sum.golang.org),
//                                     mirrored per the proxy protocol so the
//                                     tab can run with GOSUMDB on
//
// It fetches upstream server-side and *follows redirects itself*, so the
// browser only ever sees a same-origin 200 — proxy.golang.org serves large
// module zips as a 302 to a CDN on another origin, which a passed-through
// redirect would make the tab's fetch chase across origins and CORS would
// block. The tab builds with GOPROXY=<origin>/goproxy and the stock GOSUMDB,
// and both the fetch and its verification cross same-origin.
//
//	go run ./serve            # :8931, / = static, /goproxy = proxy + sumdb
//	go run ./serve -addr :9000 -upstream https://goproxy.cn
package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"strings"
)

// forward fetches base+the request's path (and query) upstream, following
// redirects, and copies the final response back to the caller same-origin.
func forward(base string) http.HandlerFunc {
	client := &http.Client{} // follows up to 10 redirects by default
	return func(w http.ResponseWriter, r *http.Request) {
		url := base + r.URL.Path
		if r.URL.RawQuery != "" {
			url += "?" + r.URL.RawQuery
		}
		resp, err := client.Get(url)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

func main() {
	addr := flag.String("addr", ":8931", "listen address")
	dir := flag.String("dir", ".", "directory to serve")
	upstream := flag.String("upstream", "https://proxy.golang.org", "Go module proxy to pass through to")
	sumdb := flag.String("sumdb", "https://sum.golang.org", "checksum database to mirror")
	flag.Parse()

	modFwd := forward(*upstream)
	sumFwd := forward(*sumdb)

	mux := http.NewServeMux()

	// The proxy sumdb-mirror protocol: /supported answered here says "yes, ask
	// me for sumdb data too"; the rest is forwarded to sum.golang.org with the
	// /goproxy/sumdb/<host> prefix stripped, leaving /lookup, /tile, /latest.
	const sumPrefix = "/goproxy/sumdb/sum.golang.org"
	mux.HandleFunc(sumPrefix+"/supported", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle(sumPrefix+"/", http.StripPrefix(sumPrefix, sumFwd))

	// Everything else under /goproxy is the module proxy. StripPrefix leaves
	// the leading slash, so /goproxy/mod/@v/list becomes /mod/@v/list — the
	// module-proxy protocol path upstream expects.
	mux.Handle("/goproxy/", http.StripPrefix("/goproxy", modFwd))

	mux.Handle("/", http.FileServer(http.Dir(*dir)))

	log.Printf("shipwright: http://localhost%s  (/goproxy → %s, sumdb → %s)",
		*addr, strings.TrimPrefix(*upstream, "https://"), strings.TrimPrefix(*sumdb, "https://"))
	log.Fatal(http.ListenAndServe(*addr, mux))
}
