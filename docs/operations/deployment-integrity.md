# Cuan Deployment Integrity & Anti-Regression Standard

## 1. Overview

Cuan employs an atomic, versioned deployment system on native host environments (such as systemd services on Linux VPS). To prevent deployment regressions, stale process execution, or pseudo-releases where symlinks point to new directories while stale processes run in memory, the following integrity controls are enforced.

---

## 2. Commit SHA Health Verification Contract

The `/healthz` liveness endpoint exposes the active running binary metadata:

```json
{
  "status": "healthy",
  "app": "cuan",
  "version": "0.9.0",
  "commit_sha": "616f3840dc2dcdbbe962d639937da250b1d694c4",
  "timestamp": "2026-10-25T09:30:00Z"
}
```

### Detection Hierarchy
The active binary resolves its `commit_sha` via:
1. Compile-time linker flags (`-ldflags "-X github.com/buchin/cuan/internal/config.GitCommit=<SHA>"`).
2. Environment variable override (`CUAN_COMMIT_SHA`).
3. Release manifest lookup (`release.json` in current or parent working directory, or `/opt/cuan/current/release.json`).

---

## 3. Strict `--no-restart` Guard

The deployment script `scripts/activate-release.py` supports `--no-restart` **strictly for static asset changes only** (e.g., CSS/JS bundles in `web/dist/`).

If the checksum of `bin/cuan-server` differs between the currently active release (`current/bin/cuan-server`) and the target release, `activate-release.py` will abort immediately with:
```
RuntimeError: Cannot use --no-restart when bin/cuan-server binary has changed; service restart is required
```

This prevents leaving an outdated Go server process running in memory with an outdated schema or business logic while the file tree has moved.

---

## 4. End-to-End Post-Deployment Verification Gate

`scripts/fural-deploy.sh` executes an end-to-end verification gate against `https://cuan.dojo.cc/healthz`:
- Must return `HTTP 200 OK` with `"status": "healthy"`.
- Must return `"commit_sha"` identical to the target `$release_sha`.
- If the running process reports a stale SHA, the deploy script waits and retries for service restart. If the SHA fails to match within the threshold, deployment halts with an error.

---

## 5. Rollback Behavior

If `systemctl restart cuan` fails or `/healthz` / `/readyz` fails health checks after symlink switch:
- `activate-release.py` automatically restores symlinks to the previously known healthy release (`previous`).
- Restarts and validates health of the restored release.
- Exits with a descriptive status code indicating rollback success or manual intervention requirement.
