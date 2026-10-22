## Find things in Cuan

Choose the search icon in the top bar, or press **Cmd+K** / **Ctrl+K**. Type at least two characters to search across Contacts, Notes, Conversations, Projects and Tasks. Results are grouped by type with a short preview. Use Up/Down to move between results, Enter to open, or Escape to close. **Show more results** loads another page without running an agent or consuming AI-provider quota.

The top-bar button is available on mobile/tablet/desktop wherever the normal page header is visible. Fullscreen work/chat layouts retain their existing chrome; the desktop shortcut remains available. Native dialog focus trapping, close behavior and focus restoration are reused from Cuan's existing modal component.

## Contacts sidebar

**Search contacts and notes…** finds names, emails, tags, sources and saved note contents, including old entries outside the first note-history page. Each contact appears once. Direct name/email matches come first, followed by contact metadata and note-only matches. Note matches include a short excerpt around the matching text. Tag/source filters still apply.

Select a note match to open that contact with the matching note search and selected-note outline. Search results use literal case-insensitive text; `%` and `_` are ordinary characters. Results load in pages of 50 contacts. Searches debounce by 250ms; superseded requests are aborted and cannot replace newer results. Edits/deletions refresh the sidebar search. Deleted notes are excluded.

## Global results

Global search performs literal case-insensitive matching. It is not a semantic/vector search or a natural-language agent query. Ask an agent in Inbox for interpreted recall or networking suggestions instead.

- Contacts: identity/metadata and matching saved notes, deduplicated by contact.
- Notes: saved text with a contact link that opens the matching note.
- Conversations: Inbox-compatible agent/customer rooms, matching titles/customer identity or saved message text. A message match opens and outlines that message, loading enough older history to make it visible.
- Projects: name/repository URL, including archived projects.
- Tasks: title, instructions, summary and completion note, linking to the project-scoped task where available.

Initial global results are limited to five per category; **Show more results** advances by five. A missing category's storage does not masquerade as an empty search: it displays a category-specific error. Search does not change data or start tasks. Project discussion rooms that are not Inbox conversations are not linked as chats. Standalone attachments/binary contents and provider diagnostics are not indexed as separate results. Attachment-message envelopes are text in storage; matching filenames may also produce a conversation result.

## API and performance

Authenticated owner routes:

| Route | Parameters | Response |
| --- | --- | --- |
| `GET /api/v1/contacts/search` | `q` (1–200 characters), `offset`, optional `tag`, `source` | `{contacts, has_more}` with optional bounded `search_note:{id,snippet}` |
| `GET /api/v1/search` | `q` (2–200 characters), `offset` | `{groups:[{kind,items:[{id,title,snippet,url}],has_more,error?}]}` |

Offsets are bounded; global requests use a four-second deadline. PostgreSQL queries load bounded result pages and lightweight task columns, without fetching task attempts, provider profiles, or whole conversation histories. Contact note previews select one latest matching note per contact. In-memory test stores have equivalent filtering and bounded responses. Existing metadata-only contact list remains compatible. No schema migration is needed beyond the already deployed schema24.

Literal substring searches can still scan text as the dataset grows. This release bounds result transfer and client updates; it does not claim indexed semantic retrieval or large-dataset scalability. Full-text/trigram indexes can be considered after measuring real workloads. Results depend on existing single-owner authentication and are not exposed to visitor Live Chat.

## Configuration principles and credentials

Cuan should follow repository/deployment conventions and let agents handle routine configuration. Ordinary conversation and relationship memory need no project/task. Use Default for one-off work; use a distinct named project for an approved ongoing goal. Ask users only when ambiguity has meaningful consequences.

Automatic credential capture, encrypted project-env storage, execution injection and sensitive-message masking are **not implemented by this search release**. Stored chat/note text remains ordinary searchable data. The agreed direction is accepting legitimately provided project access through chat, using existing secure configuration mechanisms where possible, masking the persisted conversation/preview/search/notification path, and asking for the target project only when ambiguous. A cosmetic bubble mask alone would be insufficient. Do not claim current searches redact arbitrary secrets or that current chat automatically stores credentials safely.
