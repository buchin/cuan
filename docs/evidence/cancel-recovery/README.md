## Cancellation-aware recovery release

Release `98f8811` is active in production. A cancelled task requires a later
accepted user instruction before coordinator continuation; an old approval or
system review cannot start it again. The guard is checked at delegation and
queued execution, and saved checkpoints remain available.

Clean-checkout targeted tests passed (7.386s): cancellation delegation and queued
rejection, fresh-user versus synthetic/unaccepted turns, progress preservation,
direct/approved process-loss recovery and queue reconstruction. Build passed.

Production readiness, actual rollback/reactivation, retained application data and
profiles passed. Browser checks at 360/820/1440px in both themes found no overflow
or page errors and confirmed assets exactly match the tested release. All API
writes after login were blocked by the verification harness. No task submission,
retry, GitHub write or AI inference was performed.

See release.json and production.json for results. This establishes deployment
and deterministic recovery behavior, not actual-provider end-to-end completion.
GitHub OAuth setup remains absent and accurately displayed as Setup needed.
