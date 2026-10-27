## Task execution recovery checkpoint

Production application release: 0071835c4c6f8e7a25f6ef549e23b6570a2d32d9.

A re-executed supervisor holds a kernel flock in the persistent workspace
execution-lock directory. A reconstructed manager and the task launch endpoint
check ownership before reserving a replacement. The supervisor places the agent
in its own process group, passes foreground PTY ownership to it, and terminates
that group on parent loss or shutdown before releasing its lock. The direct
agent also receives a Linux parent-death signal. Locks are not inherited by
provider children, so intentionally deployed services do not hold task ownership.
This fences local agents; detached external effects still require reconciliation.

Recovery guidance and automated coordinator review now include the latest
attempt's last eight reported progress checkpoints, chronological and bounded,
previous attempt status/error, and checklist completion flags. Terminal logs and
provider reasoning are excluded. Reports are explicitly evidence, not additional
authorization; the next agent must verify artifacts and reconcile uncertain
external effects before repeating them.

Targeted checks passed:

- Supervisor starts a real shell, rejects a duplicate while live, kills the old
  agent following parent loss, releases ownership, and preserves a checkpoint
  consumed by a continuation.
- Reconstructed terminal manager detects existing ownership; PTY/interactive
  manager checks pass. Symlink lock rejection and exit status propagation pass.
- Progress-only recovery evidence is preserved without summary/complete-note;
  unrelated attempts/comments are excluded and the last eight reports sorted.
- Existing fake-provider Inbox delegation and task-agent reporting/interactive
  checks pass (32.954 seconds). An initial 20-second command deadline was too
  short; the bounded rerun completed within a 45-second test deadline.

The integrated fake-provider flow also passes: a real task shell writes one
artifact, reports progress without a summary, and receives SIGKILL. Automated
coordinator review receives the saved checkpoint and requests a scoped revision
of the same task. The replacement verifies the prior artifact and original
no-deploy constraint, reports done, and produces one final coordinator response.
There is one task, two attempts and exactly one artifact creation.

Initial delegation identity now derives from the authorization message, rather
than a later triggering message. A stable launch request prevents re-execution
across different triggers. A stored scope digest rejects changed initial scope
or save-to-run elevation under that same approval. Tests cover managed projects,
normal delegation, subsequent triggers, changed scope/action and bounded revisions.
This pins the first stored scope; it does not prove natural-language consent or
bind an immutable proposal before approval.

Clean-worktree targeted checks passed, including task reporting and interactive
completion (35.246 seconds server tests). Native atomic deployment and readiness
passed. Read-only production mobile/desktop recovery controls passed. A service-user
probe of the actual production binary rejects an occupied lock with exit75 and
propagates a shell's exit7, with no provider invocation or task creation.
All 36 application tables (ephemeral auth sessions excluded) and 863 profile files
match the pre-release backup. Schema24 remains unchanged, auto-migration disabled.
Backup: /opt/cuan/backups/task-recovery-release. Previous release:
3b230b7c2225e69b4036983b657920328210e1d6. Build retains existing CSS syntax/chunk-size
warnings; these are outside this backend release and do not block the build.

Pending: immutable proposal/approval scope enforcement and the remaining product
audit requirements. This is
not evidence that the full product goal is complete.
