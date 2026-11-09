# Web tools in chat

Cuan uses HTTP retrieval first and an optional anonymous browser when a page
needs JavaScript. Ordinary users do not manage browser commands or AI accounts.

- `http_request {url, method?}` returns real HTTP status, selected headers and
  up to64KiB of raw text/JSON. Method defaults toGET; HEAD returns headers only.
- `web_fetch {url}` extracts readable documentation/article content.
- `browser_fetch {url}` reads script-rendered text using an installed, verified
  Obscura runtime. It is advertised only when the selected runtime is valid.

These capabilities use the public-web and per-turn CRM MCP surfaces. HTTP tools
validate public DNS destinations at connection time and on redirects. They do
not inherit proxies, cookies or credentials. Raw HTTP accepts no request body,
custom headers, shell commands or mutable methods. Non-2xx statuses are retained;
binary bodies are omitted and truncation is explicit.

The browser accepts only an HTTP(S) URL, with no embedded credentials, arbitrary
scripts, executable selection or flags. Its result records the requested URL
and text, not a verified final redirect URL or inferred HTTP status. Authenticated
service actions use the user's connected integration and recorded Cuan actions.
Task-review/recovery stages remain tool-free.

## Install the optional chat browser

The native Linux service administrator runs:

```sh
python3 scripts/setup-chat-browser-runtime.py --user SERVICE_USER
python3 scripts/setup-chat-browser-runtime.py --user SERVICE_USER --check
```

Replace SERVICE_USER with the existing non-root Cuan service account. A packaged
release carries the same installer at `runtime-tools/setup-chat-browser-runtime.py`.
Ordinary deployments carry that installer without automatically downloading or
installing the optional engine. New native runtime setup also supports
`setup-agent-runtime.py --user SERVICE_USER --chat-browser`.

The installer pins Obscura0.2.6 and official SHA256 archive digests for Linux
x86_64/aarch64. It installs only the main no-render executable, creates a stable
shared lease file and a private manifest, and removes temporary archives. A
valid existing installation is reused; changed/unrelated binaries are preserved
and rejected. `--archive PATH` imports an existing official archive through the
same checksum gate. Setup requires256MiB free headroom.

Official Linux binaries require glibc. On musl, `--loader PATH --library-dir DIR`
can explicitly select a separately managed Cuan compatibility runtime. Neither
setup nor checking discovers a Fural/provider loader. Subsequent checks reuse the
saved compatibility selection. Version checks prove executable startup only;
they do not establish complete browser/chat readiness.

The installer writes `~/.local/share/cuan-chat-browser/runtime.json`. The server
selects its service user's manifest, or an explicit `CUAN_CHAT_BROWSER_MANIFEST`.
It verifies the actual binary hash and exposes only a rebased private manifest,
engine, sandbox executable and the shared lease file inside provider isolation.
The full runtime/service HOME is not mounted. Provider environment variables
cannot replace the application's manifest selection. Missing/invalid optional
runtime leaves ordinary HTTP/chat available.

## Execution lifecycle

`internal/browserfetch` runs one inspection at a time through a cross-process
file lock. Every call receives a new temporary HOME/config/cache/storage tree,
a15s outer deadline, a10s engine timeout, a64KiB text limit and bounded stderr.
Cuan fixes the V8 old-space limit at128MiB and semi-space at4MiB; this is a JS heap
setting, not a hard limit on total process RSS. Script execution also receives
the engine's3s script-budget setting. Agent input cannot raise those values.
Cancellation kills the process group and normal finalization removes temporary
files. The outer provider namespace uses a private temporary filesystem.

Bubblewrap is mandatory. It exposes only OS executables/libraries,
certificate/DNS files, the engine and the disposable HOME. Host profiles,
application data and lease file are hidden; OS paths are read-only and `/proc`
is empty. The browser namespace inherits no account cookies, Owner credentials,
proxy or private-network opt-in. Missing sandbox support has no unrestricted
fallback. This text browser does not replace screenshot verification tooling.

## Verification and current rollout limits

Installer tests require no downloads or host changes:

```sh
python3 scripts/test-chat-browser-runtime.py
go test ./internal/browserfetch
```

Explicit native tests use `CUAN_TEST_OBSCURA` and, on musl,
`CUAN_TEST_OBSCURA_LOADER`/`CUAN_TEST_OBSCURA_LIBRARIES`.
`CUAN_TEST_BROWSER_MANIFEST` checks an actual installed manifest.
Real namespace tests cover hidden host files, read-only OS files, script-originated
private HTTP/file rejection and JavaScript output. Only a synthetic local test
opts into private networking; production calls never supply that option.

Local native CLI/MCP use is verified. A disconnected network-namespace fixture verifies a real public-address302
redirect to a private service, both through navigation and script fetch. The
private service receives no requests and a public control page succeeds. Actual
production enablement and provider-to-browser observations remain unverified.
The current provider filesystem wiring is the existing Antigravity chat path;
other harnesses must expose the MCP capability before advertising it.

TinyFish remains an optional remote-provider direction. Wigolo's wider research
stack is not a dependency of these tools. See the primary-source comparison
[evidence](../evidence/2026-11-08-inbox-browser-comparison.md).
