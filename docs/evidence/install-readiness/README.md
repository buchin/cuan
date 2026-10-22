## Fresh-install hardening

Installer contracts verified with `python3 scripts/test-install.py` (7 tests):
dry-run leaves configuration absent; generated secrets are private and retained;
an unreachable daemon fails before configuration creation; launch/readiness
failures never claim successful installation; existing placeholder secrets are
rejected; staging domain/custom port are written correctly; missing/invalid
arguments fail. Tests use disposable directories and fake Docker commands, with
no production messages or container mutations. `bash -n scripts/install.sh` and
`git diff --check` also pass.

Docker configuration now persists application data and account HOME independently
of PostgreSQL. Staging points to the actual `db` service and maps its configurable
host port to container port 8080. Build context excludes private environment files,
local data, logs, and Git history. The Go builder matches `go.mod` (1.27).

## Verification boundaries

This environment has a Docker CLI but no reachable daemon or installed Compose
plugin. Compose rendering and actual image build/container recreation were **not
verified**. Go image-tag availability was not independently established. Do not
treat these source changes as proof of a fresh-install runtime. Stock Docker AGY
execution remains unverified; native AGY sandbox evidence is separate.

No application binary or schema changed in this checkpoint. Production remains
on the previously verified schema25 release; these installation changes have not
been applied to production. Legacy container files must be exported before adding
new empty persistent mounts. Reconciled English onboarding, daily workflow,
message lifecycle, troubleshooting, and README document these limits explicitly.
