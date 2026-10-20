## Verified Docker application lifecycle

2026-10-20: full image build and actual production/staging Compose lifecycle passed
in 466.4 seconds. Source commit and exact input SHA256 values are in
[result.json](result.json). The tested Dockerfile, Compose files and test script
match the committed inputs; unrelated extension work was excluded.

- Frontend TypeScript/Vite and all three Go binaries build from source.
- App runs as UID1000 with writable persistent `/app/data` and `/home/cuan`.
- Fresh setup requires an owner and creates a neutral Default project.
- Owner, authenticated session and sentinels survive app recreation and full
  app/database down/up without deleting volumes.
- Staging has separate storage; revisiting production retains its original data.
- Real Compose rendering also verifies default/custom host ports, internal database
  hostname/port, proxy domain, missing-secret rejection and persistent storage.
- Seven targeted installer tests pass. No AI provider was called.

## Reproduce

Run `python3 scripts/test-compose-config.py --compose /path/to/docker-compose`
for configuration checks without a daemon. The lifecycle test requires an empty,
**disposable** Docker daemon labelled `cuan.install-audit=true`:

```sh
DOCKER_HOST=unix:///path/to/disposable.sock DOCKER_BUILDKIT=0 \
  python3 scripts/test-docker-install.py \
  --docker /path/to/docker --compose /path/to/docker-compose
```

The test deletes its fixture volumes. Never use a normal application daemon.
It deliberately refuses existing containers/volumes or a missing audit label.
Docker Engine28.5.2 and Compose5.6.0 were used. A transient systemd unit limited
all daemon/build/container processes to640MiB,75% CPU,512 tasks and no swap;
a real container cgroup probe confirmed inheritance. Serial build arguments were
Go parallelism1, Go memory target256MiB and Node heap384MiB. Memory targets are
runtime hints; use an external cgroup limit when a hard bound is required.
A watchdog aborts below1280MiB free disk or224MiB host available memory.

The daemon used private storage/socket, disabled iptables/default-bridge changes,
and a short private exec path (Unix socket paths must stay below104 bytes).
Initial probe failures exposed an invalid cgroup parent layout and an overlong
socket path; both were corrected before this successful run. One failed probe
container also demonstrated that the lifecycle script correctly rejects a
nonempty daemon; the successful test used a separate empty storage root.

## Cleanup and production checks

Before testing,91 upload copies were checksum-verified against retained permanent
releases and had no process references. Removing those uploads reclaimed
3,287,269,376 bytes. All permanent releases, rollback, backups and workspaces stayed.
After testing, daemon units were terminal, no dockerd/containerd processes remained,
and private image/build caches were removed. A leftover private network namespace
mount was unmounted before its directory was removed. Free disk ended at6.05GiB.
Native production remained active and its final health request succeeded.

## Limits

This proves the **application/database container lifecycle**, not AI execution.
The stock image still does not bundle a supported AGY sandbox/runtime; native Linux
is the verified AI execution path. No provider inference, external credentials,
public TLS, arbitrary external-effect recovery or native provider-session resume
was tested. The full product goal stays open while turnkey container AI execution
is unresolved. Dependency audit and bundle-size warnings from the build are
separate follow-up work; this run does not claim they were resolved.

## Default Docker sandbox policy probe

A follow-up actual container probe installed Alpine Bubblewrap in a disposable
`alpine:3.20` container, then ran the native sandbox primitives as a non-root user
with default Docker security policy. The container had no added capabilities,
custom security options or privileged mode. Network host mode was used for
package installation; it did not change the tested namespace security policy.
Bubblewrap exited1 before starting the shell: namespace creation was denied.
The same host passes the native service-user sandbox check. This establishes a
container policy constraint on this host, not a universal claim about every
Docker installation or a successful AI runtime. Raw result: [sandbox-policy.json](sandbox-policy.json).

The unit and daemon were stopped, the container was removed, its private network
namespace mount unmounted, and private daemon caches removed. Production health
passed;6.04GiB remained free. No host policy/sysctl or production service changed.
Installing additional packages alone is therefore insufficient to close the
Docker AI gap. The release needs an explicit choice between a native execution
path and designing/verifying a separate supported container execution boundary.
Neither unrestricted fallback nor privileged containers were used.

## Constrained syscall feasibility follow-up

A diagnostic trace narrowed the first denial to Bubblewrap's exact `clone` flags
(`CLONE_NEWNS|CLONE_NEWUTS|CLONE_NEWIPC|CLONE_NEWUSER|CLONE_NEWPID|SIGCHLD`).
A disposable per-container seccomp profile based on upstream Moby defaults was
extended for the relevant unshare mask and that exact clone combination. Clone
then succeeded, but mount propagation failed with EPERM. Allowing the mount,
umount2 and pivot_root syscalls in this experimental container changed the denial
to EACCES. The final trace is in [sandbox-seccomp-probe.json](sandbox-seccomp-probe.json).
This rules out a seccomp-only adjustment as sufficient on this host. A further
confinement layer is implicated; no kernel audit event was available to attribute
it conclusively to a particular AppArmor rule. No host AppArmor profile or sysctl
was changed, no capabilities added, and no privileged/unconfined container used.

These rules were temporary diagnostics, not a shipped or security-reviewed
runtime profile. A successful startup would still need full boundary, persistence
and AI execution verification. A supported specialized policy or separate runtime
remains a design option; the data does not establish that a separate service is
the only possible solution. The temporary profile, daemon caches and namespace
mount were removed after the unit stopped; final production health passed.

## Scoped AppArmor feasibility follow-up

An experimental parent/child AppArmor profile, derived from pinned Moby profiles
commit4b8214fc4971e06172938a1f1554ccff6647ee80, passed offline parser validation.
The parent retains `deny mount`; `/usr/bin/bwrap` transitions to a child profile
that allows mount and user namespaces while retaining upstream proc/sys deny
rules. The exact [experimental profile](experimental-apparmor.txt) and
[upstream license](MOBY-LICENSE.txt) are evidence only, not supported configuration.

With that temporary named profile and the prior experimental seccomp rules,
namespace creation and read-only system bind mounts succeeded. The next denial
was mounting a new private procfs: `mount("proc", "/newroot/proc", ...) = EPERM`.
The default Docker system-path protections remained in place. The result is
consistent with the kernel's restrictions on mounting procfs from a less
privileged namespace when the outer procfs has masked paths; it does not prove
the exact cause independently of those protections. No host procfs was bound into
the sandbox, no global sysctl changed, no extra capability/privileged mode used,
and no system-path protections disabled to make the check pass.

This confirms that a narrowly attached AppArmor profile can address the earlier
mount denial on this host, while private procfs remains a separate blocker.
The probe failed before executing its terminal helper and is not AI readiness
evidence. Raw trace: [sandbox-apparmor-probe.json](sandbox-apparmor-probe.json).
The temporary profile was unloaded and its absence verified. The daemon unit,
private namespace mount and caches were cleaned; production health passed.
