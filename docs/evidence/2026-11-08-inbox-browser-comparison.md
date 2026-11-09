# Production Inbox evaluation: Obscura versus Wigolo

Release: `ab0fd50f6106a1f2d3361e2fd3290ffd0b947743`.
Room: https://cuan.dojo.cc/inbox/bb9ffd6d-171d-49ee-8bc1-31af534c1868
Request: `dfdb27e4-33f2-408e-a86f-4d567fb4c8ad`.

The member asked which product suits Cuan for dynamic website research, considering
recurring server disk pressure and the planned OSS release. No tool name,
implementation request or task approval was supplied.

Observed through official `cuan chat messages` and `cuan chat trace`:

- Completed normally, without response repair, permission denial or task creation.
- Two MCP `web_fetch` calls read the supplied Obscura GitHub and Wigolo URLs.
- Two provider-native searches supplemented those sources.
- Recommended Obscura for its smaller deployment footprint and permissive license,
  while acknowledging Cuan must retain search/extraction orchestration.
- Total elapsed time was 143 seconds; the provider process took 132 seconds.
  Recorded provider stages total approximately 120.8 seconds. Tool work was a small
  fraction of latency; this is not a network-fetch performance benchmark.

Independent primary-source checks:

- Obscura repository reports Apache-2.0, Rust, embedded V8, native rendering without
  Chromium, approximately 70 MiB binary and 30 MB memory. Sizes and memory are
  upstream claims, not measurements on Cuan production.
- Wigolo package metadata reports AGPL-3.0-only. Its README requires Node >=20
  and approximately 1.5 GB free disk for setup, including browser/model downloads.
  It offers broader search/fetch/crawl/extract/cache functionality and progressive
  browser fallback; the two products have different scope.

Assessment: tool use, task routing and recommendation relevance passed this case.
Response quality is improved but not ideal: 143 seconds is too slow for this
bounded comparison, direct source citations were absent, and statements that
Wigolo would worsen disk pressure should be framed as added resource requirements
rather than a measured outcome. Do not treat upstream performance figures as
observations, or this research test as proof of the full PM/dev/verifier workflow.

Sources:
- https://github.com/h4ckf0r0day/obscura
- https://github.com/KnockOutEZ/wigolo
- https://wigolo.app/

## Primary-source-first follow-up

Release `a58035b29f632438028f857c71481e718011aab8` shortened and clarified
research guidance: fetch supplied primary URLs first, batch independent sources,
search only material gaps, cite sources and separate vendor claims from measured
outcomes. The same question was sent to a fresh room without tool directions:
https://cuan.dojo.cc/inbox/9d75cbc6-9099-4b5c-a814-b4d7f45969ff
Request `3a1c3092-329a-49b8-8859-98ec6b5e841f`.

Observed completion was 99 seconds versus the earlier 143 seconds, with the same
`gemini-3.8-flash-high` route. Both supplied URLs were fetched in one provider
turn, followed by a README fetch and two searches. The result retained the Obscura
recommendation, included direct source links and created no tasks. There was no
response recovery. This is one paired observation, not a statistical latency
claim or proof that fewer tool calls caused the improvement.

Remaining quality gaps: the answer was longer than requested, used unqualified
upstream memory/binary-size figures, overstated certainty about disk impact and
licensing safety, and appended an unrequested integration-task offer. Do not
claim these quality requirements solved from the instruction change alone.

## 2026-11-09 current-source check and production chat

Primary sources were fetched again through official `cuan mcp --public-web`:
Obscura and Wigolo upstream READMEs, and https://www.tinyfish.ai/.
Obscura still claims ~70MiB binary/30MB RAM (not a Cuan measurement). Its README
now explicitly documents private-network blocking by default and an MCP server.
Wigolo still documents Node20+ and about1.5GB setup disk, with local browser/model
downloads. TinyFish describes hosted serverless browser infrastructure; current
homepage lists TinyBrowser at$0.002/minute and TinyAgent at$0.016/step. Prices and
product claims are observations of the vendor page, not billing guarantees.

Recommended direction: existing HTTP retrieval first, optional on-demand Obscura
adapter for dynamic pages, and optional remote provider. No browser engine was
installed or enabled as a side effect of this comparison.

Production release `4cc9243a3dd5cefc203536d743c335bf2a3209fe` passed exact-SHA
health checks. Research room:
https://cuan.dojo.cc/inbox/b70ed3d8-ac6c-4d1f-8f62-fa1e6ec8c53a
Request `c76ece0d-2e63-475c-91cb-f7712d198d9d` completed using supplied public
sources; no task was created. It confirms the new operator recovery policy is
present, but this request did not time out and does not prove automatic recovery
in production. The answer remains too long and presents an older disk statistic
as current without its observation time. Treat current-state grounding and
brevity as unresolved quality gaps. CLI observation was interrupted after30s;
the accepted request was inspected, not sent again.

