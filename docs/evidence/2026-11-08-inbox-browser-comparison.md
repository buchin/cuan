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
