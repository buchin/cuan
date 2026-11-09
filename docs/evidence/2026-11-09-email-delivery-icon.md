## Delivered email status

Inbox now renders a muted14px MailCheck icon for delivered email, retaining the
timestamp and separate chat receipt. Visible delivered/team success text is
removed; title and accessible image label retain delivery state, recipient type
and the distinction from chat reading. Failure/bounce/delay states keep readable
text. Internal messages suppress email indicators.

`npm run build --prefix web` passes. `node web/tests/email-delivery/check.cjs`
renders the exact built Inbox with isolated fixture APIs at390px and1440px in
light/dark themes, with zero chat/provider writes. It checks icon-only delivered
states, accessible recipient context,14px geometry, retained failure labels,
internal-message suppression and no horizontal overflow. All four screenshots
were opened and visually inspected: quiet inherited colors, no added status pill,
natural message hierarchy, compact metadata and preserved composer space.

Screenshots and checks are outside tracked source at
`/home/dev/artifacts/cuan-email-icon/{light,dark}-{390,1440}.png` and `checks.json`.
The fixture's connection notice is simulated transport state, not evidence of a
production outage. No email was sent for this visual check. Deployment pending.
