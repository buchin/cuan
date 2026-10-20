## Editable project settings

Name, goal, instructions, and Advanced repository/branch are editable and saved.
Save disables when unchanged; failure retains draft, conflicts require explicit
reload, connection changes ask confirmation. New task is absent from Settings.

Backend uses atomic expected-version updates. Task guidance and workspace revision
are frozen at creation and retained when task instructions are revised.
Connection changes validate GitHub access/branch and create a new workspace.
Previous bindings are authenticated and retained; no existing workspace is
checked out, reset or removed. Task recovery resolves the original binding.
Existing task instructions remain uncluttered in the frontend.

Targeted tests: real local Git clone/checkout and failed connections, retained
workspace/file/guidance across service reconstruction, tamper rejection,
owner-only API/stale writes, task CRUD/attempt invariants, direct and
approval-based process-loss recovery and topic-title/delegation regression.
Browser covers save/reload/failure/conflict/advanced and responsive light/dark at
360/390/820/1440. Three release compatibility gate contracts pass.

Actual clean candidate binary + disposable PostgreSQL: two settings saves,
canonical version round-trip, stale 409, frozen original task context,
new blank-workspace revision, install/reinstall/restart. No provider invocation,
production DB writes, or real GitHub access in the tests. Temporary DB/HOME removed.

A sandbox RAM incident stopped compilation; compilation was rerun with
GOMAXPROCS=1, GOFLAGS=-p=1, GOMEMLIMIT=300MiB and GOGC=20. Frontend builds use a
384MiB Node heap. Existing CSS minification/bundle-size warnings remain.
