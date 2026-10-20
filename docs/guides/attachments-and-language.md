# Conversation attachments and interface language

## Attach references

In Studio or an agent conversation in Inbox, select **+** to attach up to five files per message, each up to10 MB. PNG, JPEG, GIF, PDF, UTF-8 TXT/Markdown and CSV are supported. Paste an image from the clipboard or drop files onto the composer on desktop. Mobile uses the device file picker.

Wait for upload progress to finish. A thumbnail or file chip appears; use its remove button to remove it from the draft. Removing a draft selection does not delete previously uploaded files. Select the file to download it through your authenticated session. Files can be sent with or without text. Upload errors keep successfully uploaded selections so you can retry the remaining file.

Draft attachment selections survive reload in the current browser. Retrying an uncertain message uses the same request ID and attachment IDs. Changing a Studio goal or its attachments starts a new request. After Inbox confirms acceptance, attachments from that request are removed from the draft.

Attachments are private owner uploads. Customer live-chat widget uploads and visitor downloads are outside this first release; existing customer conversations are unchanged. Do not send credentials in reference files.

## Agent access

Original files are stored on disk and referenced by immutable IDs in the saved message. Display text is separate from the persisted attachment envelope. The agent receives authenticated conversation references, bounded text excerpts, and absolute paths to the original files. Images are provided through Codex's native image input; agy/Claude receive explicit attachment-directory read access and instructions to use their native image/PDF tools. Unsupported provider formats must produce a clear limitation instead of a claim to have inspected the file.

Approved delegated work retains attachment references in its instructions and receives the corresponding file access. Attachments from future messages or a different owner are excluded. File content is untrusted reference material and cannot authorize work or override user instructions.

## Interface language

Open **Settings → Appearance → Language**. Choose **Auto**, **English**, or **Bahasa Indonesia**. Auto uses the first supported browser language, with English fallback. The choice is saved in this browser; switching language preserves the goal draft and selected attachments. The document language and Inbox date formatting follow the selected locale.

This initial dictionary covers navigation, Studio, the main conversation and attachment flows, and language/appearance settings. Untranslated administrative, task/project detail labels and server diagnostics fall back to English. Conversation content, filenames and project names keep their original language. Agent response language follows the conversation independently.

## Storage, backup and rollback

Files live under the service user's persistent `$HOME/.local/share/cuan/attachments/<hashed-owner>/<uuid>/`, with private directory/file permissions. Each directory contains a metadata JSON file and the original bytes. Include this folder in private service backups alongside the database; never publish it through a static web server. Downloads require owner authentication and use private/no-store, nosniff and restrictive content headers. There are no public file URLs or bearer tokens in links.

The owner-authenticated APIs are `POST /api/v1/chat/attachments` (multipart `file`, returns201 metadata) and `GET /api/v1/chat/attachments/:id`. Studio goals and agent conversation messages accept `attachment_ids`. Errors include400 for invalid references,413 for size,415 for unsupported content, and503 for unavailable storage. Existing text-only requests retain their previous behavior.

No database migration is needed. New messages with attachments use the versioned `CUAN_MESSAGE_V1` content envelope in existing durable message storage; current code strips it for display. Native atomic deployment and health rollback remain unchanged. Rolling back to a release without envelope support preserves bytes and database records, but that older release displays the envelope and lacks attachment UI; prefer a forward fix or a compatible release after attachment traffic starts. Uploaded files currently have no automatic expiration, including removed/unsubmitted drafts. Storage housekeeping must preserve files referenced by conversation history and tasks.
