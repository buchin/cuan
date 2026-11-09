## Measured production inventory

Read-only size inspection on9November2026. Figures are rounded `du -h` values,
not exact reclaimed-space forecasts. The69GB filesystem had1.5GB available after
a native release failed during three concurrent Go builds. Cleaning the idle host
Go build cache with official `go clean -cache` recovered approximately1.3GB.
Subsequent active work consumed some of this headroom; latest observation2.5GB.

| Location | Observed size | Meaning |
| --- | --- | --- |
| `/opt/cuan/data/workspaces` |13GB| Current developer workspaces and4.6GB legacy verifier snapshots |
| `/opt/cuan/data/projects/task-workspaces` |6.5GB| Another retained workspace generation |
| `/opt/cuan/data/projects/verification-snapshots` |7.2GB|39 independent verification snapshots |
| `/home/ubuntu/.local/share/cuan/accounts` |7.1GB| Account runtime profiles, histories, dependencies and old release staging |
| `/opt/cuan/data/projects/release-workspaces` |2.1GB| Native publication checkouts |
| `/var/lib/postgresql` |231MB| Database; not the main measured storage consumer |
| `/var/log/nginx` |666MB| Logs; smaller than retained source/runtime copies |

Do not add legacy verifier4.6GB to workspace13GB again. The four main workspace,
snapshot and account locations above total approximately33.8GB before native
release checkouts. Old and new verifier locations together account for11.8GB.

## Why cache cleanup alone cannot solve this

A sampled current task checkout was199MB, including `.git`92MB and `docs`81MB.
A sampled verifier snapshot was181MB, including `.git`78MB and `docs`81MB.
These are source/object copies, not removable npm caches. Most of the sampled
snapshot's footprint remains after dependency cleanup.

`runTaskWorkspaceGC` in`internal/server/gc.go` targets
`ProjectsDir/task-workspaces` only. The runner separately sweeps `WorkspacesDir`
at startup and after executions (`internal/runner/runner.go`); neither of these
workspace sweep routes expires the two verifier roots. Existing pruning deliberately protects
active leases, dirty/ignored work and commits beyond remote-tracking refs; do
not bypass that protection with a directory-age deletion.

In one sampled1.8GB account profile, AGY brain history was430MB, conversations468MB,
provider logs69MB, Go modules352MB and old Cuan release staging448MB.
Across three account profiles, release staging alone was267+245+448MB, about960MB.
These values do not prove every staged payload is redundant or eligible to delete.
No profile credentials or history contents were read or removed.

## Recommended order

1. Unify storage inventory and lifecycle metadata across current and legacy roots.
   Classify active execution, unpublished/dirty work, accepted delivery, generated
   dependencies and independent evidence before allowing cleanup. Publish byte
   totals and fixed reasons for retained candidates, rather than silent skips.
2. Reuse the maintained manifest/checksum-aware release pruner for old account and
   native-runtime staging after matching installed payloads and active processes.
   Move future staging out of AI credential profiles into an owner-managed build
   location. Estimated visible candidate pool960MB; actual eligibility unproven.
3. Retain accepted-task commit identity, reports, artifacts and verifier receipts;
   expire reconstructible clean checkout material after a bounded warm period.
   Keep active, failed/unfinished, unpublished and unarchived work until their
   lineage and recovery needs are explicitly reconciled. Cover both verifier roots.
4. Reduce repeated Git/document payloads. Evaluate isolated worktrees backed by a
   project object store with pinned source/target refs and coordinated GC, or
   archive terminal snapshots. Existing self-contained snapshot lifetime guarantees
   must remain true. Do not simply introduce alternates tied to a disposable source.
   Move heavy generated evidence out of tracked source only with durable artifact
   references and equivalent task/agent access; ordinary project instructions stay.
5. Keep authentication per brain; centralize compiler/dependency caches per owner.
   Native release currently sets per-checkout GOPATH/GOCACHE, independently of the
   existing developer owner caches. Reuse their lease/maintenance lifecycle rather
   than downloading and caching a new toolchain for each publication checkout.
6. Archive/compress old provider histories under a documented retention policy,
   preserving resumable sessions and application evidence. Measure all profiles
   before proposing a reclaim estimate. Do not treat history as disposable cache.
7. Maintain a measured working-space reserve before build admission, recheck after
   dependency preparation, and pause/recover once rather than launching repeated
   failed builds. Alert on bytes and growth rate. Choose reserve from measured
   peak builds, not only disk percentage. Expand storage if legitimate retained
   data still exceeds the budget after lifecycle fixes.

## Immediate code correction

Commit325e4444 changes the release script to build Go components sequentially,
reusing cache entries and avoiding three simultaneous temporary dependency graphs.
Five executable release-gate regressions pass, including failure/no-publication
and ordered start/end assertions. It is pushed to dev; successful native use and
production activation remain pending. No task workspace/snapshot/history was
removed during this investigation.


## Follow-up lifecycle correction

Live read-only status on the current task workspace showed ignored `.agents/`,
`.gemini/` and `internal/runner/data/`. The generic retention check rejects every
ignored entry, including MCP defaults Cuan itself creates. The runner does sweep
current workspaces, so missing invocation alone is not their demonstrated cause.

The corrected workspace retention permits only exact default Cuan MCP config
contents at `.agents/mcp_config.json` and `.gemini/config/mcp_config.json`.
Extra files/directories, custom config and other ignored paths remain protected.
All original clean-source, published-commit and execution-lock conditions remain.
Actual generator-to-pruner regressions reclaim a published fixture with only
these defaults, while edited config, saved reports, extra directories and other
ignored data remain intact. Existing unpublished/dirty/stash retention tests pass.
No production workspace was deleted to establish these fixture results. Test
runtime data and verifier snapshot lifecycle still require separate handling.
