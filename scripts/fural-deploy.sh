#!/usr/bin/env bash
# Native SSH release: versioned payload, checksums, atomic switch, health rollback.
set -euo pipefail
if [[ "${1:-}" == "--help" ]]; then
  echo 'Usage: scripts/fural-deploy.sh (from main, clean tree). Requires DOJO_TERMAPI_VPS_* env, Go, npm, sshpass, rsync.'
  exit 0
fi
cd "$(dirname "$0")/.."
[[ "$(git branch --show-current)" == main ]] || { echo 'Release must run from main.' >&2; exit 1; }
[[ -z "$(git status --porcelain)" ]] || { echo 'Working tree must be clean.' >&2; exit 1; }
release_sha="$(git rev-parse HEAD)"
release_dir="$HOME/.local/share/cuan-releases/$release_sha"
mkdir -p "$release_dir/bin" "$release_dir/web"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$release_dir/bin/cuan-server" ./cmd/server
npm run build --prefix web
cp -a web/dist "$release_dir/web/"
cp -a migrations "$release_dir/"
cp README.md "$release_dir/"
python3 - "$release_dir" "$release_sha" <<'PY'
import hashlib,json,pathlib,sys
root=pathlib.Path(sys.argv[1])
checks={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in root.rglob('*') if p.is_file() and p.name!='release.json'}
(root/'release.json').write_text(json.dumps({'sha':sys.argv[2],'sha256':checks})+'\n')
PY
export SSHPASS="$DOJO_TERMAPI_VPS_PASSWORD"
release_host="$DOJO_TERMAPI_VPS_USERNAME@$DOJO_TERMAPI_VPS_PUBLIC_IP"
sshpass -e ssh "$release_host" "sudo -n mkdir -p /opt/cuan/releases/$release_sha"
sshpass -e rsync -a --delete --rsync-path='sudo -n rsync' "$release_dir/" "$release_host:/opt/cuan/releases/$release_sha/"
sshpass -e scp -q scripts/activate-release.py "$release_host:.local/share/cuan-s25/activate-release.py"
sshpass -e ssh "$release_host" "sudo -n python3 .local/share/cuan-s25/activate-release.py --release $release_sha"
unset SSHPASS
echo "Released $release_sha. Verify the public UI, OAuth, Inbox reply and retained data before marking done."
