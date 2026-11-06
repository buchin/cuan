## Task publication limits

A task's explicit prohibition on push or deployment overrides generic delivery
instructions and project auto-deploy defaults. Cuan recognizes restrictive
English/Indonesian clauses in the task instructions, including the original
approval retained by managed delegation. This recognition only restricts; it
never grants permission or interprets unrelated positive text as authorization.
Project guidance snapshots are excluded from this task-level policy.

The policy adds worker instructions, disables the automatic git publication flow
for no-push tasks, rejects task-capability pushes before repository access, and
skips automatic apply/deploy and deployment completion requirements for no-deploy
tasks. Other project defaults remain unchanged. Local artifacts and reporting
still require their normal verification gates.

This is not a general natural-language permission parser or an OS network
sandbox. Arbitrary paraphrases and languages may require clarification; native
commands still must obey the explicit scope in the worker instructions. Do not
claim that all possible external effects are mechanically prevented.

Regression checks cover a retained Indonesian approval forbidding both actions,
English negative clauses, unchanged ordinary deployment, exclusion of project
snapshots, denied capability push and successful local completion under a
project with auto-deploy enabled. Resume the same canceled task after this fix;
do not create a duplicate or silently widen the approved scope.

## Verification for documentation-only work

The runner checks committed, staged/unstaged and untracked changes against the
project's known default-branch commit. When every changed path is Markdown,
verification checks readable nonempty UTF-8 text within the workspace instead of
running unrelated application tests. Symlinks and oversized/invalid text fail.
Mixed changes, missing baselines, failed git inspection or unknown state use the
normal project verifier. This does not establish task-specific word limits,
content correctness or delivery: those still require acceptance evidence and
accessible artifacts in the task report.

## Registered verification prerequisites

Review's `publication_context.connection_state` and a bound worker's
`publication_configuration` describe saved production verification setup:
`not_configured`, `unsupported`, `incomplete`, `configured`, or `current_process`.
`unavailable` means the project could not be read. These are not deployment,
health, feature or repository-permission results. A configured but unreachable
server differs from a project without a registered connection. An independently
verified receipt still retains its own task/attempt/commit-bound semantics.

Missing setup is supplied as a concrete prerequisite to the coordinator and
worker. Reconcile the approved target and existing owner-saved connections;
never assume the deployment failed, repeat implementation/deployment merely to
obtain a receipt, or borrow ambient developer credentials. Only genuinely absent
access or consent requires the owner. Keep other original acceptance gaps intact.

A native `publication_config_observed` activity records only project ID,
setup state and project update time. It contains no credential values, host data
or deployment claim. A changed saved project configuration changes the review
evidence digest and wakes the existing deduplicated snapshot review path. It does
not execute a task, grant consent, reset attempt budgets or accept work. Existing
cancellation, owner lineage and native publication/visual gates remain in force.

When required publication remains unverified and its registered connection is
missing/incomplete/unsupported, native review waits on that prerequisite instead
of launching another implementation attempt. A task worker cannot use its task
capability to rewrite Owner-scoped project settings. Neither a model's replan nor
a changed local `.env.deploy` file registers a trusted Cuan connection. Other
acceptance requirements remain unaccepted and must be checked after setup.
A changed saved connection wakes one fresh review of the same task; configuration
alone never starts another execution or marks the work complete.

For a configured connection with unverified publication, the first remaining
step is verification of the existing production result. Native review normalizes
that step to verification even when a model labels it implementation. The
reviewer's concrete findings stay in the revision for inspection; they are not
waived. This allows the independently authorized read-only current-outcome path
before another implementation or uncertain deployment is attempted. An existing
wrapped revision cannot bypass that ordering by retaining an old classification.
