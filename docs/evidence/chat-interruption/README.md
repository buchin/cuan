## Production chat interruption diagnosis

Read-only inspection of the reported IP question found two provider stages.
The first ended successfully after about 39 seconds and requested saved-note
search. The second repeated native inspection until the request was interrupted.
The provider's own local session records showed six terminal failures referencing
a missing /proc/self/exe. No raw conversation content, token, command arguments,
runtime IDs or credentials are reproduced here.

The external sandbox omitted proc entirely. CLI --help startup passed, but that
did not cover AGY's subprocess helper. The sandbox now mounts read-only proc in
its new PID namespace. Actual service-user tests execute /proc/self/exe as a
terminal helper, verify the host PID/environment is absent, reject project
writes, read explicit attachments and preserve provider state. AGY --help also
passes. This is dependency-path verification without paid inference, not a claim
that a live model/tool turn was sent.

No-match note search now returns a clear clarification without a second model
call. With matches, one 25-second answer-only stage runs without project or
attachment mounts and with terminal auto-approval disabled; any tool event cancels
that stage and yields a clear clarification. Fake-provider tests verify both
branches and the successful retrieved-note answer.

FinishInboxRequest records a final timestamp atomically even after 100 ordinary
activity entries. Final event persistence uses a cancellation-independent short
context. Interrupted bubbles no longer reuse an earlier stage's duration.
Existing interrupted outcomes are not rewritten or rerun.
