#!/usr/bin/env bash
# Native SSH release: versioned payload, checksums, atomic switch, health rollback.
set -euo pipefail

approved_branch=""
explicit_branch=0
frontend_only=0
force_local=0
native_sync=0
devnull_broadcast=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --branch)
      approved_branch="$2"
      explicit_branch=1
      shift 2
      ;;
    --frontend-only|--assets-only)
      frontend_only=1
      shift
      ;;
    --local)
      force_local=1
      shift
      ;;
    --native-sync)
      native_sync=1
      shift
      ;;
    --devnull)
      devnull_broadcast=1
      shift
      ;;
    --help)
      echo 'Usage: scripts/deploy.sh [--branch APPROVED_BRANCH] [--frontend-only] [--local] [--native-sync] [--devnull] (clean tree; default dev). Requires DOJO_TERMAPI_VPS_* env when running remotely; auto-detects or uses --local when running directly on the production host.'
      exit 0
      ;;
    *)
      echo "Invalid argument: $1; use --help." >&2
      exit 1
      ;;
  esac
done

cd "$(dirname "$0")/.."

# Resolve approved deployment branch if not explicitly given:
# 1. Flag --branch <NAME> (handled above)
# 2. CUAN_DEPLOY_BRANCH environment variable
# 3. git config cuan.deployBranch
# 4. Fallback default: dev
if [[ "$explicit_branch" == 0 ]]; then
  if [[ -n "${CUAN_DEPLOY_BRANCH:-}" ]]; then
    approved_branch="$CUAN_DEPLOY_BRANCH"
  elif git_deploy_cfg="$(git config --get cuan.deployBranch 2>/dev/null)" && [[ -n "$git_deploy_cfg" ]]; then
    approved_branch="$git_deploy_cfg"
  else
    approved_branch="dev"
  fi
fi

current_branch="$(git branch --show-current 2>/dev/null || true)"
if [[ -z "$current_branch" ]]; then
  echo "ERROR: Could not determine current branch (detached HEAD?)." >&2
  exit 1
fi

# Guardrail: Never deploy directly from a temporary task/feature branch
if [[ "$current_branch" =~ ^(cuan/task-|task/|feat/|fix/|chore/|spike/) ]]; then
  echo "❌ ERROR: Direct deployment from working branch '$current_branch' is prohibited." >&2
  echo "Guardrail: You must merge and push your changes to the deployment branch ('$approved_branch') before deploying." >&2
  echo "Steps to release safely:" >&2
  echo "  1. Complete and test all changes in '$current_branch'." >&2
  echo "  2. Merge '$current_branch' into '$approved_branch' (via PR or git merge)." >&2
  echo "  3. Switch to '$approved_branch' and run ./scripts/deploy.sh." >&2
  exit 1
fi

if [[ "$current_branch" != "$approved_branch" ]]; then
  echo "❌ ERROR: Release must run from the approved deployment branch ('$approved_branch'), but currently on '$current_branch'." >&2
  echo "Switch to '$approved_branch' or pass --branch '$current_branch' if explicitly approved." >&2
  exit 1
fi

[[ -z "$(git status --porcelain)" ]] || { echo 'Working tree must be clean.' >&2; exit 1; }

local_sha="$(git rev-parse HEAD)"
if [[ "$native_sync" == 1 ]]; then
  CUAN_NATIVE_RELEASE_SYNC_SHA="$local_sha"
  CUAN_NATIVE_RELEASE_SYNC_BRANCH="$approved_branch"
fi

if [[ -n "${CUAN_NATIVE_RELEASE_SYNC_SHA:-}${CUAN_NATIVE_RELEASE_SYNC_BRANCH:-}" ]]; then
  if [[ ! "${CUAN_NATIVE_RELEASE_SYNC_SHA:-}" =~ ^[a-f0-9]{40}$ || "${CUAN_NATIVE_RELEASE_SYNC_SHA:-}" != "$local_sha" || "${CUAN_NATIVE_RELEASE_SYNC_BRANCH:-}" != "$approved_branch" ]]; then
    echo "ERROR: Native prepared source does not match its observed release branch." >&2
    exit 1
  fi
  echo "==> Using native saved-connection branch observation for $local_sha..."
else
  echo "==> Verifying branch sync with origin/$approved_branch..."
  git fetch origin "$approved_branch" --quiet
  remote_sha="$(git rev-parse "origin/$approved_branch")"
  if [[ "$local_sha" != "$remote_sha" ]]; then
    echo "ERROR: Local HEAD ($local_sha) is not synchronized with origin/$approved_branch ($remote_sha)." >&2
    echo "Pull and push latest changes before releasing, or pass --native-sync for local native releases." >&2
    exit 1
  fi
