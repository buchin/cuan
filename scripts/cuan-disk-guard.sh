#!/usr/bin/env bash
# Use the same conservative pruners as deployment, never broad cache deletion.
set -euo pipefail

install_dir="${CUAN_INSTALL_DIR:-/opt/cuan}"
maintenance_home="${CUAN_MAINTENANCE_HOME:-}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --base) install_dir="${2:?--base requires a directory}"; shift 2 ;;
    --home) maintenance_home="${2:?--home requires a directory}"; shift 2 ;;
    *) echo 'Usage: cuan-disk-guard.sh [--base INSTALL_DIR] [--home SERVICE_HOME]' >&2; exit 2 ;;
  esac
done
if [[ "$install_dir" != /* || ! -d "$install_dir" ]]; then
  echo 'Cuan installation directory is unavailable; no cleanup performed.' >&2
  exit 1
fi
if [[ -z "$maintenance_home" && -d "$install_dir/releases" ]]; then
  # Native releases belong to the service user. Root-owned installations must
  # provide --home explicitly; do not guess another account directory.
  service_uid="$(stat -c %u "$install_dir/releases")"
  if [[ "$service_uid" != 0 ]]; then
    maintenance_home="$(getent passwd "$service_uid" | cut -d: -f6)"
  fi
fi
if [[ -n "$maintenance_home" && ( "$maintenance_home" != /* || ! -d "$maintenance_home" ) ]]; then
  echo 'Service home is unavailable; no cleanup performed.' >&2
  exit 1
fi
release_args=(--base "$install_dir" --apply)
if [[ -n "$maintenance_home" && -d "$maintenance_home/.local/share/cuan-releases" ]]; then
  release_args+=(--staging "$maintenance_home/.local/share/cuan-releases")
fi
python3 "$install_dir/maintenance/prune-release-artifacts.py" "${release_args[@]}"
if [[ -n "$maintenance_home" && -d "$maintenance_home/.local/share/cuan/accounts" ]]; then
  accounts_dir="$maintenance_home/.local/share/cuan/accounts"
  accounts_uid="$(stat -c %u "$accounts_dir")"
  if [[ "$(id -u)" == 0 && "$accounts_uid" != 0 ]]; then
    sudo -n -u "#$accounts_uid" python3 "$install_dir/maintenance/prune-provider-go-cache.py" --accounts "$accounts_dir" --apply
  else
    python3 "$install_dir/maintenance/prune-provider-go-cache.py" --accounts "$accounts_dir" --apply
  fi
fi
if [[ -d "$install_dir/data/projects/verification-snapshots" ]]; then
  python3 "$install_dir/maintenance/prune-verification-dependencies.py" \
    --snapshots "$install_dir/data/projects/verification-snapshots" --apply
fi
