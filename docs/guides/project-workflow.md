## Manage a project

Use New project in the Projects list to create a project. Select a project to edit its name, goal and instructions for
Cuan. Use Save changes to apply edits. A repository is optional; connect GitHub
under Settings > Integrations when the project needs one.

Describe work in Studio or Inbox. Cuan proposes scope when approval is needed,
runs approved work and reports the result in the conversation. Use the work link
in a message to inspect its session. Project Activity offers an optional view of
all work; you do not need to manage that list to use Cuan.

New instructions apply to new work. Existing work keeps its original approved
instructions and workspace. Archive a project to hide it without deleting data.

The Settings and Activity tabs sit below the project name. Settings opens by
default; Activity provides optional work inspection. Project details
have no creation or chat button; use Studio or Inbox to describe work.

On desktop, New project sits above search in the project list and remains
available while viewing a project. Contacts and Inbox use the same placement for
New contact and New message. On mobile/tablet, use the action at the upper right
of the list page. The same action is not duplicated in the detail panel.

### When AI access is limited

Cuan checks permitted account/model routes before asking you to intervene. After
an eligible failure it checks another allowed account before more models on the
same account. A location-restricted model route is not repeated in that reply. Other
permitted accounts/models may be tried only when no uncertain tool effects were recorded. Each
reply has at most eight provider attempts and the existing request deadline.
Cancellation and unknown tool effects still prevent replay.

If recovery cannot complete, the chat preserves your message and work. The
**Connect another account** button opens the Google connection dialog directly,
without an agent-name form. Connecting saves access; it does not automatically
resend a failed message or authorize replay of uncertain work. No scheduled
quota-reset retry is promised. Account/model fallback remains subject to the
owner's existing settings. Review and verification are required before reporting
work complete, even when a replacement provider returns successfully.

### Google sign-in saved but setup incomplete

Cuan opens the first-run provider wizard in a sized terminal so its navigation
buttons are visible. If Google authorization succeeds but the wizard is still
incomplete, the confirmation flow resumes setup using the same saved account and
the terms consent given with confirmation. Optional interaction sharing stays off.
No new authorization code, agent name or replacement account is required. Setup
completion is taken from the provider's own saved state; Cuan does not manufacture
it. The owner-scoped verification endpoint accepts `finish_setup: true` with
`accept_provider_terms: true` for this bounded recovery. Provider quota or location
restrictions can still prevent AI execution after setup completes.

### Default model: Auto

New AI connections and work use **Auto**: Cuan omits the model override and lets
the CLI/provider use its configured default. Auto does not pin a Gemini version
or prefer Sonnet based on quota numbers. Connection verification follows the same
default path. An explicitly selected model is still respected. Allowed model and
account fallback are separate settings; switching is only attempted after a
failure that can be safely repeated, with no overlapping work.
