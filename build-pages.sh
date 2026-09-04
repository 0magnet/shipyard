#!/bin/sh
# Assemble the STATIC, GitHub-Pages variant of the shipyard demo into _site/.
#
# Pages serves static files only — none of serve's endpoints (/goproxy, /fetch,
# /std/) exist there — so this build differs from build.sh in one way that
# matters: it harvests the WHOLE standard library eagerly (stdsrc.sh all →
# stdsrc.json, ~97 MB) instead of the lazy skeleton, and ships pages.html (which
# seeds that blob up front) as index.html. The result is the offline
# workstation: boot the terminal, `go build` from the seeded std, `run` it in a
# window — all in the tab, no server. Everything here is regenerable and
# .gitignore'd — the repo is source.
set -eu
cd "$(dirname "$0")"

echo "shipyard/pages: building shipyard.wasm + browser.wasm + the gallery demos…"
GOOS=js GOARCH=wasm go build -o shipyard.wasm ./cmd/shipyard
GOOS=js GOARCH=wasm go build -o browser.wasm ./cmd/browser

# The "What the Go Playground can't do" gallery demos: one self-contained module
# (demo/) that builds offline from seeded std alone. On Pages this is the whole
# point — every demo runs fully client-side, no server component at all.
build_demo() { ( cd demo && GOOS=js GOARCH=wasm GOFLAGS=-mod=mod go build -o "../$2" "./$1" ); }
build_demo server            server.wasm
build_demo netclient         netclient.wasm
build_demo fs                fs.wasm
build_demo timeconc          timeconc.wasm
build_demo procdemo/parent   procparent.wasm
build_demo procdemo/child    procchild.wasm
build_demo gui               gui.wasm

# shipwright supplies the Go toolchain built for js/wasm and the bottle runtime
# (jsfs.js / proc.js / wasm_exec.js) plus stdsrc.sh. Build it from source.
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

# vnet.js (the page's virtual loopback) comes from bottle — the seam the desk's
# in-tab server and browser reach each other through.
if [ ! -d .bottle ]; then
	git clone --depth 1 https://github.com/0magnet/bottle .bottle
else
	( cd .bottle && git pull -q --ff-only )
fi
cp .bottle/vnet.js .bottle/vnet-sw.js .

# demo.json: the whole demo module's source, seeded into the tab at /work/demo
# so the desk can view and rebuild any gallery demo.
gen_demojson() {
	find demo -type f \( -name '*.go' -o -name 'go.mod' \) \
	| while IFS= read -r f; do
		jq -Rs --arg p "/work/demo/${f#demo/}" '{($p): .}' "$f"
	done | jq -s 'add'
}
gen_demojson > demo.json
echo "shipyard/pages: demo.json has $(jq 'length' demo.json) files"

echo "shipyard/pages: harvesting the full standard library (stdsrc.sh all)…"
( cd .shipwright && ./stdsrc.sh all )
cp .shipwright/stdsrc.json .
echo "shipyard/pages: stdsrc.json is $(du -h stdsrc.json | cut -f1)"

echo "shipyard/pages: staging _site/…"
rm -rf _site && mkdir _site
cp pages.html _site/index.html
for f in shipyard.wasm browser.wasm server.wasm netclient.wasm fs.wasm timeconc.wasm \
         procparent.wasm procchild.wasm gui.wasm \
         go-proc.wasm compile-proc.wasm link-proc.wasm asm-proc.wasm vet-proc.wasm \
         jsfs.js proc.js wasm_exec.js vnet.js vnet-sw.js demos.js demo.json stdsrc.json; do
	cp "$f" _site/
done
echo "shipyard/pages: _site ready ($(du -sh _site | cut -f1)) — deploy its contents to Pages"
