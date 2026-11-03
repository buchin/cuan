# Public package review

Read-only GitHub review on 2026-11-03. No external repository was modified or
package published as part of the Settings release.

| Existing package | Cuan usage | Release state observed |
| --- | --- | --- |
| [taskfsm](https://github.com/buchin/taskfsm) | Imported in runner/service.go; pinned Go pseudo-version | Public, MIT; tests in repository; no tags or Actions workflows returned |
| [dodeval](https://github.com/buchin/dodeval) | Imported in runner/dod_evaluator.go; pinned Go pseudo-version | Public, MIT; tests in repository; no tags or Actions workflows returned |
| [agentrunner](https://github.com/buchin/agentrunner) | Not imported in Cuan go.mod | Public, MIT; tags v0.1.0–v0.1.2; supervisor/executor/timeout/workspace/secret tests; no Actions workflows returned |

Prioritize these packages before extracting more. Add CI and tested consumer
examples, publish intentional version tags, and update Cuan through explicit
version bumps. `taskfsm`'s README still shows the old Cuan-internal import path;
its example needs correction before presenting it as ready for outside users.
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
