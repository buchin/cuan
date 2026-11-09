## Findings

The unread-chat scan runs every minute. Duplicate suppression used only a
process-local map; persisted successful sends were written but never consulted.
A new process could therefore resend unchanged historical customer messages.
The sender hardcoded `Toko Cuan`, and both notification and integration-test
email designs retained the old store template.

Conversation60fdacfc-5584-4968-9000-61e6213d6192 contains two public owner replies
with no email delivery record. The visitor message has an operator delivered
receipt. Production source contains only two Resend send call sites: admin idle
notification and explicit integration test. There is no visitor email dispatch
path. This establishes the missing application capability; it does not assert a
provider rejection. No customer message or configuration was changed.

## Local correction and checks

Read successful-send history before sending; stop on database read failures.
Preserve existing cooldown and attach a stable provider idempotency header.
Replace shop branding and noisy template surfaces with a flat Cuan layout.

`go test ./internal/resend ./internal/server -run 'TestClient_|TestBuildChatIdleTranscript|TestChatIdle|TestResend' -count=1` passes.
The actual idle flow test clears process history and supplies an older persisted
successful send, proving unchanged messages remain suppressed after restart even
outside the cooldown. Missing history table prevents sending. HTTP-client test
checks the provider idempotency header. Fixtures never call live Resend.

Actual generated HTML was rendered and visually inspected at390px and1440px:
`/home/dev/artifacts/cuan-email-calm/390.png` and `1440.png`. Neutral flat canvas,
readable paragraphs, single44px action, no horizontal overflow, nested cards,
emoji banner or status pill. Browser inspection is not cross-client email QA.

Native task benchmark continues independently; this correction has not been
used to manually accept or publish its task implementation.

## Retained implementation history

The initial idle reminder feature exists in7bc3265f/e84cad43 (31October).
A later deduplication correction,a97707a8 (7November), exists only on retained
`origin/task/msg_56556e6a-5dc8-44a6-ae56-978f925e245d`; it is not an ancestor of
the active root dev source. It added persisted receipt/log lookups, but was never
integrated into the active implementation. This is a publication gap, not proof
that the original idle feature was removed. Retain that branch/history.

Current correction reuses existing exact operator message-email mappings plus
legacy send timestamps. Exact mappings prevent confusing a newly arrived message
with an older email still in flight. Database errors stop sending. Regression
also proves exact mapping suppresses replay after the legacy log is absent.
