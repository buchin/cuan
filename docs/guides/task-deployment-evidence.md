# Deployment evidence for task acceptance

For code work requiring automatic deployment, report prose and a runner boolean
do not establish delivery. Completion requires a structured `deployment_verified`
activity from `cuan-deploy`, bound to the exact task, attempt, and full commit SHA.
The receipt must describe a healthy production release of that same commit and
be recorded during the attempt. The runner checks its committed, clean workspace
state; the completion service independently checks the persisted receipt.

The native activation publisher in `scripts/activate-release.py` writes this
receipt after checking production health and executable release identity. A
receipt records deployment; it does not itself complete the task. Reusing a
receipt from an earlier attempt or an unrelated commit is rejected. Observed
code changes cannot be exempted by sending `requires_deploy=false`.

After loss of an ephemeral agent grant, the runner can read typed deployment
evidence through the existing authenticated active-attempt report endpoint.
That response omits task instructions, environment and arbitrary activity data.

This is an activity-store trust boundary, not cryptographic host isolation.
Agents with unrestricted host/database access remain outside that boundary.
Other deployment integrations need their own authoritative receipt producer
before they can satisfy this check; free-form deployment claims are insufficient.
This check also does not replace the separate review/apply lifecycle gates.

Agent report submission does not trigger auto-apply: it precedes runner
verification and cannot authorize publication by itself. The auto-apply helper
also rejects missing, running, failed and cancelled attempts. Explicit owner
apply with auto-deploy enabled leaves the task in review and returns
`deployment_pending=true`; a successful merge or restart does not mark it done.
Automatic reconciliation of that pending deployment remains to be implemented;
these guards do not establish end-to-end acceptance for every apply path.

Verification:

```sh
go test ./internal/task ./internal/runner -run 'TestVerifiedDeployment|TestDeploymentReceipt|TestRunner_AutoDeploy|TestConfirmAgentReport' -count=1 -timeout 30s
go test ./internal/server -run 'TestRunnerReportRead|TestQualityGate_DualPath' -count=1 -timeout 30s
```

Missing publication evidence is a verification gap, not proof that deployment
never happened. The Inbox publication guard applies this distinction to both
passing reviews and verification-only revisions, independently of the execution
harness. It requires reconciliation of the registered running release before
any further publication. Concrete implementation findings and additional
verification obligations remain intact; missing evidence never grants acceptance.

Implementation revisions also receive the reconciliation prerequisite when
publication is required but unverified. The reviewer's classification cannot
bypass it. Concrete implementation findings remain unchanged and must be
checked against current evidence before execution; this guard does not certify
that those findings are accurate.

The task-bound `stop-hook` CLI handler is an initial AGY lifecycle adapter.
It reads the native JSON event on stdin and returns a native JSON stop decision.
It requires a runner-created private `CUAN_TASK_LIFECYCLE_DIR` and an absolute
`CUAN_TASK_REPORT_DEADLINE` (RFC3339). Only a normal fully idle stop with a
successful scoped active-attempt read and no current final report can request
one report-only continuation. Cancellation, errors, unavailable reads and an
expired reporting budget allow stopping. The handler never submits evidence or
accepts work. The runner now writes a private per-invocation `.agents/hooks.json` under its
diagnostic lifecycle directory and supplies that directory as an additional AGY
workspace root. It leaves repository and account configuration untouched.
Configuration and PTY shutdown safeguards are tested; discovery by the real
authenticated CLI and end-to-end recovery still require native verification.

Final-report interpretation is shared in `task.TaskDetailResponse.FinalAgentReportState`.
Runner shutdown and lifecycle adapters use the same latest-report rule: a
nonempty result and evidence, an explicit final report state, and the exact
attempt identity are required. A later progress report invalidates an earlier
final report. This protocol is independent of harness; native event transport
and callback registration remain harness-specific. It does not certify process
success, correct implementation or publication.

### Independent SSH release observation

The native runner can now request an independent observation when an exact
production receipt is absent. This reads the existing release; it never deploys.
The server resolves its target exclusively from saved project settings, checks a
pinned SSH host key before password authentication, reads the immutable
`current -> releases/.../REVISION` binding and checks the registered HTTPS health
URL. A matching revision and healthy response produce the existing trusted,
exact-task/attempt receipt. Acceptance and original feature checks remain separate.
An unsupported/missing configuration still fails verification; it never waives
publication or treats report prose as proof. This works independently of harness.

Maintainer configuration belongs in Project settings → Environment, managed with
`cuan project env set`; users do not run SSH commands or enter these technical
defaults in ordinary work management. The initial adapter supports Deployer-style
immutable releases with a `REVISION` file and password authentication:

- `CUAN_PUBLICATION_VERIFIER=ssh-release`
- `CUAN_PUBLICATION_SSH_HOST`, `CUAN_PUBLICATION_SSH_USER`,
  `CUAN_PUBLICATION_SSH_PASSWORD`, `CUAN_PUBLICATION_SSH_HOST_KEY`
- `CUAN_PUBLICATION_DEPLOY_PATH` (absolute deployment root)
- `CUAN_PUBLICATION_HEALTH_URL` (HTTPS health endpoint)

Each value can instead reference another key in that same saved environment by
using the `_ENV` suffix, such as `CUAN_PUBLICATION_SSH_PASSWORD_ENV=PROJECT_PASSWORD`.
No value falls back to the process environment, SSH agent, developer login or
another account. The host key must be the independently trusted OpenSSH public
host key; do not silently trust a fresh network scan. Missing host trust requires
connection setup, never `StrictHostKeyChecking=no`. Key-based SSH authentication,
automatic target discovery and ordinary connection-setup UI are not implemented
by this adapter yet.

The runner-only `POST /api/v1/runner/attempts/{id}/publication/verify` accepts only
`task_id`, `runner_id` and the expected `commit_sha`. It rejects caller-supplied
facts, commands or targets. The attempt must remain current, leased, running and
uncancelled; the project connection and result must remain unchanged during the
bounded check. Existing matching receipts are reused. Errors omit credentials
and remote diagnostic output. Tests cover real fixture SSH/TLS transport,
wrong host identity/revision, oversized output, cancellation, unhealthy/redirected
health, missing saved configuration, task/attempt mismatch and revoked connections.
The native Dojo check also passed against the existing registered project
connection and previously trusted host key: revision
`4e2e71ca265600be19687fbfad705db6f861d245` matched and HTTPS health returned 200.
No deployment was performed. Runtime receipt publication through the deployed
server and task acceptance still require production verification.
