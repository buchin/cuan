## Native checks lost the leased build environment

Retention developer attempt `att_58b12ec79095c7cf1d4cd52897bcbc12`
finished its model stage in 574.411 seconds. Native project verification then
took 296.389 seconds and passed. Its final failure was the missing native
publication receipt, not a test timeout. These measurements do not establish
an autonomous speed improvement.

An allowlisted `/proc` observation of its running Go process showed the expected
workspace and test packages, but no `GOCACHE` or `GOMODCACHE` override. The runner
had prepared and leased an owner-scoped build environment for the agent, then
called native checks with the supervisor's ambient environment instead.

Native checks now receive a copied allowlist of execution build settings under
the existing lease. They retain their fixed commands, original deadline and
acceptance requirements. The environment includes no additional task/provider
credentials or project secrets; explicit command environments remain unchanged.

Regression uses a real repository `scripts/test.sh`: two runs reuse each owner's
cache, a second owner uses a separate cache, the ambient cache is overridden,
later model-environment mutation has no effect, and arbitrary project settings
are excluded. The test fails with the previous nil-environment dispatch and
passes with the fix. Existing process-tree cancellation, bounded output and
managed-release admission regressions also pass:

`GOGC=20 GOMAXPROCS=2 go test -p 1 ./internal/runner -run 'TestProjectVerifier|TestManagedReleaseAdmissionRequiresExplicitFrozenScope' -count=1`

This is a root intervention. Production cache reuse and elapsed-time improvement
still need observation in a later execution; local fixtures are not a production
benchmark.
