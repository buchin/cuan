# Coordinator boundary verification

Source: `8a79f3533897c74487f1979fff70ec049ef980f3`.

## Staging installation

The native installer activated the release on the dedicated Dracin Cuan test
installation. Binary checksum matched its release manifest. Existing configuration
was preserved, database backup taken, no migrations applied, readiness passed with
schema 25. Production was not changed.

## Native plan-mode check

One isolated invocation used the dedicated Google AI Pro test account and
`claude-sonnet-4-6`, with `--mode plan --sandbox`. The prompt requested a harmless
text file in a new disposable writable directory. AGY attempted `write_file` and
reported `denied_actions: [{action: write_file, display_name: WriteFile}]`.
The requested file was absent. This demonstrates native denial of this write
tool; it does not establish that every tool or write path is impossible.

The process exited 0 with status SUCCESS but an empty response. The denial and
missing response must not be mistaken for completed work. Existing Inbox result
validation handles denied tools; this direct CLI probe did not exercise the full
application conversation path.

## Recovery remains incomplete

Read-only application inspection before upgrade confirmed the original task
`msg_31d19b1f-d81b-4474-94e2-c3fc784988f4` was blocked with one failed attempt
(exit 137), no summary and no completion evidence. All three associated Inbox
requests were completed. The previous coordinator reply incorrectly claimed
completion and linked files under its provider profile. No user goal or approval
was replayed, and no replacement task was created during this verification.

Next acceptance gate: continue the original task with preserved approval and
workspace, obtain a real execution report, review against every criterion and
provide user-accessible deliverables. Automatic continuation and fresh-install
AI onboarding are not yet proven complete.
