#!/usr/bin/env bash
# Native SSH release: versioned payload, checksums, atomic switch, health rollback.
set -euo pipefail

approved_branch=main
frontend_only=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --branch)
      approved_branch="$2"
      shift 2
      ;;
    --frontend-only|--assets-only)
      frontend_only=1
      shift
      ;;
    --help)
      echo 'Usage: scripts/deploy.sh [--branch APPROVED_BRANCH] [--frontend-only] (clean tree; default main). Requires DOJO_TERMAPI_VPS_* env, Go, npm, sshpass. Schema25 must be explicitly prepared after private backup; activation never migrates.'
      exit 0
      ;;
    *)
      echo "Invalid argument: $1; use --help." >&2
      exit 1
      ;;
  esac
done

cd "$(dirname "$0")/.."
[[ "$(git branch --show-current)" == "$approved_branch" ]] || { echo "Release must run from the explicitly approved branch ($approved_branch)." >&2; exit 1; }
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

export SSHPASS="$DOJO_TERMAPI_VPS_PASSWORD"
release_host="$DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP"

if [[ "$frontend_only" == 1 ]]; then
  echo "==> Building frontend assets for $release_sha..."
  npm run build --prefix web
  echo "==> Syncing static assets to production ($release_host)..."
  sshpass -e ssh "$release_host" "mkdir -p .local/share/cuan-releases/$release_sha/web"
  sshpass -e scp -q -r web/dist "$release_host:.local/share/cuan-releases/$release_sha/web/"
  sshpass -e ssh "$release_host" "sudo -n rsync -a --delete /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-releases/$release_sha/web/dist/ /opt/cuan/current/web/dist/"
  unset SSHPASS
  echo "==> Verifying production health..."
  curl -sSf https://cuan.dojo.cc/healthz >/dev/null
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

sshpass -e ssh "$release_host" "sudo -n mkdir -p /opt/cuan/releases/$release_sha"
# Use native remote rsync; sandbox rsync wrappers may alter SSH authentication.
sshpass -e ssh "$release_host" 'mkdir -p .local/share/cuan-releases .local/share/cuan-s25'
sshpass -e scp -q -r "$release_dir" "$release_host:.local/share/cuan-releases/"
sshpass -e ssh "$release_host" "sudo -n rsync -a --delete /home/$DOJO_TERMAPI_VPS_USERNAME/.local/share/cuan-releases/$release_sha/ /opt/cuan/releases/$release_sha/"
sshpass -e scp -q scripts/activate-release.py "$release_host:.local/share/cuan-s25/activate-release.py"
sshpass -e ssh "$release_host" "sudo -n python3 .local/share/cuan-s25/activate-release.py --release $release_sha"
unset SSHPASS

echo "==> Verifying production health & commit SHA ($release_sha)..."
verified_sha=""
for attempt in {1..20}; do
  health_json="$(curl -sSf https://cuan.dojo.cc/healthz 2>/dev/null || true)"
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
