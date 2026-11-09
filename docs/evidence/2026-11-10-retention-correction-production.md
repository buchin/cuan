## Verified source and production recovery

Task: `msg_b82e3c7d-138b-4914-bb84-4084c8b8b9cc`.

Developer correction `fe8c0cc2fc62ab6b399b9d835c819b44e9a5bb97` was
independently examined by native verifier attempt
`att_a22a1d8d9e035e678a7789308e4e8067`. It passed the source criteria;
publication and post-deploy observation remained outstanding. The developer's
native checks passed in 296.389 seconds. The verifier execution took 8m40.717s;
its final native unchanged-repository check took 53ms.

The root independently reran the original hardlinked external-file
counterexample on the exact corrected source. Selecting the root maintenance
branch with real filesystem operations preserved mode `02755` and file contents.
This was an unprivileged simulation of that branch, not a privileged OS test.
All six source-compactor tests passed. The correction separates exclusive new
lease creation from validation of existing files before mutation.

The old developer admission lacked managed-release ownership because the
publication parser misread “tanpa membatalkan deploy”. Updating the parser did
not retroactively manufacture that missing record. To recover the already
independently verified work, root explicitly merged its exact source and used
the official `cuan release run` workflow. This is a manual recovery, not proof
of an untouched autonomous pipeline. No acceptance receipt or Done status was
written manually.

Production release `2a6594940033d770688efe0afe0b922414ce0550` contains that
source, the publication-scope corrections and native build-cache propagation.
The official release completed at `2026-11-09T21:48:10.564148385Z` with exit 0,
unchanged source and exact-SHA healthy `/healthz`. The runner was active and
undrained afterward. Python tests and targeted runner/publication regressions
also passed on the merged source before publication.

The installed maintenance bundle is
`d839a87e4ebfe02e83500bc188b6d718aded19573481d350fda552d070e53e42`.
Its compactor SHA256 matches the repository exactly:
`88b0a47366d5c5fb03827228bcbbd35a1da28729b8f74f9c34ad6e11589c1f88`.

## Actual maintenance observations

Earlier production runs at 05:19:52, 05:22:51 and 05:26:06 +08 reported nine
compacted source materializations and 974,209,024 reclaimed allocated bytes in
total, with Git objects retained. Those runs preceded this lease correction;
their bytes must not be attributed to the correction or counted again.

The existing registered maintenance job was explicitly triggered after the
corrected release at 05:48:18 +08 and completed successfully. That run reported:

- Source compaction: zero additional candidates/bytes in both roots.
- First root retained 23 warm, 17 unavailable/cold and 12 with unique files;
  one candidate was busy. Second root retained 29 warm and 8 unavailable/cold.
- One idle snapshot `web/node_modules` removed; source data was not touched.
- One eligible native backup removed: 10,272,768 allocated bytes; three retained.
- Measured free space afterward: 2,487,017,472 bytes. This is an instantaneous
  filesystem observation, not the sum of a single cleanup category.

The 48-hour source retention window, independent Git objects, receipts, active
leases and unique data were not waived. No scheduler was added. Task acceptance
and any current-outcome inspection remain native review decisions. A fresh
execution is still needed to measure production speed improvement from cache
propagation; these fixtures and manual recoveries cannot establish that claim.
