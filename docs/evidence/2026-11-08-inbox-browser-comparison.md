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
