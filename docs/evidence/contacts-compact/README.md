## Contacts and cross-page polish

Contacts archive toggle now shares the search/filter toolbar instead of consuming a separate row. Search remains available for an empty list. Row names use medium weight, side padding is reduced, and filter/archive icons share a 44px target. Shared quieter action menus and transient Undo already apply to Contacts.

Notifications read titles now use normal weight while unread remain semibold. Projects search can shrink inside a narrow toolbar; empty-state padding adapts to mobile. Settings required no layout changes in this pass.

Isolated frontend build and cross-page gesture/persistence regression pass. Additional light/dark checks at 360, 390, 820 and 1440px cover Contacts, Projects, Notifications, Profile settings, Notification settings and General settings. No horizontal overflow; Contacts archive and search align and archive target remains 44px. No inference or task launches.

Run: node docs/evidence/contacts-compact/browser-check.cjs
