#!/usr/bin/env bash
# Backwards-compatibility wrapper for scripts/deploy.sh
exec "$(dirname "$0")/deploy.sh" "$@"
