# Navigation and responsive UI

Cuan currently uses English for its interface, validation messages, and new system
activity. Localization is planned for later. User content, saved conversations,
project names, and tags retain their original language. Agents respond in the
user's language.

## Find your work

The main navigation is **Inbox → Tasks → Projects → Contacts**. On desktop it
appears in the sidebar; on phones the same four destinations appear in the bottom
navigation. Settings is separate from these work areas.

- **Inbox:** conversations with agents and live chat customers. Search or filter
  by All, Agents, or Customers. Choose a conversation to open it.
- **Tasks:** the global backlog, with project and status filters. Open a task to
  view its description, discussion, execution attempts, and saved output.
- **Projects:** connected repositories and project boards. Opening a project
  limits its board to that project. Project settings shows its saved repository
  details and archive action.
- **Contacts:** CRM records, tags, and customer conversation links. See the
  [Contacts guide](contacts-crm.md).

## Create something

Use the button at the top right. Its visible label follows the current page:

| Page | Action |
| --- | --- |
| Inbox, including a phone conversation | New message |
| Contacts | New contact |
| Tasks or a project board/settings | New task |
| Projects list | New project |
| Settings → Agents | New agent |
| Other Settings pages | New, followed by a choice of item |

There is one primary creation action per page. There is no duplicate New project
shortcut at the bottom of the sidebar. New task on a project page selects that
project. Archived projects are excluded from new-task choices.

New message chooses an existing recipient; opening a conversation does not send
anything. Draft text is kept separately for each conversation while the page
remains open. Reloading clears unsent local drafts. A pending sent request uses
its existing durable request ID; checking it does not start a duplicate run.

Dialogs have visible field labels, keyboard focus containment, Escape to close,
and focus restoration. A failed save retains the form for retry. Closing is
blocked while a save is in progress so its result stays clear.

## Sidebar and account

On wide desktops, the sidebar shows labels and the account row displays an
avatar, username, and more icon. **Settings** sits directly above it. Open the
account row for **Profile & password** and **Sign out**.

The collapse control is beside the logo at the top. In collapsed mode, hovering
or focusing the logo area reveals Expand sidebar; it remains keyboard accessible.
Your expanded/collapsed preference is saved in this browser. Tablets use a compact
icon sidebar; the account menu remains available from the avatar.

Phones have no duplicate sidebar drawer. The avatar beside the creation button
opens the account menu, including Settings. In a phone conversation, Back returns
to the conversation list and restores the bottom navigation.

## Settings

- **Agents:** add an agent, connect its account, inspect connection status, and
  manage it. The legacy `/team` URL redirects to `/settings/agents`.
- **Integrations → Live Chat:** installation code, optional visitor identity,
  and verification steps. Setup is separate from the Contacts directory.
- **Appearance:** Light, Dark, or System. Your selection is saved in this browser.
  There is no duplicate theme shortcut in the sidebar footer or phone navbar.
- **General:** workspace and project settings links.

A green Connected status requires a successful check with a valid timestamp less
than five minutes old. Not checked, Sign-in required, Quota limited, Connection
error, and Disabled are distinct states. Browsing the UI does not verify a
provider or sign an agent in automatically. A provider quota limit does not
require another sign-in.

Use Manage agent connection in the chat header to open that agent's settings.
Back to conversation appears when there is a valid return link.

Project settings reflects stored configuration. Editing repository connections
is not offered on that page. Archiving preserves repository and project data;
restore from Projects → Archived. Both operations require confirmation.

## Responsive sizes and typography

- **1200px and above:** 200px sidebar, collapsible to 64px. Inbox has an optional
  280px conversation list beside the chat.
- **768–1199px:** 64px sidebar. Inbox uses a single list/conversation view.
- **Below 768px:** four bottom tabs; no hamburger drawer. The header contains a
  24px kepeng logo, a 20px title, a contextual creation button, and an avatar.
- App headers are 56px on phones and 64px on larger screens. Main pages use one
  H1 and no redundant breadcrumb. Project/settings details provide parent links.
- Chat body text is 16px on phones and 15px on larger screens, with line height
  1.55. Reply headings are 16–18px; paragraphs and lists use compact spacing.
  The composer stays 16px. Message timestamps are 12px.
- Primary touch controls are at least 44px. Safe areas and visible viewport
  changes are respected by the phone composer.

Chat uses incoming white/charcoal and outgoing green bubbles. A single check
means Message saved, not read by the recipient. Markdown, code copy, horizontal
code/table scrolling, and response details are supported.

## Tasks from chat and response details

Ask an agent to prepare a task from the discussion. It produces an editable
Draft task; review the title, description, and project, then select Create task.
The task enters the backlog. Creation does not start agent execution. Task drafts
use bounded conversation history, so repeat important older constraints when
needed. Retrying confirmation for the same source message does not create another
task. Unsaved draft edits do not survive reload.

Details shows recorded runtime events, saved request/message IDs, and safe errors.
It does not reveal private reasoning or credentials. Opening Details, refreshing,
or checking a pending request does not rerun a failed user message automatically.
Chat analysis can use read-only project tools. Implementation belongs in a task.

## Verification and release evidence

Current English/sidebar checks: `docs/evidence/english-sidebar/`. The fixture
browser check serves the production build with isolated API responses. It covers
80 route/viewport/theme combinations, creation dialogs, focus/Escape, desktop
collapse, tablet account access, phone navigation order, recipient draft
isolation, and idempotent contact retry. The widget check covers initialization,
identity, retry, and responsive controls. Production results are recorded
separately. Chromium viewport emulation is not physical-device or Safari testing.

Earlier UI evidence is retained in `ui-refresh`, `integrations-navigation`,
`mobile-navigation`, `agent-roster`, `whatsapp-chat`, `crm-patterns`,
`header-hierarchy`, `context-new`, and `chat-typography` under `docs/evidence/`.
For production health, backup, and rollback procedures, see
[Messages and tasks operations](message_task_lifecycle.md).
