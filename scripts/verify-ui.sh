#!/usr/bin/env bash
# Fixed project visual entry point. Never accepts commands from task text.
set -euo pipefail
repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ -z "${CUAN_BROWSER_MODULE:-}" ] && [ -d "/home/ubuntu/.local/lib/node_modules/playwright" ]; then
  export CUAN_BROWSER_MODULE="/home/ubuntu/.local/lib/node_modules/playwright"
fi

node "$repository_root/web/scripts/verify-ui.mjs" "${1:?Provide a private output directory}"
