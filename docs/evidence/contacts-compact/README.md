## Contacts and cross-page polish

Contacts archive toggle now shares the search/filter toolbar instead of consuming a separate row. Search remains available for an empty list. Row names use medium weight, side padding is reduced, and filter/archive icons share a 44px target. Shared quieter action menus and transient Undo already apply to Contacts.

Notifications read titles now use normal weight while unread remain semibold. Projects search can shrink inside a narrow toolbar; empty-state padding adapts to mobile. Settings required no layout changes in this pass.

Isolated frontend build and cross-page gesture/persistence regression pass. Additional light/dark checks at 360, 390, 820 and 1440px cover Contacts, Projects, Notifications, Profile settings, Notification settings and General settings. No horizontal overflow; Contacts archive and search align and archive target remains 44px. No inference or task launches.

Run: node docs/evidence/contacts-compact/browser-check.cjs

## Production verification

Live release: 1fd2b0da25d2ccea4a7e6dc696b19e9d026b6be9 at https://cuan.dojo.cc/contacts.

Read-only production browser verification passed for menus, responsive layout and Contacts toolbar alignment. All application non-GET writes were blocked after login, including automatic read receipts. No provider inference or real task/chat submission. Actual rollback/reactivation passed; schema 25, auto-migration disabled, readiness healthy.

Retention: 37 application tables unchanged (auth sessions excluded), six item preference entries unchanged, 11 notification read flags preserved. Of 896 snapshotted profile files, 894 are byte-identical and two antigravity-oauth-token files changed concurrently, with no deletion. Their source is not attributed by this verification. Updated token files were preserved rather than restoring potentially stale credentials. No token contents were printed.

Existing build warnings about CSS minification and bundle size remain.
