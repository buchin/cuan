# Owner experience verification

Targeted Go tests cover profile validation/current-password confirmation, unchanged account IDs and sessions, new username/email login, SQLite-backed username persistence, notification read-state persistence, approval parsing without execution, incoming-customer/owner-reply distinction, quiet hours/timezone validation, private-endpoint/key rejection, Web Push encryption/VAPID signing, delivery dedupe and 410 cleanup through a fake transport. No real push service, provider or customer message is invoked by tests.

The production build passes TypeScript/Vite (existing CSS/chunk-size warnings remain). Persistent-profile Chromium verifies manifest parsing and installability with zero errors, service-worker registration, a simulated install prompt/appinstalled event, and an actual stopped fixture HTTP server producing the generic offline page. CacheStorage remains empty.

`browser.json` records desktop/tablet/mobile light/dark checks at 360/390/820/1440px, neutral inactive Account avatar, notification list/read/deep-link behavior, display-name/username forms, explicit simulated push permission and subscribe/unsubscribe, quiet preferences and reload persistence, Camera/Photos/Files wiring, dialog Escape/focus return, and the short Indonesian Inbox label. Browser errors and unknown API requests: zero.

Headless tests validate capture/file-picker wiring and PWA eligibility, not physical camera capture or OS installation. Push browser APIs and HTTP delivery are simulated; real device OS delivery is opt-in and is not claimed as verified. Original attachment and task execution behavior is covered by targeted regression tests.

## Production verification

Released `9c9ee7434f8e0cb1242ceed080923e9934526e23` through native atomic deployment. Live checks passed all eight viewport/theme cases: neutral inactive Account avatar, profile forms, notification SQL feed, notification settings, manifest/icons, service-worker no-store/root-scope headers, and exact compiled assets. Indonesian Inbox label and Camera/Photos/Files choices passed on mobile. No production username/display-name edits, notification read-state writes, real push subscriptions, provider invocations or customer messages were performed by the smoke test. Public VAPID setup generated only the private persistent application key pair/public response; no push was sent.

Production-host isolated tests passed profile validation/session preservation, notification/approval/customer classification, encrypted Web Push through a fake transport, subscription cleanup, and attachment/task regressions with a real PTY, without production database access. Readiness healthy; schema23 unchanged and automatic migrations disabled. All35 retained application tables and722 profile files match the private backup (login sessions excluded). Preferences and existing attachments were included in the private pre-release backup when present.

Real-device camera capture, system installation and push OS delivery remain device-dependent; headless coverage verifies native picker wiring, Chromium installability, simulated permission/subscription/install interactions, and encrypted transport behavior.
