# Public web tools in chat

Cuan uses lightweight HTTP retrieval before considering a browser:

- `web_fetch {url}` returns readable page content for documentation and articles.
- `http_request {url, method?}` returns HTTP status, selected response headers and
  raw text or JSON. Method defaults to `GET`; `HEAD` inspects headers only.

Both are exposed through `cuan mcp --public-web` and the per-turn Inbox CRM MCP
surface. They use public HTTP(S) destinations, validate DNS at connection time
and on redirects, and do not inherit ambient proxies, login cookies or credentials.
`http_request` accepts no shell commands, request body, custom headers or mutable
HTTP methods. Its body is capped at64KiB, non-text bodies are omitted, and
truncation is explicit. HTTP errors such as404 or429 retain their real status;
they are observations, not claims of successful work. These GET/HEAD capabilities
cover ordinary public curl-style inspection without a browser process.

Example:

```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"http_request","arguments":{"url":"https://example.com/","method":"HEAD"}}}
```

Authenticated service actions still use the user's connected integration and
recorded Cuan actions. Ordinary chat cannot run arbitrary curl/shell commands.
The tool-free task-review and recovery stages remain tool-free.

## Browser direction

Use Obscura as the candidate optional local engine, started only when HTTP is
insufficient. Keep the provider behind a Cuan adapter, isolate each owner's
session, bound lifetime/concurrency/output, clean temporary data, and measure
actual RAM/disk before enabling it by default. Browser automation is not enabled
by adding the HTTP tool or by mentioning a browser in the prompt.

TinyFish can be an optional remote provider for owners who prefer managed
infrastructure. Wigolo offers a larger search/research stack; it is not required
for ordinary HTTP retrieval. See the primary-source comparison evidence.

## Browser process adapter (integration in progress)

`internal/browserfetch` provides the per-inspection execution lifecycle. Trusted
application configuration supplies an absolute browser executable and shared lock
file; tool input cannot select a binary or add command flags. One cross-process
file lock limits active inspection. Each call uses a fresh temporary HOME,
config/cache/storage directories and an explicit minimal environment, with private
network opt-in disabled. Calls have a15s outer deadline, a10s engine timeout,
a64KiB text limit and bounded diagnostic capture. Cancellation kills the process
group and removes temporary files. A busy slot returns an explicit error rather
than launching another process.

This package is not yet exposed in the chat MCP catalogue. Before exposure,
complete the pinned-runtime installer, sandbox filesystem boundary, shared lock
mount across isolated provider processes, complete redirect/subresource network
checks, cancellation tests through MCP, and actual production use. A browser
executable alone is not evidence these integration requirements are satisfied.

Unit tests:

```sh
go test ./internal/browserfetch
```

Explicit native engine test (no downloads or ambient browser profile):

```sh
CUAN_TEST_OBSCURA=/absolute/path/to/official-engine go test ./internal/browserfetch
```

The native test first verifies default private-network rejection. Only its
synthetic local JavaScript fixture uses a test-only private-network opt-in;
production calls never supply that option. Full rendering, screenshots and login
sessions are outside this anonymous text-inspection adapter.

The process adapter now requires `Runtime.Sandbox` (Bubblewrap). It mounts only
OS libraries/executables, certificate/DNS files, the selected engine and the
call's temporary HOME. Host profiles, application data and lock file are hidden;
`/proc` is empty. Missing sandbox support is an error, never an unsandboxed
fallback. `Loader`/`LibraryDir` can select an explicit Cuan-managed compatibility
bundle on musl hosts; these are application configuration, not tool inputs.

The native test on a musl host also accepts explicit
`CUAN_TEST_OBSCURA_LOADER` and `CUAN_TEST_OBSCURA_LIBRARIES`. It exercises real
Bubblewrap, script-originated private HTTP/file rejection and JavaScript output.
These tests do not establish redirect coverage or a ready production integration.
