# Owner experience verification

Targeted Go tests cover profile validation/current-password confirmation, unchanged account IDs and sessions, new username/email login, SQLite-backed username persistence, notification read-state persistence, approval parsing without execution, incoming-customer/owner-reply distinction, quiet hours/timezone validation, private-endpoint/key rejection, Web Push encryption/VAPID signing, delivery dedupe and 410 cleanup through a fake transport. No real push service, provider or customer message is invoked by tests.

The production build passes TypeScript/Vite (existing CSS/chunk-size warnings remain). Persistent-profile Chromium verifies manifest parsing and installability with zero errors, service-worker registration, a simulated install prompt/appinstalled event, and an actual stopped fixture HTTP server producing the generic offline page. CacheStorage remains empty.

`browser.json` records desktop/tablet/mobile light/dark checks at 360/390/820/1440px, neutral inactive Account avatar, notification list/read/deep-link behavior, display-name/username forms, explicit simulated push permission and subscribe/unsubscribe, quiet preferences and reload persistence, Camera/Photos/Files wiring, dialog Escape/focus return, and the short Indonesian Inbox label. Browser errors and unknown API requests: zero.

Headless tests validate capture/file-picker wiring and PWA eligibility, not physical camera capture or OS installation. Push browser APIs and HTTP delivery are simulated; real device OS delivery is opt-in and is not claimed as verified. Original attachment and task execution behavior is covered by targeted regression tests.
