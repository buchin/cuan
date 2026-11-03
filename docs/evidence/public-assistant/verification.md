# Public assistant and AI brain routing verification

The implementation covers Studio routing, legacy conversation continuity,
automatic native brain capacity routing, truthful work counts, editable assistant
and brain metadata, and a quiet settings/list presentation.

## Automated evidence

- Owner-isolated brain selection, cross-process leases, cooldowns and persistent metadata edits.
- Native PTY quota failure is distinct from user cancellation; early output history is inspected.
- A failed native brain leaves a workspace marker; the next brain resumes it under the same task and attempt.
- A capacity-limited Inbox message stays queued without another message/request; cancellation prevents replay.
- Canonical assistant identity wins over legacy Studio choices and remains stable on replay.
- Legacy room IDs/history remain intact; new turns resolve to the configured public identity.
- Review count is separate from working count in contact lists and conversation details.
- Slow repository clone retains the project and enough request time to persist synchronization status.
- Native cgo race instrumentation was unavailable in this sandbox; normal concurrency/lease tests were run.

## Visual evidence

Screenshots render the built application in Chromium at 1440×1000 and 390×844.
The preview uses authenticated read-only production data, with state-changing
requests stubbed so inspection does not alter production preferences. Backend
status behavior is covered by targeted API tests and is checked after rollout.

- `tasks-desktop.png` and `tasks-mobile-390.png`: no Agent column or assignee picker; readable title hierarchy.
- `studio-desktop.png` and `studio-mobile-390.png`: one public assistant, project selector only, quieter focus treatment.
- `settings-desktop.png` and `settings-mobile-390.png`: flat brain list, one status summary, no backup-selection badges.
- `brain-drawer-desktop.png` and `brain-drawer-mobile-390.png`: editable name/model, connection details and actions in the drawer.
- `inbox-desktop.png` and `inbox-mobile-390.png`: public assistant header and no account-selection panel.

Visual inspection: screens have no horizontal document overflow; mobile touch
controls remain usable, and settings/list typography has clear primary and
secondary levels. The brand is stronger without increasing the header height.
