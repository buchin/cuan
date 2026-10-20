## Contacts list and detail workspace

Desktop (1200px+) uses the main navigation, a320px searchable/filterable contact sidebar and a flexible detail area. Both panels scroll independently. Tablet/phone show list and detail one at a time; Back preserves filters. New contact remains in the global header. Existing identity/source/tags and connected-conversation actions remain; no new notes/history backend is invented.

TypeScript/Vite build and Chromium fixture passed ten light/dark viewport cases at360/390/820/1199/1440px, panel geometry, switching contacts, search/source filter, tag failure/retry/save/reload, chat link, missing contact, empty state and load failure/retry. No provider calls.

Deployed code `6379051c9da31f01410ba729addb609ad20db160` to https://cuan.dojo.cc through the native atomic release. Eight production light/dark viewport checks at360/390/820/1440px passed with temporary synthetic contacts, including list/detail switching, preserved search, tag save/reload and the chat-link destination. Exact deployed assets matched the build. No real messages or provider calls occurred; two synthetic contacts were removed without creating rooms/messages.

Readiness healthy; validated private backup retained.35 existing tables and706 account-profile files unchanged (login sessions excluded), schema23 unchanged/no migration. Project checkout fast-forwarded cleanly to deployed code. Production screenshots contain synthetic contacts; unrelated tag suggestions are masked. See `production-browser.json`.
