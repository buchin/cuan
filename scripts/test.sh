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

if [[ "$MODE" == "all" && -d "$ROOT_DIR/.git" ]]; then
  # Smart change detection: if only frontend files changed relative to HEAD, avoid heavy Go backend test runs
  CHANGED_BACKEND=$(git -C "$ROOT_DIR" diff --name-only HEAD 2>/dev/null | grep -E '^(internal/|pkg/|cmd/|migrations/|go\.(mod|sum))' || true)
  CHANGED_UNTRACKED_BACKEND=$(git -C "$ROOT_DIR" status --porcelain 2>/dev/null | grep -E '^\?\? (internal/|pkg/|cmd/|migrations/|go\.(mod|sum))' || true)
  CHANGED_FRONTEND=$(git -C "$ROOT_DIR" diff --name-only HEAD 2>/dev/null | grep -E '^web/' || true)
  if [[ -n "$CHANGED_FRONTEND" && -z "$CHANGED_BACKEND" && -z "$CHANGED_UNTRACKED_BACKEND" ]]; then
    echo "==> Change detection: only web/ changed, skipping Go backend test suites."
    MODE="frontend"
  fi
fi

if [[ "$MODE" == "all" || "$MODE" == "backend" ]]; then
  echo "==> Testing Go packages (config, auth, db, task, runner)..."
  if [[ "$FAST_MODE" == "true" ]]; then
    # Stable module-relative debug paths let identical task/verifier checkouts
    # reuse compiled packages instead of keying them by workspace directory.
    # -short selects the fast cases. The timeout bounds the whole package,
    # including its many native process fixtures on constrained workers.
    go -C "$ROOT_DIR" test -trimpath -short -timeout 180s ./internal/config ./internal/auth ./internal/db ./internal/task ./internal/runner ./pkg/extensions
  else
    go -C "$ROOT_DIR" test -trimpath -v -timeout 600s ./internal/... ./pkg/...
  fi
  echo "✓ Go tests passed successfully."
fi

if [[ "$MODE" == "all" || "$MODE" == "frontend" ]]; then
  if [[ ! -d "$ROOT_DIR/web/node_modules/typescript" ]]; then
    echo "==> Frontend dependencies not installed in web/node_modules, skipping frontend validation."
  elif [[ "$FAST_MODE" == "true" ]]; then
    echo "==> Validating TypeScript types in web..."
    (cd "$ROOT_DIR/web" && npx --no-install tsc --noEmit)
    echo "✓ TypeScript check passed."
  else
    echo "==> Validating React Vite frontend build..."
    (cd "$ROOT_DIR/web" && npm run build)
    echo "✓ Frontend validation passed successfully."
  fi
fi

echo "==> All test suites passed!"
