## Observed acceptance gap

Production taskmsg_28f9a870-dbd8-4428-a4e6-5e4abbfc4301 reached Done with one
native attempt. AGY spawned its own independent-verifier subagent, but there was
no separately admitted native verifier. The file was correct and its standalone
workspace was only28KB. Correct delivery does not establish native independence.

## Root correction

Existing independent acceptance gates covered code and account operations but
not standalone operational artifacts. Commit9e90637e6c8ed220948ed9636f4984151a0d2d5f
adds that missing gate using the existing manager read-only continuation. It
passes owner-scoped actual attachment paths, sizes and SHA256 as untrusted data,
without describing them as accepted. No new queue, service or source checkout.

Acceptance requires a successful current report, a native launch bound to the
same task/instructions, the saved predecessor verification assessment, original
authorization/cancellation checks and runner unchanged-directory evidence.
Harness subagent claims alone cannot satisfy it. Missing/foreign files are not
silently regenerated or searched under another owner. Already accepted tasks
are preserved rather than reopened. Review still evaluates every original
criterion and actual file contents; lineage alone is not correctness proof.

Targeted regressions pass for developer-claim rejection, actual file metadata,
missing/foreign inputs, existing accepted history, nonartifact work, immutable
single continuation on repeated review, native runner claim/completion and
successful-verifier recognition without looping. Existing code, account,
manager recovery and next-phase file-input regressions also pass.

The official release completed2026-11-09T02:47:26Z and exact-SHA production
health passed with37 migrations. Production room694d605f-ebb4-45d5-860e-fc3fd7b8f644
request1f90e3fd-2fab-4b1c-b415-6bb230a21ceb is the fresh native workflow check.

## Native production result

Taskmsg_e90d5e5f-3850-47c1-8db3-db824902bf54 reached native Done at
2026-11-09T10:55:07.327433+08 with exactly two attempts and no manual retry,
status acceptance or worker-workspace changes:

| Stage | Native attempt | Start / finish (+08) |
| --- | --- | --- |
| Developer | att_7d681a1e666567341c07573ed2654629 | 10:49:17.808062 / 10:51:00.890215 |
| Verifier | att_be1439d6a6e703633ab13a7a58d88403 | 10:52:32.821506 / 10:54:04.656876 |

The first completed review did not accept the developer's result. It saved the
native independent-file verification continuation. The second attempt's scope
was read-only, deny push/deploy; its runner log explicitly identifies the ops
artifact workspace with no Git checkout. Native unchanged-directory verification
passed in72ms without registered application tests. The verifier report identifies
the actual developer attachment and retained artifact file, checks their matching
hashes and exact five lines/newline, and delivers the file and inspection report.

Root officially downloaded verifier attachmentacaacffd-88ce-4ca8-af05-d0a115976185:
170 bytes, SHA256 `b903747b2d3c581e6a87b03e0bc310dfcd337daf503375bfc15102ac3b8f3d50`.
It is byte-identical to the earlier independently downloaded exact fixture.
Report attachmente43aeebb-d121-41d4-a237-ca491cc190dd is1959 bytes and was also
downloaded/read. The PM then accepted the actual result without further attempts.

Without explicit maintenance requests, scheduler success followed developer
completion at10:51:12.193553784+08 and verifier completion at10:54:22.193830922+08.
This proves both completion events reach maintenance; it does not prove reclaimed
bytes or indefinite disk stability. This small file task still took several
minutes due to model execution and serialized review. Native independence now
holds in this fixture; broader performance, snapshot retention and other task
types remain incomplete.
