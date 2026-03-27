.PHONY: help dev dev-backend dev-frontend build-backend build-frontend \
        test lint docker-build docker-up docker-down docker-dev docker-logs clean

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

lint:  ## Run linters (requires golangci-lint v2: https://golangci-lint.run/welcome/install/)
	cd backend && golangci-lint run ./...
	cd frontend && npm run lint

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

# --- Cleanup ---
clean:  ## Remove build artifacts
	rm -rf backend/bin frontend/dist
