# Publication after retained-source inspection

Task msg_d0843350-b9f0-4682-b326-c0adaa2498e8 finished source-bound inspection
att_e2e46d662a2c21eb286e8150fc1408e6 and retained source 68d623b4. PM requested
native publication, but the guard rewrote it to another read-only inspection.
That admission stopped as source_is_already_an_inspection. The result stayed
blocked rather than returning to the authorized developer workspace.

The source had also not been saved to a remote ref after rebasing. Publication
ancestry required both source and target objects locally; fetching the unpublished
source failed even though a complete target history can prove non-inclusion.
Git ancestry now streams the complete target graph under the existing deadline.
A missing source object is not itself proof: shallow repositories, missing parent
commits, failed traversal and cancellation remain unknown. A complete target
history can establish that the recorded SHA is absent without modifying refs,
working tree or repository history. Source validity and feature correctness are
separate gates. A 2,000-commit fixture verifies traversal beyond the ordinary
64 KiB snapshot-output budget.

A saved PM publication continuation after a successful source-bound inspection
can now classify the remaining work as separate developer reconciliation when
native evidence proves the retained commit is absent from the registered release.
Original owner scope, report/check/unchanged-repository receipts and same-repo
identity still bind it. Historical inspection can establish its own evidence
without pretending its target is today's release. Reconcile equivalent changes
and existing effects before publication; commit non-inclusion does not establish
feature absence. No inspection snapshot becomes writable and no task is accepted.
Publication review policy v4 refreshes affected unfinished review snapshots.

The native review-to-launch regression failed before correction and now queues
a separate ordinary execution with the same original authorization. Negative
cases retain unknown ancestry, an already-contained result, missing report or
receipt, changed source, owner deployment prohibition, wait and owner-input gates.
Existing publication, scope and source-correction contracts pass (4.332s);
project-owned snapshot/connection checks pass (1.247s). Git ancestry regressions
pass (0.652s). Production rollout and recovery remain to observe.
