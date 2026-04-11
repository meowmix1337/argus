.PHONY: help dev-backend dev-frontend build-backend build-frontend \
        test coverage lint install-hooks clean \
        prod-build prod-up prod-down prod-logs \
        dev-start dev-stop dev-reset dev-logs

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- Local Dev (no Docker) ---
dev-backend:  ## Run Go backend locally
	cd backend && go run ./cmd/server

dev-frontend:  ## Run Vite dev server
	cd frontend && npm run dev

# --- Build ---
build-backend:  ## Build Go binary
	cd backend && go build -o bin/server ./cmd/server

build-frontend:  ## Build frontend for production
	cd frontend && npm run build

# --- Quality ---
test:  ## Run all tests
	cd backend && go test -race ./...

coverage:  ## Run tests and print per-package coverage report
	cd backend && go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

lint:  ## Run linters (requires golangci-lint v2: https://golangci-lint.run/welcome/install/)
	cd backend && golangci-lint run ./...
	cd frontend && npm run lint

install-hooks:  ## Install pre-commit hooks (run once after cloning; requires: pip install pre-commit)
	pre-commit install

# --- Production (Docker) ---
prod-build:  ## Build production images
	docker compose build

prod-up:  ## Start production stack
	docker compose up -d

prod-down:  ## Stop production stack
	docker compose down

prod-logs:  ## Tail production logs
	docker compose logs -f

# --- Dev (Docker + air + NSQ + persistent DB) ---
dev-start:  ## Build and start dev stack in background
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build

dev-stop:  ## Stop dev stack
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down

dev-reset:  ## Stop dev stack and wipe local SQLite database
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down
	rm -f data/dashboard.db data/dashboard.db-shm data/dashboard.db-wal

dev-logs:  ## Tail dev stack logs
	docker compose -f docker-compose.yml -f docker-compose.dev.yml logs -f

# --- Cleanup ---
clean:  ## Remove build artifacts
	rm -rf backend/bin frontend/dist
