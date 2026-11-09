## Deployment cancellation scope correction

The retention task's saved developer continuation included:

> biarkan native publisher yang saat ini berjalan menyelesaikan rekonsiliasi efek tersimpan tanpa membatalkan deploy demi bukti semata.

Execution diagnostics for attempt `att_58b12ec79095c7cf1d4cd52897bcbc12`
incorrectly reported `deny_deploy=true`. The prohibition matcher associated
`tanpa` with the later noun `deploy`, ignoring that the prohibited action was
cancellation.

The policy now removes only bounded Indonesian/English cancellation cautions
before evaluating publication prohibitions. This grants no publication authority;
original task restrictions are still combined independently. Separate no-deploy,
read-only and following deployment prohibitions remain covered by regressions.

Validation: `GOGC=20 GOMAXPROCS=2 go test -p 1 ./internal/executionpolicy -count=1`
passed, including the exact saved release-reconciliation sentence and existing
publication-policy tests. No UI changes.

This root intervention does not retroactively change the running worker's saved
claim or prove native publication, independent acceptance, storage savings or an
untouched autonomous benchmark. Those outcomes require separate native evidence.
