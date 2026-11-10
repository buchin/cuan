# Managed Go build state

AI account profiles isolate sign-in and provider settings. They must not own a
separate compiler/dependency/build cache for each account. On the Cuan VPS, the
system compiler was Go1.22.2 while the project required Go1.27.0; Go therefore
downloaded the newer toolchain into each profile's default module cache.

For Go repositories with a regular root `go.mod` or `go.work`, an owner-bound
runner now provides stable GOPATH, GOMODCACHE and GOCACHE directories under
`go-build-runtime/<owner-digest>` beside the workspace root. The source checkout
and provider credentials remain separate. Different owners get different roots;
brain selection and failover do not change these cache locations. Directory links
are rejected so a cache cannot silently refer to another owner's directory.

Go continues to select and verify the toolchain declared by the repository using
its standard automatic toolchain mechanism. This is not an unconditional compiler
upgrade or a claim that dependencies have already downloaded. The first use may
still need a download; subsequent routes reuse the same owner's cache. No host
Go configuration or credentials are imported (`GOENV=off` by default). Explicit
project Go environment overrides are preserved, including the default module
cache relationship when a custom GOPATH is configured. Non-Go projects and
unowned/unconfigured internal execution retain their existing behavior.

Managed Go executions default to `GOFLAGS=-trimpath` when the project supplies no
nonempty override. A stable cache directory alone cannot reuse compilation of
the same package at different absolute workspace paths. Trimming those paths
lets direct agent `go test`/`go build` commands reuse compiled packages across
developer and verifier snapshots, as the registered Cuan check script already
does. This changes embedded source paths, not which tests run. An explicit
project GOFLAGS value is preserved, including during native publication; native
check recipes retain control over their own flags rather than importing arbitrary
agent flags.

Existing profile caches are not deleted by this change. Cleanup requires a
separate retention/in-use check. This change does not restart failed tasks or
reset execution/recovery budgets.

Verification: native runner dispatch regression plus
`go test ./internal/runner -run 'TestManagedGoBuild|TestLiveInspectionRunsFreshChecksInSnapshotWithoutPublication'`.
Tests ask a real Go binary to resolve its environment under two private HOMEs,
check stable owner caches and owner separation, reject directory links, and
verify that native task execution receives the managed defaults.
