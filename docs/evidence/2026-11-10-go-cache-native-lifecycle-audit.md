## Completed workflow and remaining integration gap

Task `msg_aaa7e319-ebee-4def-8328-92c8f672de72` in room
`a27bb46b-7ed6-473b-bcef-b93e6e7ed4e6` reached native Done at
`2026-11-10T07:09:10.936694+08:00`, with eight of eight criteria marked complete.
Source `8d7a6f7f7c74a92c7da9349b7dc034d98aca6c5e` was independently verified
and published as healthy release `06d47a2cacff325b770428bf33809bce1c676f72`.
History and acceptance were not edited manually.

Observed stages:

- Developer `att_dc01b01ab3eef7c7291047282b162153`: 17m15.197s;
  final native checks 28.781s.
- Source verifier `att_8e0716dfd97c87290663e7139a8ad459`: 6m1.546s.
- Native publisher `att_6ec95297e9681b329235e9366b274038`: 06:43:44–06:50:27 +08.
- Post-release inspection `att_3fd0dda467659e12a493678f47d4181b`:
  7:01:14–7:07:44 +08; native checks 151.984s.
- Overall owner request 06:16:44 to Done 07:09:10: about 52m27s.

The post-release inspection was queued for storage admission after publication.
Registered maintenance succeeded but available space remained about 1.35 GB,
below the unchanged 2 GiB admission floor. The filesystem was ext4 with 760,100
reserved 4,096-byte blocks (3,113,369,600 bytes reserved for root).
Root made one reversible infrastructure adjustment: reserve 262,144 blocks,
retaining 1 GiB for root. This exposed 2,039,627,776 additional bytes to service
users without deleting data. Available space rose to 3,464,863,744 bytes and the
same queued inspection resumed automatically, without a retry or status change.
This intervention means the run is not an untouched end-to-end benchmark.

## Production timestamp observation contradicts complete integration proof

The managed Go lease at
`/opt/cuan/data/go-build-runtime/a72b4dd88cd7f669996ae58dab83ea447e383c4ad3576b408ddd6c110e4ca5c7/.build-cache.lock`
remained **zero bytes**, regular, UID 1000, link count one, with modification
time `2026-11-06 21:02:20.199928934 +0800`, after the new post-release worker
finished. Project GOCACHE/GOMODCACHE/GOPATH overrides were unset.

Maintenance reported the cache busy at 07:06:00. The inspection completed at
07:07:44.846 +08. Runner journal at 07:07:47 recorded a cancellation directive
for that terminal attempt and detached PID 4099737 exiting. At 07:08:07 the Go
pruner reported one eligible/removed idle cache; its directory later measured
228 bytes. Free space immediately *before* that cleanup was not captured, so
this deletion alone does not prove a moderate-pressure policy violation.

The missing completion timestamp is a separate, concrete integration gap.
Current code registers its writer as a defer, publishes completion, then runs
workspace GC before returning. A supervisor termination after terminal completion
can therefore prevent deferred persistence. This is the leading lifecycle
explanation, not yet a reproduced end-to-end causal verdict. Tests that directly
call `releaseBuildCacheLease` do not establish persistence at the externally
visible completion boundary.

Next correction must reproduce the production caller ordering, inspect actual
metadata in a completion-handler regression, cover developer/verifier/publisher,
and retain native scope/ownership and cache-pressure invariants. No acceptance
record is being fabricated or existing failed history removed. Publication of a
healthy binary and passing helper tests alone are not completion evidence for
the missing runtime effect.

## Completion boundary resolution and verified caller lifecycle

Regression reproduction confirmed that both `runAttempt` and `runNativeReleaseAttempt`
deferred cache lease release until after `deliverCompletion` published terminal
status to the server. Consequently, when the completion request arrived at the server,
`.build-cache.lock` remained zero bytes and still held by `flock`. Immediate process
termination or cancellation upon reaching terminal state permanently discarded the
deferred lease persistence.

The correction reorders the lifecycle:
1. Active build cache leases (`releaseGoBuildCache`, `releaseNPMBuildCache`, `closeCaches`)
   are explicitly released, flushed, and synced to disk before calling `deliverCompletion`.
2. Deferred cleanup functions remain registered with `sync.Once` as an idempotent safety net
   for early errors, cancellations, or panics.
3. `releaseBuildCacheLeaseTime` verifies file properties and returns write/sync errors
   durably rather than silently ignoring partial truncate states, while preserving foreign
   and hardlinked files unmutated.
4. If lease release encounters an error, accurate native log observability is recorded
   without failing already successful execution results or altering execution deadlines.
5. End-to-end caller regression tests (`internal/runner/go_build_cache_lifecycle_regression_test.go`)
   verify that both developer (`runAttempt`) and publisher (`runNativeReleaseAttempt`)
   guarantee `.build-cache.lock` contains valid JSON metadata with `last_used_at` and
   has released its lock before `/complete` is handled by the server.

