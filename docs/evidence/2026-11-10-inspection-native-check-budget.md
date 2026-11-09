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

This is another root correction; successful autonomous recovery still needs
production evidence. No UI, scheduler or additional recovery allowance was added.
