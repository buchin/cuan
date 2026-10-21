## Project settings as the primary view

Production db8f0f6/schema25 makes project selection open settings: project name,
goal, guidance and optional repository connection. Activity retains the existing
task list and shareable task/session URLs. Ask Cuan opens Studio; no New task
button is shown in the project workspace. Project rows show guidance or connection
state instead of task-management counters.

The production browser check passed at390/820/1440: settings form, Activity
navigation, Ask Cuan, absence of New task and no horizontal overflow. API writes
were blocked after login, no production inference was used. See production.json.
Screenshots remain local because they include actual project metadata.

## Settings and Activity tabs

Production37845e3 supersedes the earlier header actions: Settings and Activity
are adjacent navigation tabs below the project name. Settings is the default;
New project appears on the list only, and Ask Cuan/New task are absent from
project details. Actual production navigation and active-tab markers passed at
390/820/1440 with post-login writes blocked. See project-tabs.json. The browser
check waits for route rendering before checking active-tab state.
