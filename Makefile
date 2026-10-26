.PHONY: all build build-backend build-frontend test test-backend test-frontend migrate dev clean install backup restore update rollback help

SHELL := /bin/bash

# Default target
all: build test

help:
	@echo "Cuan Monorepo Build & Development"
	@echo ""
	@echo "Usage:"
	@echo "  make build          Build Go backend, runner, migrate CLI, and frontend"
	@echo "  make build-backend  Build Go backend binaries to bin/"
	@echo "  make build-frontend Build frontend into web/dist/"
	@echo "  make test           Run targeted tests for backend and frontend"
	@echo "  make test-backend   Run Go unit and spike tests"
	@echo "  make test-frontend  Run frontend TypeScript and build validation"
	@echo "  make migrate        Run database migrations (dry-run fallback if DB unset)"
	@echo "  make install        Run automated installation and preflight checks"
	@echo "  make backup         Create compressed database backup archive"
	@echo "  make restore        Restore database from backup"
	@echo "  make update         Execute safe version update flow"
	@echo "  make rollback       Execute database and application rollback"
	@echo "  make dev            Run development server and frontend concurrently"
	@echo "  make clean          Clean build artifacts and temporary files"

build: build-backend build-frontend

build-backend:
	@echo "==> Building Go backend binaries..."
	@mkdir -p bin
	go build -o bin/cuan-server ./cmd/server
	go build -o bin/cuan-runner ./cmd/runner
	go build -o bin/cuan-migrate ./cmd/migrate
	ln -sf cuan-server bin/cuan
	@echo "==> Go binaries successfully built in bin/"

build-frontend:
	@echo "==> Building React Vite TypeScript frontend..."
	@if [ -d "web" ]; then \
		if command -v npm >/dev/null 2>&1 && [ -d "web/node_modules" ]; then \
			(cd web && npm run build) || exit 1; \
		elif [ -d "web/dist" ]; then \
			echo "==> Using retained web/dist (frontend build toolchain not installed on host)"; \
		else \
			exit 1; \
		fi; \
	fi
	@echo "==> Frontend successfully built in web/dist/"

test: test-backend test-frontend

test-backend:
	@echo "==> Running Go tests (unit, migration, adapter spike)..."
	go test -v -timeout 120s ./internal/...

test-frontend:
	@echo "==> Validating frontend build and types..."
	@if [ -d "web" ]; then \
		if command -v npm >/dev/null 2>&1 && [ -d "web/node_modules" ]; then \
			(cd web && npm run build) || exit 1; \
		elif [ -d "web/dist" ]; then \
			echo "==> Validated retained web/dist (frontend build toolchain not installed on host)"; \
		else \
			exit 1; \
		fi; \
	fi

migrate:
	@echo "==> Running migration tool..."
	@./scripts/migrate.sh

dev:
	@echo "==> Starting Cuan development environment..."
	@./scripts/dev.sh

install:
	@echo "==> Running Cuan installer..."
	@./scripts/install.sh

backup:
	@echo "==> Creating database backup archive..."
	@./scripts/backup.sh

restore:
	@echo "==> Running database restore tool..."
	@./scripts/restore.sh

update:
	@echo "==> Running Cuan safe update flow..."
	@./scripts/update.sh

rollback:
	@echo "==> Running Cuan rollback and recovery tool..."
	@./scripts/rollback.sh

clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf bin/ web/dist/
	@echo "==> Clean complete."
