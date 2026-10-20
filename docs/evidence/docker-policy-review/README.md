## Alternate proc bind review: reproduced and confined in a fixture

The earlier experimental AppArmor-only replacement of OCI masked/readonly mounts
is insufficient by itself. The controlled non-root payload unshares a new user
and mount namespace, recursively bind-mounts its own /proc under /tmp, then opens
the alias of /proc/keys despite the original path denial. The probe only opens
and closes the descriptor; no kernel file contents are read or emitted.

Creating fresh procfs was denied with EPERM; nested Bubblewrap was denied by
read-only uid-map files. Those two denials did not establish protection against
the successful bind alias. before.json and before-worker.txt reproduce it.

A second seccomp filter passed to Bubblewrap via --seccomp applies only after
filesystem/namespace setup. It denies mount, umount2, pivot_root, unshare and
setns, denies namespace-bearing clone, returns ENOSYS for clone3 so normal
process/thread creation can use clone, and permits other calls. It checks the
amd64 audit architecture and rejects x32 syscall numbers. Raw BPF, decoded
instructions, exact worker and SHA-256 are included.

With that payload filter, the exact bind-alias reproduction stops at unshare
with EPERM. The official musl AGY1.4.3 version/help, read-only project/attachment,
writable selected synthetic profile, hidden outer env, self-executable helper,
normal shell subprocesses and real parent SIGKILL cleanup still pass.
The four compiled Cuan sandbox tests also pass in this container, but those Go
tests do not yet attach the new filter themselves. They are not evidence of
application integration of the new filter.

Rejected approach: switching to another AppArmor profile after Bubblewrap exec
was blocked by no_new_privs. No no_new_privs guard was disabled. Early parser
conflicts were diagnosed without loading permissive profiles. Rejected candidate
profiles are not shipped.

Both before/after runs cleaned temporary profiles, daemons, mounts and image
caches. Native production491b2dd remains healthy and unchanged. No account data
or provider inference was used.

This supersedes any implication that canonical path-denial tests alone established
the experimental Docker policy's isolation. These are still diagnostic fixtures,
not supported installation configuration. Next: integrate the payload filter
into actual Cuan commands (including the pre-inference probe), test live syscall
denials plus CLI compatibility, then verify the full container lifecycle.
