#!/usr/bin/env bash
set -euo pipefail
set -a
source .env
set +a
export SSHPASS="${CUAN_VPS_PASSWORD:-${CUAN_NEW_VPS_PASSWORD:-${DOJO_TERMAPI_VPS_PASSWORD:-}}}"
vps_user="${CUAN_VPS_USERNAME:-${CUAN_NEW_VPS_USERNAME:-${DOJO_TERMAPI_VPS_USERNAME:-ubuntu}}}"
vps_ip="${CUAN_VPS_IP:-${CUAN_NEW_VPS_IP:-${DOJO_TERMAPI_VPS_PUBLIC_IP:-}}}"
if [ $# -gt 0 ]; then
    sshpass -e ssh -o StrictHostKeyChecking=no "$vps_user@$vps_ip" "sudo -u postgres psql -d cuan" <<< "$*"
else
    sshpass -e ssh -o StrictHostKeyChecking=no "$vps_user@$vps_ip" "sudo -u postgres psql -d cuan"
fi
