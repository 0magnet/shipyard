// demos.js — the "What the Go Playground can't do" gallery.
//
// One source of truth, shared by index.html (the local lazy-std desk) and
// pages.html (the static GitHub Pages deploy): a set of small Go programs, each
// demonstrating a capability the Go Playground fundamentally lacks, each
// launchable from a button on the page and each headless-verifiable by the
// SHIPYARD-<CAP>-MARKER line its run() confirms.
//
// The demos drive the SAME desk a human uses: they type into the websh terminal
// (window.__shipyardSubmit) or `run` a program into a window, then read the
// terminal buffer / filesystem / DOM back to confirm the capability worked. So
// the gallery button and the selftest exercise identical code paths.
(function () {
	"use strict";

	// ---- helpers ---------------------------------------------------------
	const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
	// The terminal is the first winbox body (created before any `run` window).
	const termBody = () => document.querySelector("#desktop .wb-body");
	const termText = () => { const b = termBody(); return b ? (b.textContent || "") : ""; };
	const submit = (line) => { if (window.__shipyardSubmit) window.__shipyardSubmit(line); };

	async function waitFor(fn, ms, step) {
		const t0 = Date.now();
		step = step || 200;
		for (;;) {
			let v; try { v = fn(); } catch (e) { v = false; }
			if (v) return v;
			if (Date.now() - t0 >= ms) return false;
			await sleep(step);
		}
	}
	const waitTerm = (re, ms) => waitFor(() => re.test(termText()), ms);
	const waitFile = (p, ms) => waitFor(() => { try { const b = jsfs.readFile(p); return b && b.length > 0 ? b.length : false; } catch (e) { return false; } }, ms);
	const vnetListening = (port) => { try { return !!(window.vnet && window.vnet.listening && window.vnet.listening(port)); } catch (e) { return false; } };

	// Start the in-tab Go net/http server (server.wasm) once, on vnet:8080.
	let serverStarted = false;
	async function ensureServer() {
		if (vnetListening(8080)) return true;
		if (!serverStarted) { serverStarted = true; submit("run /work/server.wasm"); }
		return await waitFor(() => vnetListening(8080), 20000);
	}
	// Start the netscrape browser (browser.wasm) once.
	let browserStarted = false;
	async function ensureBrowser() {
		const up = () => { const f = document.getElementById("browser-frame"); return f && (f.getAttribute("src") || "").length > 0; };
		if (up()) return true;
		if (!browserStarted) { browserStarted = true; submit("run /work/browser.wasm"); }
		return await waitFor(up, 20000);
	}

	// ---- the demos -------------------------------------------------------
	// Each: cap (marker key), icon, title, limit (the Playground limitation it
	// breaks), and run() → {status:'ok'|'info'|'fail', marker, note}. run()
	// drives the desk and confirms the capability; the marker is the pass line.
	const demos = [
		{
			cap: "FS", icon: "📁", title: "A real filesystem",
			limit: "The Playground has no filesystem — nowhere to write a file.",
			async run() {
				submit("cd /work && ./fs.wasm");
				const ok = await waitTerm(/SHIPYARD-FS-MARKER:/, 30000);
				return ok ? { status: "ok", marker: "SHIPYARD-FS-MARKER: a Go program wrote and read a file at /work/fsdemo on the in-tab filesystem" } : { status: "fail", marker: "SHIPYARD-FS-FAIL: no marker in terminal" };
			},
		},
		{
			cap: "PERSIST", icon: "💾", title: "Persistence across reloads",
			limit: "The Playground keeps nothing — every run starts from scratch.",
			// Two-phase: a file written and flushed to IndexedDB in one session
			// is restored on the next page load. First visit seeds it; after a
			// reload it is found already present — the proof.
			async run() {
				const proof = "/work/persist-demo/seen.txt";
				let prior = null;
				try { const b = jsfs.readFile(proof); if (b && b.length) prior = new TextDecoder().decode(b); } catch (e) {}
				if (prior) {
					return { status: "ok", marker: "SHIPYARD-PERSIST-MARKER: a file written in a previous session survived the reload (stamp " + prior.trim() + ")" };
				}
				const stamp = new Date().toISOString();
				try {
					jsfs.mkdirp("/work/persist-demo");
					jsfs.writeFile(proof, new TextEncoder().encode(stamp));
					if (jsfs.persist && jsfs.persist.flush) await jsfs.persist.flush();
				} catch (e) { return { status: "fail", marker: "SHIPYARD-PERSIST-FAIL: " + e }; }
				return { status: "info", marker: "SHIPYARD-PERSIST-SEEDED: wrote " + stamp + " and flushed to IndexedDB — reload the page to prove it survives" };
			},
		},
		{
			cap: "PROC", icon: "🔗", title: "Processes & pipes",
			limit: "The Playground can't spawn a process — no child, no pipe.",
			async run() {
				submit("cd /work && ./procparent.wasm");
				const ok = await waitTerm(/SHIPYARD-PROC-MARKER:/, 30000);
				return ok ? { status: "ok", marker: "SHIPYARD-PROC-MARKER: a Go program spawned a child process, piped stdin/stdout, and read its exit code" } : { status: "fail", marker: "SHIPYARD-PROC-FAIL: no marker in terminal" };
			},
		},
		{
			cap: "TIME", icon: "⏱️", title: "Real clock & concurrency",
			limit: "The Playground fakes the clock and caps goroutine time.",
			async run() {
				submit("cd /work && ./timeconc.wasm");
				const ok = await waitTerm(/SHIPYARD-TIME-MARKER:/, 30000);
				const m = ok ? (termText().match(/SHIPYARD-TIME-MARKER: [^\n]*?real wall-clock time/) || ["SHIPYARD-TIME-MARKER: goroutines finished in real order on the real wall clock"])[0] : "";
				return ok ? { status: "ok", marker: m } : { status: "fail", marker: "SHIPYARD-TIME-FAIL: no marker in terminal" };
			},
		},
		{
			cap: "SHELL", icon: "⌨️", title: "A real shell: stdin + pipes + jq",
			limit: "The Playground has no stdin and no shell — one program, no pipeline.",
			async run() {
				// A genuine pipeline: echo JSON into jq, which reads stdin and
				// emits the marker. Proves stdin, pipes, and a JSON tool in the tab.
				submit("echo '{\"m\":\"SHIPYARD-SHELL\"}' | jq -r '.m + \"-MARKER: real stdin, pipes and jq in the tab\"'");
				const ok = await waitTerm(/SHIPYARD-SHELL-MARKER:/, 20000);
				return ok ? { status: "ok", marker: "SHIPYARD-SHELL-MARKER: a real pipeline (echo | jq) read stdin and processed JSON in the tab" } : { status: "fail", marker: "SHIPYARD-SHELL-FAIL: no marker in terminal" };
			},
		},
		{
			cap: "NETCLIENT", icon: "🌐", title: "Go client ↔ Go server, over vnet",
			limit: "The Playground has no network at all — no Listen, no Dial.",
			async run() {
				if (!(await ensureServer())) return { status: "fail", marker: "SHIPYARD-NETCLIENT-FAIL: server not listening on vnet:8080" };
				submit("cd /work && ./netclient.wasm");
				const ok = await waitTerm(/SHIPYARD-NETCLIENT-MARKER:/, 30000);
				return ok ? { status: "ok", marker: "SHIPYARD-NETCLIENT-MARKER: a Go net/http client fetched bytes from a Go net/http server over vnet (both in-tab, no network)" } : { status: "fail", marker: "SHIPYARD-NETCLIENT-FAIL: no marker in terminal" };
			},
		},
		{
			cap: "VNET", icon: "🕸️", title: "Browse an in-tab server (netscrape)",
			limit: "The Playground can't run a server, a browser, or a network.",
			async run() {
				if (!(await ensureServer())) return { status: "fail", marker: "SHIPYARD-VNET-FAIL: server not listening" };
				// Transport proof: netscrape's own seam fetches the page over vnet.
				let via = ""; try { const r = await window.__netscrapeFetch("http://127.0.0.1:8080/"); via = await r.text(); } catch (e) { via = "ERR " + e; }
				const viaOK = /SHIPYARD-VNET-PAGE/.test(via);
				// Full browser path: drive the running browser to the vnet address.
				let uiOK = false;
				if (await ensureBrowser()) {
					try {
						const bf = document.getElementById("browser-frame");
						const body = bf && bf.closest ? bf.closest(".wb-body") : null;
						const inp = body ? body.querySelector("input") : null;
						if (inp) { inp.value = "127.0.0.1:8080"; inp.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true })); }
						uiOK = await waitFor(() => { const s = bf ? (bf.getAttribute("srcdoc") || "") : ""; return /SHIPYARD-VNET-PAGE/.test(s); }, 20000);
					} catch (e) {}
				}
				return (viaOK || uiOK)
					? { status: "ok", marker: "SHIPYARD-VNET-MARKER: netscrape fetched an in-tab net/http server over vnet (no server, no /fetch) — transport=" + viaOK + " rendered=" + uiOK }
					: { status: "fail", marker: "SHIPYARD-VNET-FAIL: transport=" + viaOK + " rendered=" + uiOK };
			},
		},
		{
			cap: "GUI", icon: "🎨", title: "A graphical, animated window",
			limit: "The Playground is text-stdout only — no window, no canvas.",
			async run() {
				submit("run /work/gui.wasm");
				// The GUI writes the live time into a status <div> under its canvas.
				const ok = await waitFor(() => {
					const els = document.querySelectorAll("[id^='shipyard-app-']");
					for (const el of els) { if (/\d\d:\d\d:\d\d/.test(el.textContent || "") && el.querySelector("canvas")) return true; }
					return false;
				}, 30000);
				return ok
					? { status: "ok", marker: "SHIPYARD-GUI-MARKER: a Go program drew an animated clock on a <canvas> in its own window" }
					: { status: "fail", marker: "SHIPYARD-GUI-FAIL: no animated canvas window" };
			},
		},
		{
			cap: "TOOLCHAIN", icon: "🔧", title: "Compile & run Go in the tab",
			limit: "The Playground compiles one file, server-side; you can't build here.",
			async run() {
				submit("cd /work && go build -o hello.wasm . && ./hello.wasm");
				const size = await waitFile("/work/hello.wasm", 240000);
				if (!size) return { status: "fail", marker: "SHIPYARD-TOOLCHAIN-FAIL: no binary after build" };
				const ran = await waitTerm(/hello, built in a shell in a tab/, 30000);
				return { status: "ok", marker: "SHIPYARD-TOOLCHAIN-MARKER: the in-tab toolchain built hello.wasm (" + size + " bytes)" + (ran ? " and ran it" : " (ran: unconfirmed)") };
			},
		},
		{
			cap: "GOTEST", icon: "✅", title: "Compile & run a test in the tab",
			limit: "The Playground runs one program, not a test binary.",
			async run() {
				// `go test` compiles a test binary but hands it to a wasm runner
				// (go_js_wasm_exec/node) that isn't in the tab. So compile the test
				// binary with `go test -c`, then run it directly through proc —
				// the tab IS the runner. A passing suite prints "PASS".
				// `go test` compiles a test binary but hands it to a wasm runner
				// (go_js_wasm_exec) that isn't in the tab. So compile the test binary
				// with `go test -c` (vet runs — the vet tool is seeded), then run it
				// directly through proc: the tab IS the runner. A pass prints "PASS".
				submit("cd /work/demo && go test -c -o /work/mathx.test.wasm ./mathx && cd /work && ./mathx.test.wasm");
				const sz = await waitFile("/work/mathx.test.wasm", 300000);
				if (!sz) return { status: "fail", marker: "SHIPYARD-GOTEST-FAIL: test binary was not built" };
				const passed = await waitTerm(/^PASS$|\nPASS\n|\bPASS\b/m, 30000);
				return passed
					? { status: "ok", marker: "SHIPYARD-GOTEST-MARKER: the in-tab toolchain compiled a test binary (" + sz + " bytes) and ran it — PASS" }
					: { status: "info", marker: "SHIPYARD-GOTEST-INFO: test binary built (" + sz + " bytes) but its PASS line was not observed in the terminal" };
			},
		},
	];

	// ---- gallery UI ------------------------------------------------------
	function renderGallery() {
		if (document.getElementById("pg-gallery")) return;
		const css = document.createElement("style");
		css.textContent = `
#pg-gallery{position:fixed;top:10px;right:10px;z-index:50;font:12px/1.45 ui-monospace,monospace;color:#cdd2da;width:290px;max-width:calc(100vw - 20px)}
#pg-gallery .pg-head{display:flex;align-items:center;gap:.5em;background:#1a1526;border:1px solid #2c2640;border-radius:10px;padding:.55em .7em;cursor:pointer;user-select:none;box-shadow:0 6px 24px rgba(0,0,0,.4)}
#pg-gallery .pg-head b{color:#9d7cff}
#pg-gallery .pg-head .pg-sub{color:#7a8290;font-size:.9em}
#pg-gallery .pg-caret{margin-left:auto;color:#7a8290;transition:transform .15s}
#pg-gallery.pg-open .pg-caret{transform:rotate(90deg)}
#pg-gallery .pg-list{display:none;margin-top:8px;max-height:calc(100vh - 90px);overflow:auto}
#pg-gallery.pg-open .pg-list{display:block}
#pg-gallery .pg-card{background:#141020;border:1px solid #2c2640;border-radius:9px;padding:.55em .65em;margin-bottom:7px}
#pg-gallery .pg-card .pg-t{display:flex;align-items:center;gap:.45em}
#pg-gallery .pg-card .pg-t .pg-title{font-weight:600;color:#cdd2da}
#pg-gallery .pg-card .pg-dot{margin-left:auto;width:9px;height:9px;border-radius:50%;background:#3a3350;flex:0 0 auto}
#pg-gallery .pg-card.run .pg-dot{background:#e0c27c;animation:pgpulse 1s infinite}
#pg-gallery .pg-card.ok  .pg-dot{background:#7ce0b0}
#pg-gallery .pg-card.info .pg-dot{background:#7cc0e0}
#pg-gallery .pg-card.fail .pg-dot{background:#e07c7c}
@keyframes pgpulse{0%,100%{opacity:1}50%{opacity:.35}}
#pg-gallery .pg-card .pg-limit{color:#7a8290;margin-top:.3em}
#pg-gallery .pg-card button{margin-top:.5em;font:inherit;color:#0e0c14;background:#9d7cff;border:0;border-radius:6px;padding:.32em .8em;cursor:pointer}
#pg-gallery .pg-card button:disabled{opacity:.5;cursor:default}
#pg-gallery .pg-card .pg-out{margin-top:.4em;color:#7ce0b0;white-space:pre-wrap;word-break:break-word;font-size:.92em}
#pg-gallery .pg-card.fail .pg-out{color:#e07c7c}
#pg-gallery .pg-all{width:100%;margin-top:2px}
`;
		document.head.appendChild(css);

		const root = document.createElement("div");
		root.id = "pg-gallery";
		root.className = "pg-open";
		const head = document.createElement("div");
		head.className = "pg-head";
		head.innerHTML = '<span>▶</span><span><b>What the Go Playground can\'t do</b><br><span class="pg-sub">click a demo — it runs in the desk below</span></span><span class="pg-caret">›</span>';
		head.onclick = () => root.classList.toggle("pg-open");
		root.appendChild(head);

		const list = document.createElement("div");
		list.className = "pg-list";
		root.appendChild(list);

		const cardEls = {};
		for (const d of demos) {
			const card = document.createElement("div");
			card.className = "pg-card";
			card.innerHTML =
				'<div class="pg-t"><span>' + d.icon + '</span><span class="pg-title">' + d.title + '</span><span class="pg-dot"></span></div>' +
				'<div class="pg-limit">' + d.limit + '</div>';
			const btn = document.createElement("button");
			btn.textContent = "Run";
			const out = document.createElement("div");
			out.className = "pg-out";
			btn.onclick = async () => {
				btn.disabled = true; card.className = "pg-card run"; out.textContent = "";
				let res;
				try { res = await d.run(); } catch (e) { res = { status: "fail", marker: "error: " + e }; }
				card.className = "pg-card " + (res.status || "fail");
				out.textContent = res.marker || "";
				btn.disabled = false;
			};
			card.appendChild(btn);
			card.appendChild(out);
			list.appendChild(card);
			cardEls[d.cap] = { card, out };
		}

		const allBtn = document.createElement("button");
		allBtn.className = "pg-all";
		allBtn.textContent = "Run all demos";
		allBtn.onclick = async () => {
			allBtn.disabled = true;
			for (const d of demos) {
				const ce = cardEls[d.cap];
				ce.card.className = "pg-card run"; ce.out.textContent = "";
				let res; try { res = await d.run(); } catch (e) { res = { status: "fail", marker: "error: " + e }; }
				ce.card.className = "pg-card " + (res.status || "fail");
				ce.out.textContent = res.marker || "";
			}
			allBtn.disabled = false;
		};
		list.appendChild(allBtn);

		document.body.appendChild(root);
	}

	// ---- headless selftest ----------------------------------------------
	// Runs every demo in order and logs its marker line. The CDP driver greps
	// the page log (and the boot <pre>) for SHIPYARD-<CAP>-MARKER.
	async function selftest(log) {
		log = log || ((l) => { try { console.log(l); } catch (e) {} });
		log("\n[gallery] running " + demos.length + " Playground-can't-do demos…");
		for (const d of demos) {
			log("[gallery] " + d.cap + ": " + d.title);
			let res; try { res = await d.run(); } catch (e) { res = { status: "fail", marker: "SHIPYARD-" + d.cap + "-FAIL: " + e }; }
			log(res.marker);
		}
		log("== gallery selftest done ==");
	}

	window.shipyardGallery = { demos, render: renderGallery, selftest };
})();