fi

echo '::cuan-release-stage::checks'
echo "==> Checking release activation safety..."
python3 scripts/test-activate-release.py
python3 scripts/test-release-ancestry.py
python3 scripts/test-authorized-release.py
python3 scripts/test-cuan-disk-guard.py
if [[ "$(uname -s)" != "Darwin" ]]; then
  python3 scripts/test-prune-release-artifacts.py
  python3 scripts/test-prune-provider-go-cache.py
  python3 scripts/test-prune-verification-dependencies.py
  python3 scripts/test-compact-verification-source.py
  python3 scripts/test-prune-native-release-caches.py
  python3 scripts/test-prune-runtime-build-cache.py
  python3 scripts/test-runtime-cache-maintenance.py
fi

echo '::cuan-release-stage::dependencies'
echo "==> Installing locked frontend dependencies in this release checkout..."
export NODE_OPTIONS="${NODE_OPTIONS:---max-old-space-size=512}"
npm ci --prefix web --no-audit --no-fund
# Frontend gates inspect generated chunks; build this exact source once before
# testing so clean checkouts never depend on assets from an older release.
echo "==> Building current frontend assets before verification..."
npm run build --prefix web

echo '::cuan-release-stage::tests'
echo "==> Running frontend test gate..."
npm test --prefix web

release_sha="$local_sha"

is_local=0
if [[ "$force_local" == 1 ]]; then
  is_local=1
elif [[ -d "/opt/cuan/releases" && -z "${DOJO_TERMAPI_VPS_PASSWORD:-}" ]]; then
  is_local=1
fi

if [[ "$is_local" == 0 ]]; then
  export SSHPASS="${DOJO_TERMAPI_VPS_PASSWORD:-}"
  release_host="${DOJO_TERMAPI_VPS_USERNAME:-}@${DOJO_TERMAPI_VPS_PUBLIC_IP:-}"
  if [[ -z "${DOJO_TERMAPI_VPS_PASSWORD:-}" || -z "${DOJO_TERMAPI_VPS_USERNAME:-}" || -z "${DOJO_TERMAPI_VPS_PUBLIC_IP:-}" ]]; then
    echo "ERROR: Remote release requires DOJO_TERMAPI_VPS_* environment variables." >&2
    echo "If running directly on the production host, pass --local." >&2
    exit 1
  fi
fi

if [[ "$frontend_only" == 1 ]]; then
  if [[ "$is_local" == 1 ]]; then
    echo "==> Syncing static assets locally to /opt/cuan/current/web/dist/..."
    mkdir -p "$HOME/.local/share/cuan-releases/$release_sha/web"
    cp -aL web/dist "$HOME/.local/share/cuan-releases/$release_sha/web/"
    sudo -n rsync -a --delete web/dist/ /opt/cuan/current/web/dist/
  else
    echo "==> Syncing static assets to production ($release_host)..."
    sshpass -e ssh "$release_host" "mkdir -p .local/share/cuan-releases/$release_sha/web"
    sshpass -e scp -q -r web/dist "$release_host:.local/share/cuan-releases/$release_sha/web/"
    sshpass -e ssh "$release_host" "sudo -n rsync -a --delete /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-releases/$release_sha/web/dist/ /opt/cuan/current/web/dist/"
    unset SSHPASS
  fi
  echo "==> Verifying production health..."
  if [[ "$is_local" == 0 ]]; then
    export SSHPASS="${DOJO_TERMAPI_VPS_PASSWORD:-}"
    sshpass -e ssh "$release_host" "curl -s http://127.0.0.1:8080/healthz" >/dev/null
    unset SSHPASS
  else
    curl -sSf http://127.0.0.1:8080/healthz >/dev/null 2>&1 || curl -sSf "${CUAN_PUBLICATION_HEALTH_URL:-https://app.cuan.work/healthz}" >/dev/null
  fi
  echo "Frontend assets updated to $release_sha. Zero downtime; active runner sessions uninterrupted."
  exit 0
fi

