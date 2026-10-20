## Universal search and contact-note discovery

Implemented global Spotlight-style search via the top-bar search button and Cmd/Ctrl+K, plus sidebar contact-note search. Results remain scoped to the authenticated owner, include bounded previews, preserve note/message navigation, and never invoke an AI provider.

Verified:

- Frontend TypeScript/Vite production build; pre-existing CSS/chunk warnings remain.
- Targeted Go: `TestCRMSearchNotesDistinctRankAndIsolation`, `TestUniversalSearchProjectTaskAndInboxScope`, existing contact memory CRUD. Coverage includes authentication, distinct contacts, identity-first rank, note history older than 50 records, filters, literal wildcard behavior, bounded category pages, and correct project/task/Inbox links.
- Actual PostgreSQL: `TestCRMSQLPersistence` verifies sidebar/global note retrieval and deletion exclusion. `TestUniversalSearchSQL` applies all migrations in a fresh schema of isolated `cuan_s25`, exercises real lightweight project/task/conversation queries, then removes its schema.
- Chromium fixture at 360/820/1440px × light/dark: note-only sidebar discovery, one contact row, selected-note navigation, five global result groups and no horizontal overflow. Also tests Ctrl+K, arrow navigation, Escape, error retry, result navigation and ignoring superseded slow sidebar responses. See browser.json and search screenshots.
- A 200-message fixture confirms a matching message older than the initial 50 is mounted and scrolled into view. Reproduce with `node docs/evidence/universal-search/old-message-check.cjs`.

Browser reproduction: build, then `node docs/evidence/universal-search/browser-check.cjs`. All fixtures are synthetic, with zero provider calls or production writes. No Lighthouse/Core Web Vitals claim is made.

Limits and API details are in `docs/guides/search.md`. Credential storage/masking and whole-contact/conversation archive/delete remain outside this release; those ideas were discussed and recorded without claiming them implemented.
