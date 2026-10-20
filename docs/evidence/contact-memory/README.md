## Verification

Implementation: persistent contact notes/facts/birthdays, read-first contact details with collapsed tag editor, owner-agent save and read-only cross-contact search, and a project-picker sidebar with project-scoped task main view. Archive/delete of whole contacts/conversations and bubble gestures were inspected and recommended, not implemented.

- TypeScript/Vite production build passed (existing CSS/chunk warnings retained).
- Targeted Go tests cover authentication, field validation, birthday validation, idempotent retries/conflicts, cross-contact edit rejection, pagination, old-note search, deleted-note exclusion, provenance, ambiguous names and email disambiguation.
- Fake CLI integration verifies two provider stages: search action, actual backend retrieval, then final reply. No paid inference.
- Actual PostgreSQL test applies migrations22/24 in a fresh schema inside the isolated `cuan_s25` database; verifies restart persistence, keyword search, deletion exclusion and replay protection; removes the schema afterward.
- Six Chromium fixture cases: 360/820/1440px × light/dark. Verified project scoping, responsive picker, read-first details, old notes and search, no overflow; create/edit/delete and failure preserving editor text. See browser.json and contact screenshots.
- Native activation script safety tests pass, including exact schema/HTML, atomic switch, failed restart and invalid backup gates. A pre-existing schema21 test was updated to use the activation script's current schema constant.

Browser fixture: `node docs/evidence/contact-memory/browser-check.cjs` after building. Server: `go test ./internal/server -run '^(TestContactMemory|TestInboxContactMemory|TestInboxMemorySearchRound|TestCRMManualContact|TestInboxLifecycleContext)' -count=1`. SQL test additionally needs the documented isolated database environment. See the user/API/deployment guide at `docs/guides/contact-memory.md`.

Limitations: lexical retrieval (no embeddings/typo tolerance), result/context limits, one extra search response stage, no automatic memory extraction without an explicit save instruction, and no agent edit/delete. Whole-contact/conversation archive and bubble actions remain follow-up scope. Synthetic/fake-provider tests do not establish the answer quality of a live language model.
