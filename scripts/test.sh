#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

show_help() {
  cat << 'EOF'
Cuan Test Runner

Usage:
  ./scripts/test.sh [options]

Options:
  --fast            Skip long-running agent spike tests (unit tests only)
  --backend-only    Run only Go tests
  --frontend-only   Run only frontend typecheck and build validation
  --help            Show this help message
EOF
}

FAST_MODE=false
MODE="all"

for arg in "$@"; do
  case "$arg" in
    --fast) FAST_MODE=true ;;
    --backend-only) MODE="backend" ;;
    --frontend-only) MODE="frontend" ;;
    --help|-h) show_help; exit 0 ;;
    *) echo "Unknown option: $arg"; show_help; exit 1 ;;
  esac
done

echo "==> Running Cuan verification tests..."

if [[ "$MODE" == "all" || "$MODE" == "backend" ]]; then
  echo "==> Testing Go packages (config, auth, db, task, runner)..."
  if [[ "$FAST_MODE" == "true" ]]; then
    # -short selects the fast cases. The timeout bounds the whole package,
    # including its many native process fixtures on constrained workers.
    go -C "$ROOT_DIR" test -short -timeout 120s ./internal/config ./internal/auth ./internal/db ./internal/task ./internal/runner ./pkg/extensions
  else
    go -C "$ROOT_DIR" test -v -timeout 120s ./internal/... ./pkg/...
  fi
  echo "✓ Go tests passed successfully."
fi

if [[ "$MODE" == "all" || "$MODE" == "frontend" ]]; then
  if [[ "$FAST_MODE" != "true" ]]; then
    echo "==> Validating React Vite frontend build..."
    (cd "$ROOT_DIR/web" && npm run build)
    echo "✓ Frontend validation passed successfully."
  fi
fi

echo "==> All test suites passed!"
