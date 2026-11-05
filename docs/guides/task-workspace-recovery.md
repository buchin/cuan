# Recovering saved task work

Ask Cuan to continue the existing work in its original Inbox. Cuan preserves the
task, project and approved scope. A missing working directory does not mean the
task must be duplicated or its history discarded.

For an authorized continuation with concrete revision feedback and an earlier
attempt, Cuan can build a separate working copy from the retained committed
branch. The original branch and Git metadata remain untouched. When available,
the original index and staged binary patch are saved as private recovery evidence.
The agent inspects this evidence and verifies the original completion criteria.
Unstaged or untracked files that disappeared are not claimed recovered; missing
artifacts must be recreated within the approved scope or reported as blockers.

Opening task details alone does not trigger reconstruction. Damaged retirement
receipts, changed repository/branch identity, interrupted reconstruction evidence
and another deletion of the replacement workspace stop for inspection. They do
not cause an automatic reset. Canceled work requires a fresh instruction to
continue; accepted completed work remains accepted.

A stopped launch is blocked, rather than indefinitely queued, when it is still
the latest launch and there is no queued/running attempt. Capacity backoff remains
queued and retries through the saved launch. Avoid repeatedly sending the same
instruction while its request or attempt remains active.

Implementation evidence: [PM recovery](../evidence/pm-recovery.md).

A terminal answer and a completed task are separate observations. The agent must
save a final task report with evidence; the runner then closes its execution and
Cuan evaluates the original criteria. New reporting sessions retain their scoped
authorization across server deployments using encrypted durable grants. Restart
does not authorize unknown tokens or another task. Older sessions created before
this repair may need a scoped continuation that verifies saved work and reports
it without repeating implementation or external actions.

Publication restrictions in a continuation (for example, “verify only; do not
push or deploy”) are bound to that execution attempt. They narrow project defaults
and apply to the runner and server, including automatic commit/push and auto-apply.
A later continuation cannot relax an earlier attempt's restrictions. This does
not waive original acceptance criteria: unavailable live-install evidence must
still be reported honestly as a gap or blocker.

Cuan owns recovery of an already-authorized execution and advancement through its
approved work. Repeated manual resume messages or administrative status changes
are not an orchestration mechanism. External development sessions diagnose and
repair the product; they must not finish Cuan's backlog on its behalf or create
success records to make a recovery test pass. Escalate to the user only when a
real prerequisite cannot be resolved by the agent within the approved scope.

## Legacy provider compiler caches

Compiler state belongs in the runner's per-owner `go-build-runtime` cache, not
inside each AI sign-in profile. Brain failovers reuse that owner cache without
sharing credentials or another owner's modules.

Native deployment runs `scripts/prune-provider-go-cache.py` before transferring a
release. It removes only recognized Linux Go toolchain distributions older than
24 hours from legacy provider `home/go/pkg/mod/golang.org` directories. The same
account execution gate used by native sessions is held exclusively during pruning;
a leased account, a running binary/cwd using the toolchain, a missing gate, an
unknown module or a symlinked directory is preserved. Credentials, provider
history, attachments, workspaces and the managed owner cache are outside its scope.
The Go distribution is redownloadable if a legacy profile needs it again.

For an operational dry run, supply the actual account directory explicitly:

```sh
python3 scripts/prune-provider-go-cache.py --accounts /path/to/cuan/accounts
```

Add `--apply` only for authorized maintenance. This operation cannot restart tasks,
change their status, reset attempt budgets or make a failed attempt successful.
