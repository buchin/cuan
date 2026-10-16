#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

show_help() {
  cat << 'EOF'
Cuan Migration Runner Wrapper

Usage:
  ./scripts/migrate.sh [options]

Options:
  --dry-run           Print SQL statements without applying
  --down              Rollback migrations
  --db-url <URL>      PostgreSQL connection URL
  --help              Show this help message

Environment:
  DATABASE_URL        PostgreSQL connection URL (e.g. postgres://user:pass@host:5432/cuan?sslmode=disable)
EOF
}

DIRECTION="up"
DRY_RUN=false
DB_URL="${DATABASE_URL:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=true; shift ;;
    --down) DIRECTION="down"; shift ;;
    --db-url) DB_URL="$2"; shift 2 ;;
    --help|-h) show_help; exit 0 ;;
    *) echo "Unknown option: $1"; show_help; exit 1 ;;
  esac
done

if [[ "$DRY_RUN" == "true" ]]; then
  echo "==> Running migration in dry-run mode (direction: $DIRECTION)..."
  go -C "$ROOT_DIR" run ./cmd/migrate --dry-run --direction "$DIRECTION"
  exit 0
fi

if [[ -z "$DB_URL" ]]; then
  echo "Notice: DATABASE_URL is not set."
  echo "==> Falling back to migration dry-run to verify SQL statements..."
  go -C "$ROOT_DIR" run ./cmd/migrate --dry-run --direction "$DIRECTION"
  echo ""
  echo "To apply against a real database, set DATABASE_URL or pass --db-url:"
  echo "  export DATABASE_URL='postgres://user:pass@localhost:5432/cuan?sslmode=disable'"
  echo "  ./scripts/migrate.sh"
  exit 0
fi

echo "==> Applying migrations ($DIRECTION) against database..."
go -C "$ROOT_DIR" run ./cmd/migrate --db-url "$DB_URL" --direction "$DIRECTION"
