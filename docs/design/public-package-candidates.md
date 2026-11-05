# Public package review

Read-only GitHub review on 2026-11-03. No external repository was modified or
package published as part of the Settings release.

| Existing package | Cuan usage | Release state observed |
| --- | --- | --- |
| [taskfsm](https://github.com/buchin/taskfsm) | Imported in runner/service.go; pinned v0.1.0 | Public, MIT; executable consumer example, test/race/vet workflow, v0.1.0 release |
| [dodeval](https://github.com/buchin/dodeval) | v0.1.0; requirement parsing in Inbox delegation and runner gap analysis | Public, MIT; safe requirement API, executable example, test/race/vet workflow |
| [agentrunner](https://github.com/buchin/agentrunner) | v0.2.0; process cleanup adapters in internal/agent/process_tree.go | Public, MIT; hardening merged and v0.2.0 released; Go checks workflow added |

Prioritize these packages before extracting more. Add CI and tested consumer
examples, publish intentional version tags, and update Cuan through explicit
version bumps. `taskfsm`'s public import and executable example were corrected for v0.1.0.
Keyword matching in `dodeval` must remain a gap-analysis aid, not proof that work
actually passed independent checks.

`agentrunner` is the strongest existing candidate for reducing duplicate low-level
process code. It already advertises locks, process-tree termination, masking,
timeouts, and workspace setup. First compare its implementation against current
Cuan invariants: shared saved deadline, per-attempt ownership, lease expiry,
atomic completion, read-only limits, account isolation, restart/drain, and safe
workspace retention. Introduce one adapter (process supervision/termination first)
and prove parity before replacing more. Database transactions, task approval,
private account routing and Cuan product policy remain in Cuan.

The new UI primitives are still being exercised in Cuan. Keep them local until
their API is stable and a second consumer exists; avoid publishing a large generic
UI framework now. A small dependency-free process utility is a better next public
package only when existing agentrunner cannot reasonably own it.

## AgentRunner source findings

Inspected `timeout.go`, `executor.go`, and `go.mod` through authenticated GitHub
reads. `executor.go` starts from `os.Environ()` and appends request values; this is
not equivalent to Cuan's private-account environment allowlist. It also captures
stdout/stderr in unbounded `bytes.Buffer` instances and assumes one provider's
`-p`, `--output-format`, and `--dangerously-skip-permissions` flags. `Version()`
uses an unbounded command without a context. These are concrete parity gaps,
not reasons to discard the package. Add explicit environment and bounded-output
contracts, configurable provider arguments, and cancellation/version-probe limits
before using its executor in Cuan. Tags and existing tests alone do not establish
that it meets the current account-isolation/lifecycle requirements. No external
package test suite was executed during this read-only review.

## Implemented after approval

AgentRunner hardening is merged at 294b661 and released as v0.2.0. The CLI adapter now uses an explicit environment allowlist, bounded streams with truncation signals, request/version timeouts, opt-in permission bypass, and provider argument overrides. Truncated output is excluded from structured evidence. Local package tests and vet pass. GitHub-hosted jobs could not start because of an account billing lock; no hosted success is claimed. Cuan imports v0.2.0 through three stable process helper wrappers. Its descendant-process and lifecycle/account regression checks pass. Runner task orchestration and private account routing remain Cuan-owned.

## taskfsm v0.1.0 (2026-11-05)

The public release is available at https://github.com/buchin/taskfsm/releases/tag/v0.1.0.
The package API and lifecycle behavior are unchanged. Its README now documents
installation, executable examples, guard locking and listener semantics. Package
tests, race tests and vet passed locally. GitHub-hosted checks did not start due
to an account billing lock; no hosted pass is claimed. Cuan resolves v0.1.0 through
the public Go module transport and its runner state-machine integration test
passes against that version. No additional framework or duplicate FSM was added.

## dodeval v0.1.0 (2026-11-05)

Released at https://github.com/buchin/dodeval/releases/tag/v0.1.0 and pinned in
Cuan. `ParseRequirements` distinguishes unverified approved work from tracked
checkbox state. Explicit acceptance sections, numbered parentheses, deduplication
and long preambles are covered by package tests. Existing tracking APIs remain
compatible. Cuan's duplicated parser was replaced with a small application adapter;
criterion limits and application section labels remain Cuan policy. The runner
also uses the unverified requirement API, so prechecked plans cannot substitute
for outcome evidence. Package tests/race/vet and published-module consumer tests
passed locally; hosted checks remain unavailable due to account billing.
