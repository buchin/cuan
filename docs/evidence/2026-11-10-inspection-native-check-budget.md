## Native inspection deadline exhaustion

Current-outcome attempt `att_89ac987e5833edb4205a924ec1c2ce50` returned
a saved model report, then exhausted the original task deadline during native
project checks after 183.924 seconds. Config/auth/db had passed; runner tests
were incomplete. Its report's claim that all criteria passed is not native
acceptance and must not override this failure. Its target was historical release
`2fc01ccb...`, not the subsequently published retention correction.

An allowlisted process observation confirmed the cache propagation fix was active:
the native Go process used the owner-scoped `go-build-runtime` GOCACHE and
GOMODCACHE under the existing lease. This establishes production configuration,
not a successful timing benchmark.

The reporting budget previously reserved only one fifth of the remaining window,
capped at two minutes, for live native checks that themselves permit five minutes.
Live inspections now reserve half of the remaining window, capped by that same
five-minute command budget. Source-only inspections retain their smaller window.
The overall task deadline, fixed test commands and acceptance gates are unchanged.

A native-recorded project-verification deadline after a saved final report can
now use the existing single infrastructure-recovery allowance. Eligibility
requires the exact native failure, provider exit zero, a canonical runner-authored
incomplete check record and repository baseline, without known assertion failure
or a changed observed boundary. Missing final fingerprint remains unknown, never
unchanged evidence. A fresh read-only snapshot must pass all original checks.
Existing immutable intent validation preserves source, scope, owner and repository;
only the freshly observed running target can change. Cancellation, missing report,
foreign evidence, actual assertions and a second infrastructure retry are excluded.
Failed history remains failed. No manual status or acceptance update is involved.

Validation:

- Real runner deadline/process tests, including strengthened reserved-window
  assertions, pass with the total deadline unchanged.
- Task evidence-classification tests cover missing/foreign/native failure records,
  changed boundaries, blocked reports, cancellation and assertion failures.
- Server lifecycle tests exercise canonical completion, durable deduplicated
  recovery on a fresh release, read-only claim and rejection of a second retry.

Commands:

`GOGC=20 GOMAXPROCS=2 go test -p 1 ./internal/runner -run 'TestInspectionReportingBudgetPreservesOriginalDeadline|TestInspectionReportingLeavesTimeForNativeChecks|TestProjectVerifier' -count=1`

`GOGC=20 GOMAXPROCS=2 go test -p 1 ./internal/task -run 'TestInspectionInfrastructure|TestInspectionProjectDeadline' -count=1`

`GOGC=20 GOMAXPROCS=2 go test -p 1 ./internal/server -run '^TestInspectionInfrastructureRecovery$' -count=1`

## Production recovery outcome

Release `675be07ad11b08d9e4425c47aac88605234be3da` completed with exact-SHA
healthy `/healthz` at `2026-11-09T22:05:52.253155599Z`; runner active and undrained.
The existing recovery automatically admitted attempt
`att_8fd72e99843a36c77dc6b2570f37fd05` against that exact running target while
preserving source `fe8c0cc2...` and its independent verifier. `/proc/PID/exe`
confirmed the detached worker used the corrected release binary.

The model returned its saved report after 236.965 seconds, before its reporting
deadline. Native project verification passed after 201.615 seconds. Total attempt
duration was 440.556 seconds; source, original scope and failed history remained
intact. These are different attempts/revisions, not a controlled cache benchmark.

PM request `706dbf79-2d4a-48ec-a89a-2902431d28f5` completed with
`decision=pass`, `requested_outcome=completed`, `remaining_work=none`. The
coordinator changed task `msg_b82e3c7d-138b-4914-bb84-4084c8b8b9cc` from review
to Done at `2026-11-10T06:13:12.291802+08:00`, with five of five criteria complete.
No operator retried that recovery, edited its acceptance or changed its task status.

The overall retention task still involved earlier root code fixes and a manual
publication recovery; it is not an untouched end-to-end autonomy benchmark.
No UI, scheduler or additional recovery allowance was added.
