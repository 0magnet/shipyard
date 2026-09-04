#!/bin/sh
# Assemble the shipyard demo: shipyard.wasm (the websh shell over jsfs, with go
# on PATH) plus shipwright's toolchain and bottle's runtime, served alongside.
# Everything it writes is regenerable and .gitignore'd — the repo is source.
set -eu
cd "$(dirname "$0")"

echo "shipyard: building shipyard.wasm + the demo programs…"
GOOS=js GOARCH=wasm go build -o shipyard.wasm ./cmd/shipyard
GOOS=js GOARCH=wasm go build -o browser.wasm ./cmd/browser

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
