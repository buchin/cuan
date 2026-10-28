#!/usr/bin/env bash
# Native SSH release: versioned payload, checksums, atomic switch, health rollback.
set -euo pipefail

approved_branch=""
explicit_branch=0
frontend_only=0
force_local=0

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
    --help)
      echo 'Usage: scripts/deploy.sh [--branch APPROVED_BRANCH] [--frontend-only] [--local] (clean tree; default dev). Requires DOJO_TERMAPI_VPS_* env when running remotely; auto-detects or uses --local when running directly on the production host.'
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

echo "==> Verifying branch sync with origin/$approved_branch..."
git fetch origin "$approved_branch" --quiet
local_sha="$(git rev-parse HEAD)"
remote_sha="$(git rev-parse "origin/$approved_branch")"
if [[ "$local_sha" != "$remote_sha" ]]; then
  echo "ERROR: Local HEAD ($local_sha) is not synchronized with origin/$approved_branch ($remote_sha)." >&2
  echo "Pull and push latest changes before releasing to avoid regressing production." >&2
  exit 1
fi

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
  echo "==> Building frontend assets for $release_sha..."
  npm run build --prefix web
  if [[ "$is_local" == 1 ]]; then
    echo "==> Syncing static assets locally to /opt/cuan/current/web/dist/..."
    mkdir -p "$HOME/.local/share/cuan-releases/$release_sha/web"
    cp -a web/dist "$HOME/.local/share/cuan-releases/$release_sha/web/"
    sudo -n rsync -a --delete web/dist/ /opt/cuan/current/web/dist/
  else
    echo "==> Syncing static assets to production ($release_host)..."
    sshpass -e ssh "$release_host" "mkdir -p .local/share/cuan-releases/$release_sha/web"
    sshpass -e scp -q -r web/dist "$release_host:.local/share/cuan-releases/$release_sha/web/"
    sshpass -e ssh "$release_host" "sudo -n rsync -a --delete /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-releases/$release_sha/web/dist/ /opt/cuan/current/web/dist/"
    unset SSHPASS
  fi
  echo "==> Verifying production health..."
  curl -sSf http://127.0.0.1:8080/healthz >/dev/null 2>&1 || curl -sSf https://cuan.dojo.cc/healthz >/dev/null
  echo "Frontend assets updated to $release_sha. Zero downtime; active runner sessions uninterrupted."
  exit 0
fi

release_dir="$HOME/.local/share/cuan-releases/$release_sha"
mkdir -p "$release_dir/bin" "$release_dir/web"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X github.com/buchin/cuan/internal/config.GitCommit=$release_sha" -o "$release_dir/bin/cuan-server" ./cmd/server
npm run build --prefix web
cp -a web/dist "$release_dir/web/"
cp -a migrations "$release_dir/"
cp README.md "$release_dir/"
python3 - "$release_dir" "$release_sha" <<'PY'
import hashlib,json,pathlib,sys
root=pathlib.Path(sys.argv[1])
checks={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in root.rglob('*') if p.is_file() and p.name!='release.json'}
(root/'release.json').write_text(json.dumps({'sha':sys.argv[2],'sha256':checks,'features':{'project_workspace_revisions':1,'task_execution_worktrees':1}})+'\n')
PY

if [[ "$is_local" == 1 ]]; then
  echo "==> Deploying release payload locally to /opt/cuan/releases/$release_sha..."
  sudo -n mkdir -p "/opt/cuan/releases/$release_sha"
  mkdir -p "$HOME/.local/share/cuan-releases"
  cp -a "$release_dir" "$HOME/.local/share/cuan-releases/"
  sudo -n rsync -a --delete "$release_dir/" "/opt/cuan/releases/$release_sha/"
  sudo -n CUAN_DEPLOY_EXCLUDE_TASK="${CUAN_DEPLOY_EXCLUDE_TASK:-${TASK_ID:-}}" CUAN_DEPLOY_EXCLUDE_ATTEMPT="${CUAN_DEPLOY_EXCLUDE_ATTEMPT:-}" CUAN_TASK_CAPABILITY_FILE="${CUAN_TASK_CAPABILITY_FILE:-}" python3 scripts/activate-release.py --release "$release_sha"
else
  sshpass -e ssh "$release_host" "sudo -n mkdir -p /opt/cuan/releases/$release_sha"
  # Use native remote rsync; sandbox rsync wrappers may alter SSH authentication.
  sshpass -e ssh "$release_host" 'mkdir -p .local/share/cuan-releases .local/share/cuan-s25'
  sshpass -e scp -q -r "$release_dir" "$release_host:.local/share/cuan-releases/"
  sshpass -e ssh "$release_host" "sudo -n rsync -a --delete /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-releases/$release_sha/ /opt/cuan/releases/$release_sha/"
  sshpass -e scp -q scripts/activate-release.py "$release_host:.local/share/cuan-s25/activate-release.py"
  sshpass -e ssh "$release_host" "sudo -n CUAN_DEPLOY_EXCLUDE_TASK=\"${CUAN_DEPLOY_EXCLUDE_TASK:-}\" CUAN_DEPLOY_EXCLUDE_ATTEMPT=\"${CUAN_DEPLOY_EXCLUDE_ATTEMPT:-}\" python3 .local/share/cuan-s25/activate-release.py --release $release_sha"
  unset SSHPASS
fi

echo "==> Verifying production health & commit SHA ($release_sha)..."
verified_sha=""
for attempt in {1..20}; do
  health_json="$(curl -sSf http://127.0.0.1:8080/healthz 2>/dev/null || curl -sSf https://cuan.dojo.cc/healthz 2>/dev/null || true)"
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

echo "Released $release_sha. Verify the public UI, OAuth, Inbox reply and retained data before marking done."
