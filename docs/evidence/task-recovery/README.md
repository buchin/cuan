## Task execution recovery checkpoint

Local implementation, not deployed yet.

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

Pending: immutable proposal/approval scope enforcement, clean release and
production verification. This is
not evidence that the full product goal is complete.
