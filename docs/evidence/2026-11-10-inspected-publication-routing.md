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

## Follow-up: continuation scope and optional manager metadata

Production review still stalled because an explicit publication revision omitted
optional manager action/reason. Normalize that explicit revision only under the
same native source/receipt/non-inclusion and original-authority gates. Explicit
wait, owner input and unknown evidence remain excluded. Repeated normalization
preserves the saved revision exactly.

The generated continuation also described the predecessor as a read-only
inspection, accidentally restricting its separate developer when reviewer prose
mentioned code. New text omits that historical adjective; policy parsing repairs
only the exact old native completed-inspection header. Explicit owner/attempt
read-only and deployment restrictions remain effective.

Publication policy v5 participates in the review evidence digest so affected
unfinished reviews receive one fresh assessment through existing deduplication.
Targeted execution policy tests pass (0.018s); server publication, persisted
handoff, idempotence and native-check regressions pass (4.427s). Production
recovery and task acceptance remain unproven at this checkpoint.

## Production contract loss observed after v5

Request 1bfbbfaf-7cc8-4ae7-ac53-d38322797d7d completed at 02:34:26+08.
The chat-worker journal records response_bytes=2475, structured_valid=false,
parsed=true and empty remaining_work/manager_action/source_review. The provider
boundary nevertheless emitted 'Provider reply validated'. The legacy text
fallback retained only revise/explanation/revision, losing routing metadata;
publication review then scheduled the already-completed inspection again.
The precise original JSON violation is not retained, so it is not attributed to
a particular model field. No new developer was admitted by that review.

Task reviews now use the same strict parser at the provider boundary as at
application. AGY can repair one rejected candidate through the existing bounded
45-second format stage (40-second native timeout), with no original context,
attachments or inspection tools. It uses the review schema rather than the chat
action schema. A second invalid response or attempted tool remains a failure,
never accepted/fallback work. Native scope and acceptance gates are unchanged.

A regression reproduced the old false-success behavior and passes after this
change, including successful classification preservation, repeated-invalid and
unexpected-tool cases. Existing formatter deadline/no-replay, publication launch,
source verdict and manager authority regressions pass (3.269s).

Official task execution diagnostics now expose inspection_handoff with bounded
native classification, approved scope, source-correction eligibility and current
publication ancestry observations. Historical attempt selection omits current
recovery diagnostics. It contains no reviewer prose or credentials and grants no
authority. Publication policy v6 requests a fresh assessment for affected
unfinished reviews through the existing digest/deduplication path. Production
recovery is still pending; neither this test nor deployment means task Done.

Before rollout, native storage maintenance reported success at02:38:25+08 but
available disk remained below2GiB (2,135,969,792 bytes). Six old /tmp/go-build
compilation directories had no open files (lsof) and no Go/compiler/test process.
Only those regenerable intermediate directories were removed; source, reports,
snapshots, logs and receipts were retained. Available disk then measured
2,654,560,256 bytes. This was explicit OS recovery, not proof the native workflow
already reclaims abandoned Go temporary builds automatically.
