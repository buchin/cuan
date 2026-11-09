# Resend connection

Open **Settings → Integrations → Resend** to configure the email sender. The key
is encrypted by Cuan and the Settings API returns only connection metadata.
Use an address on a domain verified in your Resend account. Credential validation
does not send an email; sending the test email is a separate explicit action.

For authorized administration, the official CLI uses the same authenticated API:

```sh
cuan integration resend show
cuan integration resend set --key-from-env RESEND_KEY --from-email noreply@example.com --from-name Cuan
cuan integration resend validate
```

Only the explicitly named environment variable is imported. Never put a key in
command arguments. Output excludes secret values and provider error bodies.
Task-bound sessions cannot change Owner settings. Configuration preserves omitted
fields, including automatic-email preferences and existing notification recipients.
A valid key does not prove a message was delivered. Test delivery separately to
an authorized recipient when needed. Resend's official HTTP API is used because
this integration needs web application email delivery, rather than an interactive
provider CLI sign-in.

Production Cuan uses the user-authorized organization `RESEND_KEY`, sender
`Cuan <noreply@dojo.cc>`. The key was validated and the `dojo.cc` domain confirmed
verified through Resend. Idle automatic notifications remain disabled until
configured deliberately; no recipients or test messages were added during setup.

## Team invitations

**Settings → Team → Undang Anggota** remains available to Owners when the shared
Settings header replaces the embedded page heading. Enter an email, select Member
or Owner, create the invitation, and share its link. Creating the link does not
send an email. Members cannot create invitations. Browser coverage:
`node docs/evidence/team-invite-check.cjs`; server authorization and invite
acceptance: `go test ./internal/server -run TestTeam_`.

Automatic email can be changed explicitly with
`cuan integration resend set --enabled=true` (or `--enabled=false`). Omitting
this flag preserves the current preference. Enabling delivery does not backfill
historical sent/delivered claims. Signed receipt events use the installation's
own Resend webhook at `/api/webhooks/resend`; configure email sent, delivered,
delayed, failed, bounced, complained and suppressed events. Import its signing
secret with `--webhook-secret-from-env NAME`. Chat read status remains separate
from email delivery, and email open tracking is not enabled.

## Unread conversation reminders

Admin reminders consult persisted `resend_email_logs` before sending. Restarting
or deploying does not reset successful-send history: an unchanged customer
message is not mailed again, and new reminders retain a five-minute cooldown.
If history cannot be read, sending stops rather than assuming nothing was sent.
The Resend request also supplies a stable idempotency key for the conversation
and latest customer message. Provider idempotency has a bounded retention window;
it is additional protection, not an indefinite exactly-once delivery guarantee.
The database remains the retained successful-send history.

Notification templates use Cuan branding, neutral typography, a single quiet
conversation action and flat message rows. They do not identify Cuan as a shop.
Email preview verification uses isolated content at390px and1440px; it does not
send messages or prove rendering in every email client.

Public chat replies currently persist in the Inbox and visitor chat transport.
Resend is wired for admin reminders and explicit integration tests, not an
outgoing visitor-reply email queue. Enabling Resend alone does not email public
replies to customers. Operator notification receipts must never be presented as
customer email delivery. Historical public replies are not automatically resent.
