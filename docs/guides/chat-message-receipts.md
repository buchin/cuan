# Chat delivery and read status

Public customer messages show a small receipt next to their time. Internal
whispers retain only the saved indicator; customer delivery/read checks never
apply to private messages or task previews.

| Indicator | Required evidence |
| --- | --- |
| Single check | Cuan persisted the message. |
| Double check | The recipient browser explicitly acknowledged the message UUID. |
| Muted blue double check | The message intersected the visible chat viewport while the document was visible. It means displayed, not understood. |
| Email sent | Resend accepted an actual email and returned its provider ID. |
| Email delivered | A signed Resend webhook confirmed acceptance by the recipient mail server. It does not prove reading or inbox placement. |

Chat polling and Inbox unread preferences do not confirm reading. Receipts are
idempotent, scoped to the conversation and opposite sender, and survive restart.
Visitor acknowledgements require both session UUID and matching guest UUID.
Read implies delivery. Requests accept at most 500 UUIDs per category. Clients
retry failed acknowledgements without inventing local success. Incremental
visitor polling also returns recent outgoing receipt updates.

## Email channel

The current idle-chat email is a notification **to the team/owner**. Its receipt
is labelled `Team · Email delivered`, independently of customer chat status.
It never claims that a customer received an email. Customer email receipts require
an actual customer-addressed send and corresponding message/provider mapping;
this change does not introduce a new email sending workflow.

Configure the existing Resend integration under Settings → Integrations → Resend.
The optional delivery-tracking section saves the webhook signing secret encrypted
alongside that installation's existing connection. It is never returned by the
settings API; `has_webhook_secret` reports only whether it is saved.

In the connected Resend account, register the installation's
`https://<cuan-host>/api/webhooks/resend` endpoint for `email.sent`,
`email.delivered`, `email.delivery_delayed`, `email.failed`, `email.bounced`,
`email.complained` and `email.suppressed`. Save the signing secret belonging to
**that endpoint**. Do not copy a different application's webhook secret or enable
open/click tracking. The endpoint verifies signatures and replay timestamps with
the maintained official Svix library. Without a configured secret, it rejects
events and never fabricates delivered status.

For agents, the official CLI supports:

```sh
cuan integration resend show
cuan integration resend set --webhook-secret-from-env CUAN_RESEND_SIGNING_SECRET
```

The named variable must contain this installation's explicit connection secret;
ambient developer credentials are not a Cuan user connection. Configuration and
tests do not send customer emails. `email.opened` is ignored. Email and chat read
timestamps are independent. Duplicate and out-of-order events cannot regress a
confirmed delivered/failed state to sent. Webhooks arriving before sender mapping
remain available for reconciliation. Unmapped historical emails stay untracked.

## Deployment and verification

Migration 000035 adds nullable receipt records and message/email mapping tables;
it does not rewrite old messages or infer old read states. Deploy using the native
atomic release/migration gate. Roll back application code while leaving these
additive tables in place; do not run destructive down migrations in production.

Targeted backend coverage: `TestChatReceiptsScopeReadAndPolling` and
`TestChatEmailAndReceiptsPostgres` (disposable PostgreSQL via
`CUAN_LIFECYCLE_SQL_DSN`). Browser fixtures:
`node docs/evidence/chat-receipts-check.cjs` and
`node docs/evidence/chat-widget-receipts-check.cjs`. They render actual frontend
builds at 390px/1440px and verify visible/hidden/closed/offscreen receipt behavior
without contacting customers or sending emails. Optional `CUAN_VISUAL_OUTPUT`
selects a private screenshot directory.