release_dir="$HOME/.local/share/cuan-releases/$release_sha"
release_repository="$(python3 scripts/release-repository.py)"
release_linker_flags="-X github.com/buchin/cuan/internal/config.GitCommit=$release_sha -X github.com/buchin/cuan/internal/config.SourceRepository=$release_repository"
mkdir -p "$release_dir/bin" "$release_dir/web"
echo '::cuan-release-stage::build'
echo "==> Compiling Go backend binaries with shared cache reuse for $release_sha..."
# Each concurrent go build creates its own temporary dependency graph. On the
# production host that multiplied peak disk use and exhausted the filesystem.
# Serial builds reuse Go's completed cache entries and stop at the first failure.
for component in server migrate runner; do
  if CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$release_linker_flags" -o "$release_dir/bin/cuan-$component" "./cmd/$component"; then
    :
  else
    build_status=$?
    echo "ERROR: cuan-$component build failed with exit code $build_status." >&2
    exit "$build_status"
  fi
done
ln -sf cuan-server "$release_dir/bin/cuan"
echo "✓ Backend binaries compiled successfully."
cp -aL web/dist "$release_dir/web/"
cp -a migrations "$release_dir/"
cp README.md "$release_dir/"
# Optional runtime installers travel with the verified release; they are not
# executed during ordinary deployment or bundled into provider profiles.
mkdir -p "$release_dir/runtime-tools"
cp scripts/setup-chat-browser-runtime.py "$release_dir/runtime-tools/"
# Cryptographic commit-parent graph for activation, independent of a stale host
# repository cache. Only commit objects are packed; no source blobs or secrets.
git --no-replace-objects rev-list HEAD | git --no-replace-objects pack-objects --stdout > "$release_dir/source-commits.pack"
python3 - "$release_dir" "$release_sha" <<'PY'
import hashlib,json,pathlib,sys
root=pathlib.Path(sys.argv[1])
(root/'REVISION').write_text(sys.argv[2].strip()+'\n')
checks={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in root.rglob('*') if p.is_file() and not p.is_symlink() and p.name!='release.json'}
(root/'release.json').write_text(json.dumps({'sha':sys.argv[2],'sha256':checks,'features':{'project_workspace_revisions':1,'task_execution_worktrees':1,'inbox_worker':1}})+'\n')
PY

maintenance_tool_names=()
while IFS= read -r line || [[ -n "$line" ]]; do
  [[ -n "$line" ]] && maintenance_tool_names+=("$line")
done < <(python3 scripts/install-runtime-cache-maintenance.py --print-tools)
[[ "${#maintenance_tool_names[@]}" -gt 0 ]] || { echo 'Maintenance payload manifest unavailable' >&2; exit 1; }
maintenance_tool_paths=()
for maintenance_tool_name in "${maintenance_tool_names[@]}"; do
  [[ -f "scripts/$maintenance_tool_name" ]] || { echo "Maintenance payload is missing $maintenance_tool_name" >&2; exit 1; }
  maintenance_tool_paths+=("scripts/$maintenance_tool_name")
done

if [[ "$is_local" == 1 ]]; then
  sudo -n mkdir -p /opt/cuan/maintenance
  sudo -n install -m 755 scripts/prune-release-artifacts.py "${maintenance_tool_paths[@]}" /opt/cuan/maintenance/
  # Native release HOME is an isolated build home, not the host service profile.
  maintenance_service_home="$(getent passwd "${DOJO_TERMAPI_VPS_USERNAME:-$(id -un)}" | cut -d: -f6)"
  [[ -n "$maintenance_service_home" ]] || { echo 'Service home unavailable for maintenance installation' >&2; exit 1; }
  sudo -n python3 scripts/install-runtime-cache-maintenance.py --service-home "$maintenance_service_home"
  sudo -n install -m 755 scripts/cuan-disk-guard.sh /usr/local/bin/cuan-disk-guard.sh
  if [[ -f /etc/cron.hourly/cuan-disk-guard ]]; then
    sudo -n install -m 755 scripts/cuan-disk-guard.sh /etc/cron.hourly/cuan-disk-guard
  fi
  if [[ -d "$HOME/.local/share/cuan/accounts" ]]; then
    python3 scripts/prune-provider-go-cache.py --accounts "$HOME/.local/share/cuan/accounts" --apply
  fi
  if [[ -L /opt/cuan/current && -L /opt/cuan/previous ]]; then
    sudo -n python3 scripts/prune-release-artifacts.py --staging "$HOME/.local/share/cuan-releases" --apply
  fi
  echo "==> Deploying release payload locally to /opt/cuan/releases/$release_sha..."
  sudo -n mkdir -p "/opt/cuan/releases/$release_sha"
  sudo -n chown -R "$(id -u):$(id -g)" /opt/cuan/releases 2>/dev/null || true
  if [[ "$release_dir" != "$HOME/.local/share/cuan-releases/"* ]]; then
    mkdir -p "$HOME/.local/share/cuan-releases"
    cp -a "$release_dir" "$HOME/.local/share/cuan-releases/"
  fi
  sudo -n rsync -a --delete "$release_dir/" "/opt/cuan/releases/$release_sha/"
  sudo -n CUAN_DEPLOY_ALLOW_RESUME="${CUAN_DEPLOY_ALLOW_RESUME:-1}" CUAN_DEPLOY_EXCLUDE_TASK="${CUAN_DEPLOY_EXCLUDE_TASK:-${TASK_ID:-}}" CUAN_DEPLOY_EXCLUDE_ATTEMPT="${CUAN_DEPLOY_EXCLUDE_ATTEMPT:-}" CUAN_TASK_CAPABILITY_FILE="${CUAN_TASK_CAPABILITY_FILE:-}" python3 scripts/activate-release.py --release "$release_sha"
