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
