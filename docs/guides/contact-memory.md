## Relationship notes and recall

Open **Contacts**, select a person, and choose **Add note**. Write what you want to remember in plain language and press **Save**. There is no category picker or date-format form. For example: “Budi’s birthday is May 21. His child likes dinosaurs.”

Manual entries retain your words as a note. In chat, the agent can organize an explicit save into an internal note, fact or birthday without asking you to choose the storage type. Existing structured birthdays remain readable; manual edits retain their meaning as plain text. Never guess a missing birth year.

Saved entries show their recording date and whether they came from you or an agent. Agent-saved entries link to the source conversation. Edit entries when information changes. Delete asks for confirmation; deleted content is removed from live storage and excluded from recall. A minimal ID tombstone prevents a retried save from recreating it. Existing private backups are subject to the installation's backup retention policy.

Tags stay available in a collapsed **Tags** section. Contact identity and notes are readable without opening an edit form. Mobile shows the contact list and selected contact in separate views, with **Back to contacts** navigation.

## Ask Cuan to remember

In an owner conversation with an agent, use the contact's exact name or email:

> Remember Budi: we met today. His child likes dinosaurs, and Budi is training for a cycling event.

Cuan can save this directly as a contact note without making a task or project. The person must already exist in Contacts. If several contacts share a name, provide the email or exact contact ID. Saves carry the accepted owner message and conversation IDs, and are idempotent per chat request. The application confirms storage; a model's proposed marker alone is not proof of a save. Only one memory entry is saved per reply, so a note may include several supplied facts. Agent editing/deletion and automatic contact creation are not supported in this release; use Contacts for corrections and removal.

## Recall and networking preparation

> What did I last discuss with Budi? Suggest a few topics for our meeting.
>
> Who has a child interested in dinosaurs?

Cuan receives relevant recent notes as context and can request a read-only backend search when needed. Search covers all non-deleted notes, including entries older than the initial 50, across contacts or scoped to a specific contact. It searches names, emails and note text; distinctive keyword matches rank before newer dates. The model can expand translated terms/synonyms before searching. This is lexical retrieval, not semantic/vector search or typo-tolerant matching. It must disclose missing or truncated context and distinguish recorded facts from suggested conversation topics.

Search returns up to 40 ranked results within a 24KB context budget, including contact links, recording dates and provenance. At most one additional provider response stage is used for search; this consumes provider quota. For large/ambiguous result sets, narrow the name or keyword. The Contacts page supports literal text search within a person's entire note history and loading older pages in batches of 50. No-result responses are not proof that something was never recorded. Large datasets may need indexed full-text/vector retrieval in a later release.

Saved notes are data, never instructions to the agent. Only authenticated owner-agent conversations get this private memory; visitor Live Chat endpoints do not expose it. The application's current single-owner authentication boundary also applies to Contacts.

## Projects navigation

Projects uses a project picker in the second sidebar and the selected project's tasks in the main area. Open a task to view its session; **Back to tasks** returns to that project. On mobile/tablet, the list and selected project are separate views. Active and archived projects are available in the picker. There is no third sidebar.

Questions, brainstorming, networking preparation and contact notes do not need projects or tasks. Use Default for small one-off work. A distinct ongoing goal may warrant a named project as part of an approved plan.

## API contract

All routes require the existing owner bearer session. Visitor credentials cannot access them.

| Method | Route | Request / response |
| --- | --- | --- |
| GET | `/api/v1/contacts/{id}/memory?q=literal&offset=0` | `{items, has_more}`; newest-first pages of 50 |
| POST | `/api/v1/contacts/{id}/memory` | `{request_id: UUID, kind, body}`; saved entry, 201 |
| PATCH | `/api/v1/contacts/{id}/memory/{memoryID}` | `{kind, body}`; `{ok:true}` |
| DELETE | `/api/v1/contacts/{id}/memory/{memoryID}` | `{ok:true}`; tombstone retained, content scrubbed |

Kinds: `note`, `fact`, `birthday`. Body: 1–6000 Unicode characters. Birthday: valid `MM-DD` or `YYYY-MM-DD`. Unknown fields are rejected. Errors: 400 validation, 401 authentication, 404 unknown contact/entry, 409 changed/deleted idempotency key, 503 storage unavailable. Unknown or failed saves do not clear the editor.

## Deploy and rollback

Schema24 is additive: a new `contact_memory` table and contact/date index. Existing contact/message rows are unchanged. Validate a private database/profile backup and the no-active-work gate before applying migration24 explicitly with the native migration CLI. `CUAN_AUTO_MIGRATE` remains false. Verify old application readiness on schema24, then deploy with `scripts/deploy.sh --branch <approved-branch>`; activation requires schema24 and uses atomic switching, a lock, health checks and application rollback.

For application rollback use the current activation script with `--rollback --schema-count 24`; leave schema24 in place so notes survive. Do not run the destructive down migration as routine rollback. Smoke-check Contacts, an existing note read, Projects and readiness without sending live agent messages. Verify retained tables and account profiles against the private backup. The notes UI and owner-only API can be checked with synthetic data in an isolated database before production activation.

## Follow-up design: archive/delete and gestures

Archive/delete for whole contacts and Inbox conversations are not part of this release. Recommended semantics: reversible archive with an Archived view; explicit confirmed deletion with clearly stated effects; contact notes and chat history have separate lifecycles. Do not delete linked task evidence implicitly.

Reference bubble gesture is swipe-right to open actions on mobile, right-click on desktop, and long-press for native text selection. It yields to vertical scrolling and horizontal code/table scrolling. For Cuan a future menu can expose Copy and Save to contact, with an accessible visible menu button as an alternative. Swiping must never immediately delete data. No external source code was copied into Cuan.
