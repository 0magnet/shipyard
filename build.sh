#!/bin/sh
# Assemble the shipyard demo: shipyard.wasm (the websh shell over jsfs, with go
# on PATH) plus shipwright's toolchain and bottle's runtime, served alongside.
# Everything it writes is regenerable and .gitignore'd — the repo is source.
set -eu
cd "$(dirname "$0")"

echo "shipyard: building shipyard.wasm + the demo programs…"
GOOS=js GOARCH=wasm go build -o shipyard.wasm ./cmd/shipyard
GOOS=js GOARCH=wasm go build -o browser.wasm ./cmd/browser

# The vnet demo: a real Go net/http server that listens on bottle's vnet, so the
# netscrape browser reaches it over the in-tab virtual network — no server. It
# is a self-contained module (demo/server carries a copy of bottle's vnet net.*
# drop-ins) so it builds offline in the desk too.
( cd demo/server && GOOS=js GOARCH=wasm GOFLAGS=-mod=mod go build -o ../../server.wasm . )

# shipwright supplies the Go toolchain built for js/wasm and the bottle runtime
# (jsfs.js / proc.js / wasm_exec.js). Build it from source next door.
if [ ! -d .shipwright ]; then
	git clone --depth 1 https://github.com/0magnet/shipwright .shipwright
else
	( cd .shipwright && git pull -q --ff-only )
fi
( cd .shipwright && ./build.sh )

for f in go-proc.wasm compile-proc.wasm link-proc.wasm asm-proc.wasm \
         jsfs.js proc.js wasm_exec.js; do
	cp ".shipwright/$f" .
done

# vnet.js (the page's virtual loopback) and its optional service-worker bridge
# come from bottle. jsfs.js/proc.js above are bottle's too (shipwright vendors
# them); vnet.js is not, so pull it straight from bottle.
if [ ! -d .bottle ]; then
	git clone --depth 1 https://github.com/0magnet/bottle .bottle
else
	( cd .bottle && git pull -q --ff-only )
fi
cp .bottle/vnet.js .bottle/vnet-sw.js .

# demo.json seeds the vnet server's SOURCE into the tab's filesystem, so the
# desk can rebuild it (cd /work/server && go build -o server.wasm .). Keyed by
# the in-tab path; the whole module is self-contained (no external imports).
echo "shipyard: generating demo.json (the vnet server demo source)…"
jq -n \
	--rawfile gomod demo/server/go.mod \
	--rawfile main  demo/server/main.go \
	--rawfile vjs   demo/server/vnet/vnet_js.go \
	--rawfile vnat  demo/server/vnet/vnet_native.go \
	'{"/work/server/go.mod":$gomod,"/work/server/main.go":$main,"/work/server/vnet/vnet_js.go":$vjs,"/work/server/vnet/vnet_native.go":$vnat}' \
	> demo.json

# The standard library is served lazily (serve's /std/ endpoint, rooted at
# GOROOT) rather than shipped as one big blob: stdskel.json is just each std
# source file's path and size, so the page seeds a cheap skeleton and fetches a
# file's bytes only when a build first imports it.
echo "shipyard: generating stdskel.json (the lazy std skeleton)…"
GOROOT=$(go env GOROOT)
{
	find "$GOROOT/src" \( -name '*.go' -o -name '*.s' -o -name '*.h' \) \
		-not -name '*_test.go' -not -path '*/testdata/*'
	ls "$GOROOT"/pkg/include/*.h
} | while IFS= read -r f; do
	printf '/goroot/%s\t%s\n' "${f#"$GOROOT"/}" "$(stat -c%s "$f")"
done | jq -Rn '[inputs | split("\t") | {(.[0]): (.[1]|tonumber)}] | add' > stdskel.json
echo "shipyard: stdskel.json has $(jq 'length' stdskel.json) files ($(du -h stdskel.json|cut -f1))"

echo "shipyard: ready — go run ./serve and open http://127.0.0.1:8931/demo.html"
