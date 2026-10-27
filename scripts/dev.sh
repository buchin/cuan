#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

show_help() {
  cat << 'EOF'
Cuan Development Runner

Usage:
  ./scripts/dev.sh [options]

Options:
  --server-only   Start only Go backend API server (:8080)
  --web-only      Start only Vite frontend dev server (:3000)
  --help          Show this help message

Environment:
  PORT            API server port (default: 8080)
  DATABASE_URL    PostgreSQL connection string
EOF
}

MODE="both"
for arg in "$@"; do
  case "$arg" in
    --server-only) MODE="server" ;;
    --web-only) MODE="web" ;;
    --help|-h) show_help; exit 0 ;;
    *) echo "Unknown option: $arg"; show_help; exit 1 ;;
  esac
done

cleanup() {
  echo ""
  echo "Shutting down dev servers..."
  trap - SIGINT SIGTERM
  kill -- -$$ 2>/dev/null || true
  exit 0
}

trap cleanup SIGINT SIGTERM

echo "==> Starting Cuan development environment ($MODE)..."

if [[ "$MODE" == "both" || "$MODE" == "server" ]]; then
  echo "==> Starting Go API server on :${PORT:-8080}..."
  go -C "$ROOT_DIR" run ./cmd/server &
fi

if [[ "$MODE" == "both" || "$MODE" == "web" ]]; then
  echo "==> Starting Frontend dev server..."
  (cd "$ROOT_DIR/web" && npm run dev) &
fi

wait
