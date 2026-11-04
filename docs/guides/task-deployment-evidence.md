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

Verification:

```sh
go test ./internal/task ./internal/runner -run 'TestVerifiedDeployment|TestDeploymentReceipt|TestRunner_AutoDeploy|TestConfirmAgentReport' -count=1 -timeout 30s
go test ./internal/server -run 'TestRunnerReportRead|TestQualityGate_DualPath' -count=1 -timeout 30s
```
