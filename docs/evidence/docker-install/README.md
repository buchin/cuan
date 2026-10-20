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
