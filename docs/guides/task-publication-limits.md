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
