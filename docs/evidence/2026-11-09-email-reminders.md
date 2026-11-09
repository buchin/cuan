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
