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

## Production

Native atomic release `5004f6d4884adb26c3ad3526aec4062b6a9ebff7` activated successfully at https://cuan.dojo.cc with schema24 and automatic migrations disabled. Private database/profile backup validated; old application readiness passed after additive migration24. Read-only smoke confirmed authenticated note API (unauthenticated access rejected), contact details at 390/1440px, and project picker/scoped tasks, without chat/task writes or provider calls. All 35 pre-existing application tables and 742 account/profile files retained unchanged. See production.json.

The manual editor was subsequently simplified to a single plain-language field and Save; it no longer exposes a storage-kind picker or date-format field. The browser CRUD/error recovery scenario also asserts that the picker is absent. Agent chat saves may still organize information internally, while manual notes preserve the owner's wording.

Final production application release: `5e92bbae0d34a775fff9f371c5e7033153404398`. Includes the bounded save-context correction and single-field editor. Read-only smoke reconfirmed note API/authentication, the single-field editor at mobile/desktop sizes, Projects navigation, and zero provider calls. Schema24 healthy; 35 pre-existing application tables and 742 profile files remain unchanged.
