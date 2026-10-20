## Projects and task overview

Project sidebar rows show a compact name/count and the most important attention/active/review state, without a growing stack of totals or redundant timestamp/chevron. Not running contributes to attention counts.

Task rows show status, short reference, agent when present, update time and the saved summary. Completed tasks prefer the completion note. No per-task detail request is made for these rows. Filters expose All, Working (including queued), Needs attention (including not running), Review and Completed. Search includes saved summary/reference/agent.

Review remains the actual backend review state; no human approval requirement is invented. Arbitrary live activity is not claimed when only a saved summary is available.

Isolated build and targeted cross-page regression pass; filter membership, result-over-progress, agent/search, compact project height and no overflow checked in light/dark at 360/390/820/1440. No task launches or provider inference.