else
  sshpass -e ssh "$release_host" "sudo -n mkdir -p /opt/cuan/releases/$release_sha && sudo -n chown -R $DOJO_TERMAPI_VPS_USERNAME:$DOJO_TERMAPI_VPS_USERNAME /opt/cuan/releases"
  sshpass -e ssh "$release_host" 'mkdir -p .local/share/cuan-releases .local/share/cuan-s25'
  sshpass -e scp -q scripts/prune-release-artifacts.py "${maintenance_tool_paths[@]}" scripts/install-runtime-cache-maintenance.py scripts/cuan-disk-guard.sh "$release_host:.local/share/cuan-s25/"
  sshpass -e ssh "$release_host" "sudo -n mkdir -p /opt/cuan/maintenance && sudo -n install -m 755 /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-s25/prune-release-artifacts.py /opt/cuan/maintenance/prune-release-artifacts.py && if [ -L /opt/cuan/current ] && [ -L /opt/cuan/previous ]; then sudo -n python3 /opt/cuan/maintenance/prune-release-artifacts.py --staging /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-releases --apply; fi"
  sshpass -e ssh "$release_host" "sudo -n install -m 755 /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-s25/prune-verification-dependencies.py /opt/cuan/maintenance/prune-verification-dependencies.py"
  sshpass -e ssh "$release_host" "sudo -n install -m 755 /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-s25/prune-native-release-caches.py /opt/cuan/maintenance/prune-native-release-caches.py"
  sshpass -e ssh "$release_host" "sudo -n install -m 755 /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-s25/prune-runtime-build-cache.py /opt/cuan/maintenance/prune-runtime-build-cache.py"
  sshpass -e ssh "$release_host" "sudo -n install -m 755 /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-s25/prune-provider-go-cache.py /opt/cuan/maintenance/prune-provider-go-cache.py && if [ -d /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan/accounts ]; then python3 /opt/cuan/maintenance/prune-provider-go-cache.py --accounts /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan/accounts --apply; fi"
  sshpass -e ssh "$release_host" "sudo -n install -m 755 /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-s25/cuan-disk-guard.sh /usr/local/bin/cuan-disk-guard.sh && if [ -f /etc/cron.hourly/cuan-disk-guard ]; then sudo -n install -m 755 /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-s25/cuan-disk-guard.sh /etc/cron.hourly/cuan-disk-guard; fi"
  sshpass -e ssh "$release_host" "sudo -n python3 /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-s25/install-runtime-cache-maintenance.py --service-home /home/$DOJO_TERMAPI_VPS_USERNAME"
  echo '::cuan-release-stage::transfer'
  echo "==> Transferring verified payload (compressed, resumable)..."
  sshpass -e rsync -az --partial --append-verify -e "sshpass -p '$DOJO_TERMAPI_VPS_PASSWORD' ssh -o StrictHostKeyChecking=no" "$release_dir/" "$release_host:.local/share/cuan-releases/$release_sha/"
  sshpass -e ssh "$release_host" "sudo -n rsync -a --delete /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-releases/$release_sha/ /opt/cuan/releases/$release_sha/"
  sshpass -e scp -q scripts/activate-release.py scripts/verify-authorized-release.py "$release_host:.local/share/cuan-s25/"
  sshpass -e ssh "$release_host" "sudo -n CUAN_DEPLOY_ALLOW_RESUME=\"${CUAN_DEPLOY_ALLOW_RESUME:-1}\" CUAN_DEPLOY_EXCLUDE_TASK=\"${CUAN_DEPLOY_EXCLUDE_TASK:-${TASK_ID:-}}\" CUAN_DEPLOY_EXCLUDE_ATTEMPT=\"${CUAN_DEPLOY_EXCLUDE_ATTEMPT:-}\" CUAN_TASK_CAPABILITY_FILE=\"${CUAN_TASK_CAPABILITY_FILE:-}\" python3 .local/share/cuan-s25/activate-release.py --release $release_sha"
  unset SSHPASS
