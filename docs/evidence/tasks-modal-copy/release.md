# Minimal modal copy release

Production release `fb2b6ec768be8207a18a2021fc495bd4104d1ece` is active at
https://cuan.dojo.cc. Native atomic activation and readiness passed. Schema23
is unchanged; no migration. Persistent checkout fast-forwarded to this release.

The modal now says New task, uses the placeholder Describe your task, and shows
Project, agent selection and Create/Create & start. Promotional introductory copy,
normal help text and decorative icons were removed. Agent-load failure still
shows actionable help; No agent clearly identifies the save-only option.

Vite/TypeScript build and existing browser workflows passed. Public production
smoke passed eight viewport/theme combinations, prompt/button states and exact
JS/CSS/font bytes. It made no product mutations or provider calls. Screenshots
filter the background to avoid exposing real task content.

A private validated backup captured36 tables and507 profile files before release.
Concurrent real task execution occurred during post-release checks: two attempts
were recorded (failed then succeeded), with status Todo→Review and assignee update.
Original task title/instructions and all original row identities were retained.
Thirty tables matched exactly; other differences were added execution records and
normal task/project-relation timestamps. Eight profile files changed during that
activity, so exact whole-profile hash retention is not claimed. No concurrent
work or profile changes were discarded or overwritten.

Rollback can activate previous application release f4c8220 while retaining
schema23. The change only affects modal presentation/copy.
