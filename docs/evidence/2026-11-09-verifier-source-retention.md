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

## Accepted-source maintenance integration

The next change records exact native acceptance before the existing installed
worker runs. Current successful typed source/live verifier snapshots of Done
tasks receive a durable acceptance binding. Pending review receives no grant;
existing native release acceptance fixtures verify that transition and confirm
task status, attempts and original source remain unchanged.

The same pinned oneshot worker now scans both roots for accepted, clean, idle
materializations older than48 hours. It keeps complete Git objects and receipts,
checks real process readers plus the existing lease, and preserves unknown files.
The native generated config whitelist is validated once by Go and rechecked by
hash and exact directory contents before the Python worker changes files. The
cold marker and metadata directory are synced before sparse checkout.

A real cross-language fixture calls native Go acceptance annotation, the maintained
Python compactor, then Go lease restoration after deleting the disposable source.
It verifies positive materialized-block reclamation, exact source bytes, generated
defaults, unchanged object counts and the same lease inode. Interrupted restoration
with one exact existing source file recovers; unique replacements remain protected.
Separate real-Git/OS tests cover preview/no mutation, warm and unaccepted retention,
dirty/unknown evidence, active flock and an actual child-process reader. Existing
native source/live acceptance and scheduler/event regressions pass.

Automatic compactor deployment and production eligibility/reclamation observation
are still pending at this entry. Fixtures are not attributed as production savings.

## Deployment and native-host observation

The first rolloutf6f9ded8 stopped before activation: its remote payload list did
not include the new compactor required by the installer. The maintained release
script now derives local/remote payloads from the installer's read-only manifest
instead of duplicating tool names. Release-fixture SCP checks require every
transferred file to exist and assert coverage of the actual installer manifest.
The compactor tests are included in the release preflight. All five executable
release-build gate tests and four installer tests pass.

Release730958c534bb1f34b835823c34faa202b97fb89a then passed exact-SHA health
with38 migrations at2026-11-09T04:28:15Z. Both compactor commands are installed
in the existing oneshot unit, using pinned tool directory
a300b7072f5fe4c99dbea20dca2992ab88e38b58d078ddbe648871c170f855e3.

Root explicitly requested `cuan maintenance run` for diagnosis; do not attribute
this invocation as a new autonomous task-completion event. Actual scheduler
status recorded success at12:28:59.359699483+08. Native acceptance metadata was
created for eight source-verifier snapshots. The worker retained those eight as
warm,17 other legacy snapshots as unaccepted/missing and40 project snapshots as
unaccepted/missing. No production snapshot compacted and reclaimed materialized
bytes remained zero. Server logs identify nine accepted task snapshots protected
by unique/unknown files, including the native publication benchmark. Investigate
those files' provenance before removing them; do not broaden the whitelist.

A compiled isolated Git fixture executed on the VPS as root against the exact
installed pinned compactor, and its cross-language compact/restore round trip
passed in0.21s. It used its own temporary trees and an explicit empty fixture
process directory; the separate OS tests cover actual process readers, while
the production worker above used real `/proc`. The uploaded binary was removed.
No production task workspace or failed history was edited for this fixture.

The host had3.9GB available after deployment. This is an observation, not bytes
attributed to source compaction. The lifecycle is installed and verified through
fixtures/production retention scans; substantial source reclamation remains
unproven until real snapshots qualify. Retained Git object growth and unknown
generated runtime data still require further work.
