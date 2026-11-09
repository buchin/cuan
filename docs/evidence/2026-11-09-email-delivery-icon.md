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
production outage. No email was sent for this visual check.

Release730958c534bb1f34b835823c34faa202b97fb89a passed exact-SHA production
health with38 migrations at2026-11-09T04:28:15Z. The same existing customer
message67094169-3e6e-4590-86a0-9fdcbbf970a6 still has visitor/delivered state
after activation; no resend was used to verify the presentation.
