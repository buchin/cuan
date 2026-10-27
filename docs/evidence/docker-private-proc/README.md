## Private procfs feasibility with replacement path policy

2026-10-20: a disposable Alpine container passed the sandbox boundary and parent-loss
fixture with a private read-only procfs, a writable selected profile, a read-only
project and attachment, and no access to an unrelated fixture file. The terminal
helper ran through `/proc/self/exe`; the process exited0. The exact result, policy
hashes and tested worker are retained alongside this file.

This is experimental feasibility evidence, not a supported Docker AI installation.
No production container, application configuration or provider account was changed.
The production native application stayed0da59d4 and its final health check passed.

## Policy being tested

The default Docker masked/readonly path mounts prevented nested private procfs.
For this test only, `systempaths=unconfined` removed those OCI path mounts and an
explicit AppArmor policy replaced their access restrictions. This option alone
would not preserve those restrictions and is not a supported configuration.

The AppArmor parent denies mount and user-namespace creation. Only execution of
root-owned `/usr/bin/bwrap` transitions to a child allowed to build the sandbox.
Both profiles retain upstream proc/sys restrictions and add deny rules for all
masked paths and readonly paths listed in Moby28.5.2 `oci/defaults.go`, including
CPU thermal-throttle paths. Eleven present kernel paths were actually opened by
the fixture and rejected with EACCES; absent paths and all possible alias/bypass
cases were not dynamically established by this run. A direct namespace syscall
from the ordinary application profile was also rejected with EACCES.

The seccomp base matches pinned Moby profiles commit
4b8214fc4971e06172938a1f1554ccff6647ee80. Experimental additions allow the exact
Bubblewrap clone flags, the relevant unshare flags, and mount/umount2/pivot_root.
There are no added container capabilities, privileged mode, unconfined AppArmor
or unconfined seccomp. No global kernel sysctl or production profile was changed.
The profiles derive from Moby; its [Apache license](../docker-install/MOBY-LICENSE.txt)
applies. See experimental-apparmor.txt and experimental-seccomp.json.

## Process and filesystem checks

The accepted fixture launches Bubblewrap with a persistent Python parent and a
minimal environment, matching Cuan's persistent `exec.Command` parent and
`AntigravityProfileEnvironment` sanitization. The outer container's synthetic
secret is not inherited or visible through private procfs. The original file is
not mounted; the project cannot be changed or extended; the selected profile can
be updated and its change is verified from outside the sandbox.

The `--die-with-parent` guard is retained. A second child writes a heartbeat while
its Python parent is live. After SIGKILL of that owned parent, a scan of the
container's process metadata confirms that all sentinel descendants disappear.
This does not establish task checkpoint recovery or account/model routing in a
real Cuan image; those remain integration requirements.

Two initial fixture issues were distinguished from passing evidence: BusyBox's
`/proc/self/exe` invocation selected an unavailable applet, so the shell fixture
uses Dash. Direct `docker exec` launch intermittently returned137 despite printing
the boundary markers; a persistent parent eliminated that result, and parent-loss
cleanup was separately verified rather than dropping the guard. A temporary
no-parent-death diagnostic and a traced diagnostic were not accepted as readiness
proof. The accepted normal run has exit0 without tracing and with the guard.

## Resource ownership and cleanup

The exact tested worker in probe-worker.txt is an inspection artifact, not a
standalone installer. It requires the dedicated static Docker28.5.2 tooling and
private root used by the prior docker-install audit, a delegated transient systemd
unit with256MiB memory/no swap,75% CPU and512 tasks, and the two candidate policy
files. Container memory is128MiB. It must not be pointed at a normal daemon.
The named profile is added only while the dedicated test daemon/container runs.

The authoritative unit ended with MainPID0; no dockerd/containerd remained.
The named parent/child profiles were unloaded and checked absent. The private
network-namespace mount and Docker image/build caches were removed;5.25GiB remained
free. Source/policy SHA256 matches were checked before writing these artifacts.

## Remaining acceptance work

- Review the replacement path policy, including alias/bypass and full protected
  path coverage, before shipping a profile or Compose override.
- Verify the official musl AGY CLI and its helpers inside the actual Cuan image.
- Integrate repeatable Linux-host prerequisite setup, fail-closed readiness,
  supported-host detection, persistent account/workspace storage and rollback.
- Prove the existing approval/execution/interruption/reconstruction/no-duplicate
  contracts through that actual container runtime. No live provider call was made.

The native path and Docker app/database lifecycle retain their earlier evidence.
No requirement has been removed or the full goal marked complete by this probe.
