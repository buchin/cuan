# Read-only task verification

An approved repository audit must report what it found without editing project
files to satisfy unrelated implementation checks. A recovery launch retains the
Owner's continuation; automatic review feedback cannot expand that boundary.

Cuan binds execution policy to the exact launch/attempt. Explicit repository
read-only audit scope restricts publication and deployment, even when the
historical task still uses the code execution lane. Wording that merely requests
implementing a read-only editor or dashboard does not impose an audit policy.
Project guidance defaults do not override the task boundary.

Before executing the agent, the runner fingerprints its prepared repository:
HEAD, refs, local Git configuration, index/worktree differences, and nonignored
untracked file contents, modes and symlink targets. Existing retained work is
included in that baseline. The runner compares the final state without running
implementation test scripts for this audit. It does not revert changes or repair
unrelated tests. A missing baseline, unreadable repository, exceeded verification
budget, changed files or a new commit prevents successful completion.

The completion API requires matching runner fingerprints and rejects new commit
or PR metadata for this scope. It records `repository_scope_verified` against the
exact attempt and runner. Inbox acceptance requires that record; a passing model
review, a prose claim or an older attempt's receipt is insufficient. The review
receives the approved continuation and its scope so any requested correction must
stay within the audit. Task-specific factual evidence still needs acceptance.
Implementation tasks retain their existing project verification requirements.

This proves the repository state covered by the fingerprints, not the correctness
of every reported finding or absence of every external side effect. Git-ignored
runtime files, transient changes later reverted, and arbitrary remote mutations
are not proven by the fingerprint. Publication APIs separately enforce the
attempt's no-push/no-deploy boundary. This is an acceptance check, not an OS
read-only filesystem sandbox. No production task history is rewritten by rollout.

Verification is bounded: Git output at8MiB per command,4096 untracked files,
32MiB per untracked file,64MiB aggregate, and20seconds per fingerprint. Exceeding
the budget fails verification explicitly, retaining saved work for inspection.
No secret contents are returned in receipts or logs; only hashes are recorded.

## Regression evidence

- `go test ./internal/executionpolicy ./internal/task -run 'TestTaskPublication|TestReadOnly|TestAttemptPublication'`
- `go test ./internal/runner -run 'TestReadOnly|TestImplementationStillRequires|TestRunnerContinuation|TestRevisionNoPush'`
- `go test ./internal/server -run 'TestInboxReview.*ReadOnly'`
- Disposable PostgreSQL `TestLaunchPostgresContract` verifies that the exact
  attempt retains its scope while newer launches do not replace its policy.

The runner integration fixture exercises actual execution, report confirmation,
repository verification and completion submission: an unchanged audit succeeds;
a mock agent that edits `scripts/test.sh` fails. A coordinator fixture forces a
model pass and proves that missing runner evidence prevents acceptance.

## Verifier process lifetime and diagnostics

The fixed project verifier runs in a separate process group. Task cancellation or
expiry terminates that group, including ordinary test/compiler descendants. The
runner bounds inherited output-pipe waits to 500 ms and cleans up descendants left
behind by an exited script. A zero-exit shell with incomplete child output does
not produce passing project-check evidence.

Diagnostic output retains its final 12,000 bytes while running; it does not collect
unbounded stdout/stderr before truncation. Start failures use an unknown exit
status instead of claiming exit zero. Diagnostics distinguish task deadline,
cancellation, the verifier's five-minute cap, and inherited pipes left open.
Attempt logs record verification start, result and elapsed time, including a
bounded final observation after the execution context expires. These observations
never extend execution, accept missing evidence, reset correction budgets or resume
canceled work.
