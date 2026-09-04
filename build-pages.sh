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

echo "shipyard/pages: building shipyard.wasm + browser.wasm…"
GOOS=js GOARCH=wasm go build -o shipyard.wasm ./cmd/shipyard
GOOS=js GOARCH=wasm go build -o browser.wasm ./cmd/browser

# shipwright supplies the Go toolchain built for js/wasm and the bottle runtime
# (jsfs.js / proc.js / wasm_exec.js) plus stdsrc.sh. Build it from source.
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

echo "shipyard/pages: harvesting the full standard library (stdsrc.sh all)…"
( cd .shipwright && ./stdsrc.sh all )
cp .shipwright/stdsrc.json .
echo "shipyard/pages: stdsrc.json is $(du -h stdsrc.json | cut -f1)"

echo "shipyard/pages: staging _site/…"
rm -rf _site && mkdir _site
cp pages.html _site/index.html
for f in shipyard.wasm browser.wasm go-proc.wasm compile-proc.wasm link-proc.wasm \
         asm-proc.wasm jsfs.js proc.js wasm_exec.js stdsrc.json; do
	cp "$f" _site/
done
echo "shipyard/pages: _site ready ($(du -sh _site | cut -f1)) — deploy its contents to Pages"