## Independent Obscura engine probe (sandbox, not production)

Downloaded official release v0.2.6 `obscura-x86_64-linux-no-render.tar.gz`.
GitHub asset digest and downloaded SHA256 both:
`f1188c1a3b1ef70d56d594beebbb34b693ff89c280fa073820ad019ed555a02c`.
The43,559,951-byte archive contains a~32MiB main binary and~30MiB worker.
Artifacts: `/home/dev/artifacts/cuan-obscura-evaluation/v0.2.6/`.

The native release failed directly on this musl sandbox (`mallopt`/`__res_init`
missing). It ran with the independently retained Cuan UI glibc loader/libraries;
the ambient `/usr/local/bin/obscura` wrapper was not used because it points into
Fural-managed assets and is not a Cuan product dependency.

With an empty environment and isolated HOME:
- One public static fetch completed in0.13s with17,408KiB maximum RSS.
- One local synthetic page changed its DOM using setTimeout; the returned text
  was `CUAN_JS_RENDERED`. With an explicit1s settle wait it completed in1.03s,
  also17,408KiB maximum RSS.
- Default navigation to the local fixture was rejected before reaching the
  HTTP server. Only the synthetic JS test explicitly enabled private networking.

These are small sandbox observations, not production or complex-site benchmarks.
No screenshots/full rendering, interactive login, concurrent loads, subresource
SSRF behavior or long-lived memory retention were verified. The engine is not yet
integrated into chat. A pinned managed runtime, session/lifetime/concurrency
controls, complete destination tests and actual production use remain necessary.

## Browser process lifecycle implementation

`internal/browserfetch` now implements the real per-call process adapter, not yet
MCP exposure. Tests prove minimal environment/no ambient Owner token or proxy,
unique HOME/cache removal, shared-file slot exclusion/release,64KiB output cap,
URL scheme/embedded-credential rejection, lock-symlink rejection and deadline
cancellation of a child process with cleanup. A cap test caught and fixed an
`io.Copy` optimization bypass caused by embedding bytes.Buffer; the bounded
writer now contains its buffer privately and does not expose ReadFrom.

Explicit native v0.2.6 test passed with the independently retained Cuan glibc
loader wrapper: default local navigation was rejected before the fixture server
was reached, then a test-only local opt-in observed `CUAN_JS_RENDERED` after script
execution. This does not prove cross-subresource policy or filesystem isolation.
The package is deliberately not advertised to the chat until the installer,
sandbox/lock mounting and full network/lifecycle integration are verified.

## Mandatory browser filesystem boundary

`Runtime.Fetch` now requires a filesystem sandbox and never falls back to
unsandboxed execution. Bubblewrap exposes only OS executables/libraries,
certificate/DNS files, the selected engine and a writable disposable HOME.
Provider profiles, host HOME, application data and the shared lease file stay
outside the browser namespace. OS directories and engine are read-only; `/proc`
is empty. Private PID/IPC/UTS namespaces and a new session remain enabled.
An explicitly application-managed loader/library bundle supports the musl test
host without any Fural-managed files or ambient LD_LIBRARY_PATH.

Real bubblewrap tests passed: host sentinel unreadable/unchanged, Owner token
absent, private-network opt-in forced off, `/proc/self/environ` absent and writes
to `/usr/bin` denied. Engine absence/sandbox absence do not trigger a fallback.

Official v0.2.6 additionally passed inside that boundary: direct local navigation
rejected, a controlled local HTML script could not fetch a local HTTP endpoint or
read a host-private file, and a separate test-only local-network opt-in rendered
the synthetic JavaScript output. The script probes use a local HTML fixture
without lifting network restrictions; public tool inputs cannot request local
files, arbitrary evaluation scripts or private-network opt-ins.

The engine source at v0.2.6 explicitly opts the CLI fetch context into local file
access (`set_allow_file_access(true)`). This confirms why environment isolation
alone was insufficient and a separate filesystem boundary is mandatory. A blank
page was initially unsuitable for the DOM-dump probe; tests now use a complete
controlled HTML document and require explicit ACCESS_BLOCKED markers.

Remaining: redirect/network-path coverage, pinned installer, mounting the shared
lease and runtime through the outer provider sandbox, MCP registration, and
production observations. The feature is not yet exposed or deployed.
