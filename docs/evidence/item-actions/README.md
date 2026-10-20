## Consistent item actions and readable state

Inbox, contacts, project/task lists, notifications and chat messages share an
action menu: visible44px button, right-click, touch long press, Shift+F10 and
keyboard navigation/Escape. Vertical movement cancels long press. Only Inbox
archive and notification mark-read have swipe shortcuts. Short swipes do nothing;
a full horizontal swipe previews its action before release. Task execution,
approval, deployment and permanent deletion have no gesture triggers.

Conversation/contact archives and conversation read markers persist in a separate
owner-scoped0600 JSON file via an authenticated API. Independent fields survive
Undo/restore. An archived conversation resurfaces after a new message. Archive
does not remove messages/notes or stop work. A malformed preferences file is
reported rather than overwritten. Another owner sees independent data.

Incoming agent/customer messages are counted relative to saved read markers,
using message queries already performed by the contact list. No extra per-room
queries are added. Auto-read acknowledges only the last loaded message timestamp,
so a newer message arriving before the receipt is not silently marked read.
Unread titles are heavier, times blue and badges visible; selected background
remains a separate state. Manual unread and read state survive reload.

Messages offer copy, quote into the unsent composer and share links. Projects
show completed work counts. Tasks without active execution are labelled Not
running rather than Working. Agents distinguish connection from quota, with
attention states in amber. Profile/notification settings distinguish unsaved,
saving and saved; save failures retain drafts. Contact pagination is outside
row buttons and remains available when a page contains only archived results.

## Verification

An isolated exact production build passes TypeScript/Vite. Browser fixtures cover
360/820/1440px in both themes, plus detailed390px interactions: real CDP touch
long press and swipe, vertical/short-swipe cancellation, Escape/keyboard, read
and unread reload, archive Undo/restore/reload, failed archive retaining the row,
contact restore, project/task navigation, notification action menu, message
copy/quote without sending, truthful idle-task state, agent explanation and
settings failure/retry/saved behavior. No provider inference/task launches occur.

Targeted Go tests exercise authenticated persistent read/archive updates,
separate owners, private files, malformed-data preservation and a new message
arriving after the last seen timestamp. Existing complete chat-flow regression
also passes. Unrelated extension work is excluded from the release.
