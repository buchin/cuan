#!/usr/bin/env bash
# Fixed project visual entry point. Never accepts commands from task text.
set -euo pipefail
repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
node "$repository_root/web/scripts/verify-ui.mjs" "${1:?Provide a private output directory}"
