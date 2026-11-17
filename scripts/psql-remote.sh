#!/usr/bin/env bash
set -euo pipefail
set -a
source .env
set +a
export SSHPASS="$CUAN_NEW_VPS_PASSWORD"
if [ $# -gt 0 ]; then
    sshpass -e ssh -o StrictHostKeyChecking=no "$CUAN_NEW_VPS_USERNAME@$CUAN_NEW_VPS_IP" "sudo -u postgres psql -d cuan" <<< "$*"
else
    sshpass -e ssh -o StrictHostKeyChecking=no "$CUAN_NEW_VPS_USERNAME@$CUAN_NEW_VPS_IP" "sudo -u postgres psql -d cuan"
fi