fi

echo '::cuan-release-stage::health'
echo "==> Verifying production health & commit SHA ($release_sha)..."
verified_sha=""
for attempt in {1..20}; do
  if [[ "$is_local" == 0 ]]; then
    export SSHPASS="${DOJO_TERMAPI_VPS_PASSWORD:-}"
    health_json="$(sshpass -e ssh "$release_host" "curl -s http://127.0.0.1:8080/healthz" 2>/dev/null || true)"
    unset SSHPASS
  else
    health_json="$(curl -sSf http://127.0.0.1:8080/healthz 2>/dev/null || curl -sSf "${CUAN_PUBLICATION_HEALTH_URL:-https://app.cuan.work/healthz}" 2>/dev/null || true)"
  fi
  if [[ -n "$health_json" ]]; then
    live_sha="$(python3 -c "import sys, json; print(json.loads('''$health_json''').get('commit_sha', ''))" 2>/dev/null || true)"
    live_status="$(python3 -c "import sys, json; print(json.loads('''$health_json''').get('status', ''))" 2>/dev/null || true)"
    if [[ "$live_status" == "healthy" ]]; then
      if [[ "$live_sha" == "$release_sha" ]]; then
        verified_sha="$live_sha"
        echo "✓ Production /healthz verified healthy matching commit SHA: $verified_sha"
        break
      else
        echo "Waiting for service to report new commit SHA (current: '$live_sha', target: '$release_sha')... attempt $attempt"
      fi
    fi
  fi
  sleep 1
done

if [[ -z "$verified_sha" ]]; then
  echo "ERROR: Production deployment failed verification. /healthz did not report expected commit SHA $release_sha." >&2
  exit 1
fi

echo "==> Auto-undrain deploy hook: Ensuring /tmp/cuan-drain cleanup & checking runner service..."
has_active_workers() {
  local target_host="${1:-}"
  local py_check="import json, os, glob, sys; sys.exit(0 if any(os.kill(json.loads(open(w).read()).get('pid', 0), 0) is None for w in glob.glob('/tmp/cuan-workers/att_*.json') if os.path.isfile(w)) else 1)"
  if [[ -n "$target_host" ]]; then
    sshpass -e ssh "$target_host" "python3 -c \"$py_check\"" 2>/dev/null
  else
    python3 -c "$py_check" 2>/dev/null
  fi
}

if [[ "$is_local" == 0 ]]; then
  export SSHPASS="${DOJO_TERMAPI_VPS_PASSWORD:-}"
  sshpass -e ssh "$release_host" "sudo -n rm -f /tmp/cuan-drain 2>/dev/null || true"
  if has_active_workers "$release_host"; then
    echo "==> Notice: Active detached worker detected in /tmp/cuan-workers; deferring cuan-runner restart to avoid interrupting in-flight tasks."
    echo "✓ Auto-undrain completed (runner restart deferred while task is active)."
  else
    sshpass -e ssh "$release_host" "sudo -n systemctl restart cuan-runner.service 2>/dev/null || true"
    echo "✓ Auto-undrain & cuan-runner restart completed."
  fi
  unset SSHPASS
else
  sudo -n rm -f /tmp/cuan-drain 2>/dev/null || true
  if has_active_workers ""; then
    echo "==> Notice: Active detached worker detected in /tmp/cuan-workers; deferring cuan-runner restart to avoid interrupting in-flight tasks."
    echo "✓ Auto-undrain completed (runner restart deferred while task is active)."
  else
    sudo -n systemctl restart cuan-runner.service 2>/dev/null || true
    echo "✓ Auto-undrain & cuan-runner restart completed."
  fi
fi

echo "Released $release_sha. Verify the public UI, OAuth, Inbox reply and retained data before marking done."

if [[ "$devnull_broadcast" == 1 || "${DEVNULL_BROADCAST:-false}" == "true" ]]; then
  echo "==> Broadcasting release announcement to dev-null.sh..."
  commit_msg="$(git log -1 --pretty=%s "$release_sha" 2>/dev/null || echo "New production update")"
  "$(dirname "$0")/post-to-devnull.sh" "Shipped Cuan release ${release_sha:0:7}: $commit_msg 🚀 https://app.cuan.work" || echo "Warning: dev-null broadcast failed; continuing." >&2
fi

