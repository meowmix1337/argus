.PHONY: help dev dev-backend dev-frontend build-backend build-frontend \
        test coverage lint install-hooks docker-build docker-up docker-down docker-dev docker-logs clean \
        docker-dev-up docker-dev-down docker-dev-reset docker-dev-logs

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

# --- Docker ---
docker-build:  ## Build production images
	docker compose build

docker-up:  ## Start production stack
	docker compose up -d

docker-down:  ## Stop production stack
	docker compose down

docker-dev:  ## Start dev stack with hot reload
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up

docker-logs:  ## Tail logs from all containers
	docker compose logs -f

docker-dev-up:  ## Start dev stack (air + NSQ + persistent DB) in background
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d

docker-dev-down:  ## Stop dev stack
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down

docker-dev-reset:  ## Stop dev stack and wipe local SQLite database
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down
	rm -f data/dashboard.db data/dashboard.db-shm data/dashboard.db-wal

docker-dev-logs:  ## Tail dev stack logs
	docker compose -f docker-compose.yml -f docker-compose.dev.yml logs -f

# --- Cleanup ---
clean:  ## Remove build artifacts
	rm -rf backend/bin frontend/dist
