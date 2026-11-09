# Task duration

Duration should answer how long the user has waited for the requested result.
Measure from `created_at` to the native Done transition, including admission,
development, independent verification and final PM review. It does not include
the initial discussion before the task exists.

Task detail and task listings expose optional `completed_at`, read from the
latest persisted `status_change` activity only when the current task is Done.
It is not `updated_at` and not the latest attempt's `finished_at`: the former
changes on edits, while the latter omits PM acceptance and earlier attempts.
List reads batch the transition history for the returned Done tasks. No schema
migration, worker execution or historical status rewrite is involved.

Missing/contradictory history leaves `completed_at` absent. Do not substitute the
last edit timestamp or display a successful completion duration for stopped,
failed, canceled or reopened work. A later genuine Done transition replaces the
earlier completion when the work is reopened and accepted again.

The API foundation is implemented; duration display is not yet implemented.
The proposed quiet display is `Berjalan 3 menit` / `Running for 3 minutes` during
active work and `Selesai dalam 7 menit` / `Completed in 7 minutes` once accepted.
Use minute-level refresh without another status pill. Avoid estimated time left
until sufficient data supports an estimate. Detailed phase timing needs attempt
and review intervals; elapsed lifecycle duration must not be described as model
compute time. Paused or user-blocked time remains part of elapsed time and must
not be labeled continuous agent execution.

Before shipping the display, inspect real desktop and390px mobile screenshots,
check long durations/localization, confirm terminal timers stop and that later
title edits do not change completed duration. API omission means unknown, not0.
