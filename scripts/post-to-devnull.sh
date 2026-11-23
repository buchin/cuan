#!/usr/bin/env bash
set -euo pipefail

# post-to-devnull.sh: Publish updates, ship logs, or release announcements to dev-null.sh
# Usage:
#   scripts/post-to-devnull.sh "Shipped Cuan: feature description"
#   scripts/post-to-devnull.sh --image screenshots/image.png "Shipped Cuan: feature description"
#   scripts/post-to-devnull.sh --reply <commit-hash> "Reply message"

if [ $# -eq 0 ]; then
  echo "Usage: scripts/post-to-devnull.sh [--image <path>] [--reply <hash>] <message>"
  exit 1
fi

curl -fsSL https://dev-null.sh/commit | bash -s -- "$@"
