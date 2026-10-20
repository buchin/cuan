## Contacts list and detail workspace

Desktop (1200px+) uses the main navigation, a320px searchable/filterable contact sidebar and a flexible detail area. Both panels scroll independently. Tablet/phone show list and detail one at a time; Back preserves filters. New contact remains in the global header. Existing identity/source/tags and connected-conversation actions remain; no new notes/history backend is invented.

TypeScript/Vite build and Chromium fixture passed ten light/dark viewport cases at360/390/820/1199/1440px, panel geometry, switching contacts, search/source filter, tag failure/retry/save/reload, chat link, missing contact, empty state and load failure/retry. No provider calls.

Production verification follows deployment.
