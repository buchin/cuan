## Projects and task overview

Project sidebar rows show a compact name/count and the most important attention/active/review state, without a growing stack of totals or redundant timestamp/chevron. Not running contributes to attention counts.

Task rows show status, short reference, agent when present, update time and the saved summary. Completed tasks prefer the completion note. No per-task detail request is made for these rows. Filters expose All, Working (including queued), Needs attention (including not running), Review and Completed. Search includes saved summary/reference/agent.

Review remains the actual backend review state; no human approval requirement is invented. Arbitrary live activity is not claimed when only a saved summary is available.

Isolated build and targeted cross-page regression pass; filter membership, result-over-progress, agent/search, compact project height and no overflow checked in light/dark at 360/390/820/1440. No task launches or provider inference.

## Production delivery

Live release d1c78532b54b59614a647b3c16d67acf7c7c76aa; health/readiness and actual rollback/reactivation passed. Schema remains25 with auto-migration disabled. Retention verified37 application tables,904 unchanged profile files,6 item preference entries and11 notification read flags. Read-only production browser checks cover filters across360/390/820/1440 in light/dark; all application writes were blocked after login. No provider inference, task submission or title update was invoked in production verification. Title semantics are verified through deterministic backend integration tests, not a live model.
