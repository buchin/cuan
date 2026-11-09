## Retained source lifecycle prerequisite

Cache-only maintenance cannot reclaim the tracked file copies in independent
verifier snapshots. Removing their source objects or relying on disposable Git
alternates would break exact-source inspection. The selected mechanism retains
the complete independent `.git` and persistent lease inode while a future cold
transition may remove only a clean, accepted snapshot's materialized tree.
This entry implements restoration first, before enabling any compactor. That
ordering is required so the preceding rollback release can also restore cold
snapshots; an old binary must not mistake a sparse tree for full source.

## Implemented restoration and checks

`AcquireVerificationLease` restores a recognized cold snapshot while holding
its existing exclusive lease. The marker must match the native source/target
binding, actual HEAD/ancestry and clean state; alternates are rejected. Git's
native sparse-checkout disable reconstructs files from retained local objects.
There is no network fetch, source workspace repair, reset or Git object deletion.
Unknown files, edited ignored configuration, linked/mismatched markers and files
placed at skipped tracked paths are preserved by refusing restoration. Native
commit verification rejects cold snapshots before inspection. Existing snapshot
preparation also acquires the lease so the API's normal path restores first.

Real-Git regression tests compact a fixture with `sparse-checkout set --no-cone
'!*'`, delete its disposable source and restore the exact result under lease.
They verify file bytes, unchanged object counts and lease inode, marker cleanup,
preserved unique files, invalid bindings, native generated MCP defaults and an
interrupted transition marker on an already materialized tree. Targeted existing
snapshot, lease, runner private-verifier and live-inspection/project tests pass.

No production snapshot is cold yet. Accepted-task admission, minimum retention,
maintenance process-reader checks and automatic compaction still need wiring and
production evidence. This prerequisite alone reclaims zero production bytes and
does not solve retained failed/unknown workspace growth.

Restoration support is deployed in productionb0d4f0d94ce361a493cb2924b7e86163f2cf91c2,
which passed exact-SHA health with38 migrations at2026-11-09T03:43:41Z. This
establishes the required compatibility release before a later compactor rollout;
automatic source compaction is still not enabled and no production byte saving
is attributed to this prerequisite.
