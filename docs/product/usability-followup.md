## Authorized usability follow-up

The owner approved the full list after the onboarding and Inbox review.
This is incidental product work, with Scrum explicitly waived in this session.

- Onboarding: shipped7c54fa0, with draft-preserving AI sign-in, installation
  prerequisite feedback, retry and optional project connection. Unified native
  installation and full Docker AI support remain outstanding.
- Inbox: persistent archive/restore with Undo; long press, context menu and
  visible action button; truthful read/unread state and distinct typography.
- Messages: copy, quote and share link shortcuts; preserve text selection,
  ordinary tap and explicit approvals.
- Contacts: consistent action menu and persistent archive/restore; retain notes.
- Projects/tasks: consistent actions without swipe-triggered execution or status
  changes; preserve scoped task routes and honest work states.
- Notifications: distinct unread/resolved appearance, mark-read shortcut and
  consistent context menu.
- Agents: distinguish signed-in account, limited quota and failed connection;
  user must not infer work readiness merely from an existing account.
- Settings: distinguish unsaved, saving, saved and failed changes where editable.

Gesture constraints: touch scroll must remain natural, movement cancels long
press, all gestures have visible/keyboard alternatives, and no gesture approves
or executes work, deploys, or permanently deletes data. Archive is reversible
and must survive reload and use from another device. Test actual interactions,
not only screenshots. Keep production data and connected AI profiles intact.

## Shipped interaction pass

Release0e3e3c0 ships the Inbox/message/contact/project/task/notification actions,
persistent archive/read state, truthful idle-task labels, connection/quota copy
and settings save indicators. See docs/evidence/item-actions/ for exact-build,
touch/keyboard, failure and production evidence.

The latest screenshot review identifies the next visual refinements: archive
control in the filter row, quieter per-row menu buttons, and a transient compact
Undo notice. Preserve discoverable keyboard/touch alternatives.
