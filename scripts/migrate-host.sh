#!/usr/bin/env bash
# ==============================================================================
# Cuan Host Migration Runner Wrapper
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "${SCRIPT_DIR}/migrate-host.py" "$@"
