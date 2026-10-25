#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

show_help() {
  cat << 'EOF'
Cuan Build Script

Usage:
  ./scripts/build.sh [options]

Options:
  --backend-only    Build only Go binaries (cuan-server, cuan-runner, cuan-migrate)
  --frontend-only   Build only React Vite frontend
  --help            Show this help message

Output:
  Binaries are placed in bin/
  Frontend assets are placed in web/dist/
EOF
}

TARGET="all"
for arg in "$@"; do
  case "$arg" in
    --backend-only) TARGET="backend" ;;
    --frontend-only) TARGET="frontend" ;;
    --help|-h) show_help; exit 0 ;;
    *) echo "Unknown option: $arg"; show_help; exit 1 ;;
  esac
done

echo "==> Cuan Monorepo Build (Target: $TARGET)"

if [[ "$TARGET" == "all" || "$TARGET" == "backend" ]]; then
  if ! command -v go &>/dev/null; then
    echo "Error: 'go' is not installed or not in PATH." >&2
    exit 1
  fi
  echo "==> Compiling Go binaries into bin/..."
  mkdir -p "$ROOT_DIR/bin"
  GIT_COMMIT="$(git -C "$ROOT_DIR" rev-parse HEAD 2>/dev/null || echo '')"
  go -C "$ROOT_DIR" build -ldflags "-X github.com/buchin/cuan/internal/config.GitCommit=${GIT_COMMIT}" -o "$ROOT_DIR/bin/cuan-server" ./cmd/server
  go -C "$ROOT_DIR" build -o "$ROOT_DIR/bin/cuan-runner" ./cmd/runner
  go -C "$ROOT_DIR" build -o "$ROOT_DIR/bin/cuan-migrate" ./cmd/migrate
  echo "✓ Go backend binaries built: bin/cuan-server, bin/cuan-runner, bin/cuan-migrate"
fi

if [[ "$TARGET" == "all" || "$TARGET" == "frontend" ]]; then
  if ! command -v npm &>/dev/null; then
    echo "Error: 'npm' is not installed or not in PATH." >&2
    exit 1
  fi
  echo "==> Building frontend assets in web/..."
  (cd "$ROOT_DIR/web" && npm run build)
  echo "✓ Frontend built successfully into web/dist/"
fi

echo "==> All requested builds finished successfully."
