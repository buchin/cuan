# Reconciled task checkpoint publication

Task msg_d0843350-b9f0-4682-b326-c0adaa2498e8 retained checkpoint
68d623b493b124bf5fcbe561debfa552ba7559f5 after baseline reconciliation and native
checks passed (47.029s). Attempt att_ca1022be1721c7694e5d284918b85e89 then logged
HTTP409 from branch publication and remained failed at the publication gate.
The remote task branch still pointed to c84c84b82c6d86b758204d050504e5edfe65ed5a;
registered workspace ancestry inspection returned 1 for that source relative to
68d623b4. Thus the existing normal push cannot fast-forward this rebased result.
The historical HTTP response did not expose a stage code, so it does not rule out
an additional earlier publication prerequisite.

The project publication path now recognizes only the Git client's confirmed
remote-divergence category and reuses existing PushExecutionSource. It retains a
clean exact checkpoint on an immutable source ref rather than rewriting the old
remote branch. It returns no PR against the original branch's older contents.
Unknown outcomes, access errors and other push failures do not take this path.
No native acceptance, source-review or production receipt is manufactured.

A real bare-repository fixture reproduced the failed rebased publication before
correction. It now confirms exact immutable source, idempotent reconciliation,
preserved original remote history, no stale-source PR, and rejection/preservation
of dirty source and repository-rule failure. Existing immutable source tests also
cover saved credentials, changed heads, wrong repositories, unavailable access,
conflicting refs and concurrent writers. Targeted project tests pass (0.349s),
and Git publication groups pass (3.663s). Production rollout remains unobserved.

PM independently admitted source verifier att_354e19154459d9cd6eff7681fe33a396
on exact 68d623b4 after the receipt failure, without replaying developer work.
The task is not accepted or deployed at this observation.
