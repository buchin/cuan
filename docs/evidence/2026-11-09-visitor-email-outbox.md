## Root gap and scope

Production room60fdacfc-5584-4968-9000-61e6213d6192 had three public owner
replies but no customer email receipts; its delivered receipt was an operator
notification for an incoming visitor message. Resend was enabled/valid. The
active source had admin reminders and explicit test sends, no customer reply
dispatch. The root correction adds future public-reply delivery, not historical
resending or claims that the provider rejected messages it never received.

## Implementation and verification

Migration38 creates a durable outbox and captures future operator message inserts
in their original database transaction, scoped to a connected CRM contact.
The existing scheduler sends unread public text after five minutes. Shared
visibility classification keeps whispers, approvals and regeneration decisions
out. Each exact request is persisted before any network call; per-message locks,
provider idempotency and atomic receipt/completion protect restart/concurrency.
An uncertain request cannot retry through a different API key or past23 hours.
No HTML/UI styling changes or provider-specific SDK dependencies were introduced.

Targeted local tests pass for public/private content, existing visitor history
and approval privacy, Resend HTTP behavior and migration loading. PostgreSQL
coverage executed against a new isolated `cuan_email_test_*` database, using a
compiled fixture binary and a mock Resend client. The database and uploaded
binary were removed after each run. No customer data or live Resend calls were
used. The final native PostgreSQL run passed2026-11-09T11:16:58+08.

Actual database assertions cover: no historical backfill; five-minute delay;
public unread send and visitor receipt; whisper/already-read suppression;
restart deduplication; fixed payload despite sender-setting changes; concurrent
worker serialization; provider acceptance followed by receipt persistence failure
and identical retry; expiry; changed API connection; changed contact address.
These establish application behavior under fixtures, not real customer delivery.

Deployment and production scheduler observation remain pending at this entry.
The historical target room is deliberately not used to send a test or replay mail.
