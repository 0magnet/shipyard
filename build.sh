#!/bin/sh
# Assemble the shipyard demo: shipyard.wasm (the websh shell over jsfs, with go
# on PATH) plus shipwright's toolchain and bottle's runtime, served alongside.
# Everything it writes is regenerable and .gitignore'd — the repo is source.
set -eu
cd "$(dirname "$0")"

echo "shipyard: building shipyard.wasm…"
GOOS=js GOARCH=wasm go build -o shipyard.wasm ./cmd/shipyard

# shipwright supplies the Go toolchain built for js/wasm and the bottle runtime
# (jsfs.js / proc.js / wasm_exec.js). Build it from source next door.
if [ ! -d .shipwright ]; then
	git clone --depth 1 https://github.com/0magnet/shipwright .shipwright
else
	( cd .shipwright && git pull -q --ff-only )
fi
# A workstation builds anything, so seed the whole standard library (stdsrc.sh
# all), not just the demo closure — any program you compile has its imports.
( cd .shipwright && ./build.sh && ./stdsrc.sh all )

for f in go-proc.wasm compile-proc.wasm link-proc.wasm asm-proc.wasm \
         stdsrc.json jsfs.js proc.js wasm_exec.js; do
	cp ".shipwright/$f" .
done

echo "shipyard: ready — go run ./serve and open http://127.0.0.1:8931/demo.html"
