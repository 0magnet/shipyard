#!/bin/sh
# Assemble the shipyard demo: shipyard.wasm (the websh shell over jsfs, with go
# on PATH) plus shipwright's toolchain and bottle's runtime, served alongside.
# Everything it writes is regenerable and .gitignore'd — the repo is source.
set -eu
cd "$(dirname "$0")"

echo "shipyard: building shipyard.wasm + the demo programs…"
GOOS=js GOARCH=wasm go build -o shipyard.wasm ./cmd/shipyard
GOOS=js GOARCH=wasm go build -o browser.wasm ./cmd/browser

# The gallery demo programs: one self-contained module (demo/) whose only
# non-std imports are its own copies of bottle's vnet and proc drop-ins, so each
# builds offline in the desk from the seeded standard library alone. These are
# the "What the Go Playground can't do" demos, prebuilt so a gallery click
# launches instantly; their source is seeded too (demo.json) for viewing.
echo "shipyard: building the gallery demo programs (demo/)…"
build_demo() { ( cd demo && GOOS=js GOARCH=wasm GOFLAGS=-mod=mod go build -o "../$2" "./$1" ); }
build_demo server            server.wasm      # net/http server on vnet
build_demo netclient         netclient.wasm   # net/http client over vnet
build_demo fs                fs.wasm          # filesystem read/write
build_demo timeconc          timeconc.wasm    # real clock + concurrency
build_demo procdemo/parent   procparent.wasm  # spawn a child, pipe stdio
build_demo procdemo/child    procchild.wasm   # the child it spawns
build_demo gui               gui.wasm         # animated canvas widget

# shipwright supplies the Go toolchain built for js/wasm and the bottle runtime
# (jsfs.js / proc.js / wasm_exec.js). Build it from source next door.
if [ ! -d .shipwright ]; then
	git clone --depth 1 https://github.com/0magnet/shipwright .shipwright
else
	( cd .shipwright && git pull -q --ff-only )
fi
( cd .shipwright && ./build.sh )

for f in go-proc.wasm compile-proc.wasm link-proc.wasm asm-proc.wasm vet-proc.wasm \
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

# demo.json seeds the demo module's SOURCE into the tab's filesystem at
# /work/demo, so the desk can view and rebuild any demo (cd /work/demo && go
# build ./...). Keyed by in-tab path; the whole module is self-contained.
echo "shipyard: generating demo.json (the gallery demo source)…"
gen_demojson() {
	find demo -type f \( -name '*.go' -o -name 'go.mod' \) \
	| while IFS= read -r f; do
		jq -Rs --arg p "/work/demo/${f#demo/}" '{($p): .}' "$f"
	done | jq -s 'add'
}
gen_demojson > demo.json
echo "shipyard: demo.json has $(jq 'length' demo.json) files"

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
