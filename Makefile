.PHONY: help build run stop clean test lint dev-setup

# Default target
help: ## Show this help message
	@echo "TradeCaptain - Development Commands"
	@echo "Author: Tarun Thakur (thakur[dot]cs[dot]tarun[at]gmail[dot]com)"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development Setup
dev-setup: ## Set up development environment
	@echo "Setting up development environment..."
	cp .env.example .env
	@echo "Please edit .env file with your API keys"
	docker-compose up -d postgres redis kafka
	@echo "Waiting for services to be ready..."
	sleep 10
	make db-migrate
	@echo "Development environment ready!"

# Database Operations
db-migrate: ## Run database migrations
	@echo "Running database migrations..."
	docker-compose exec postgres psql -U tradecaptain_user -d tradecaptain -f /docker-entrypoint-initdb.d/init.sql

db-reset: ## Reset database (WARNING: destroys all data)
	docker-compose down -v
	docker volume rm tradecaptain_postgres_data tradecaptain_timescaledb_data
	docker-compose up -d postgres timescaledb
	sleep 5
	make db-migrate

# Build Commands
build: ## Build all services
	@echo "Building all services..."
	docker-compose build

build-data-collector: ## Build data collector service
	cd services/data-collector && go build -o bin/data-collector .

build-api-gateway: ## Build API gateway service
	cd services/api-gateway && go build -o bin/api-gateway .

build-calc-engine: ## Build calculation engine
	cd services/calculation-engine && cargo build --release

build-frontend: ## Build frontend
	cd frontend && npm run build

# Run Commands
run: ## Start all services with Docker Compose
	docker-compose up -d

run-dev: ## Start services in development mode
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml up

run-local: ## Run services locally (without Docker)
	@echo "Starting local development servers..."
	# Start infrastructure services
	docker-compose up -d postgres redis kafka
	# Run services locally
	make run-data-collector & \
	make run-calc-engine & \
	make run-api-gateway & \
	make run-frontend

run-data-collector: ## Run data collector locally
	cd services/data-collector && go run main.go

run-api-gateway: ## Run API gateway locally
	cd services/api-gateway && go run main.go

run-calc-engine: ## Run calculation engine locally
	cd services/calculation-engine && cargo run

run-frontend: ## Run frontend development server
	cd frontend && npm run dev

# Stop Commands
stop: ## Stop all services
	docker-compose down

stop-all: ## Stop and remove all containers, networks, and volumes
	docker-compose down -v
	docker system prune -f

# Testing Commands
test: ## Run all tests
	@echo "Running all tests..."
	make test-go
	make test-rust
	make test-frontend

test-go: ## Run Go tests
	cd services/data-collector && go test ./...
	cd services/api-gateway && go test ./...

test-rust: ## Run Rust tests
	cd services/calculation-engine && cargo test

test-frontend: ## Run frontend tests
	cd frontend && npm test

# Linting and Formatting
lint: ## Run linters on all services
	make lint-go
	make lint-rust
	make lint-frontend

lint-go: ## Run Go linter
	cd services/data-collector && go fmt ./... && go vet ./...
	cd services/api-gateway && go fmt ./... && go vet ./...

lint-rust: ## Run Rust linter
	cd services/calculation-engine && cargo fmt && cargo clippy

lint-frontend: ## Run frontend linter
	cd frontend && npm run lint

# Monitoring and Logs
logs: ## Show logs from all services
	docker-compose logs -f

logs-data-collector: ## Show data collector logs
	docker-compose logs -f data-collector

logs-api-gateway: ## Show API gateway logs
	docker-compose logs -f api-gateway

logs-calc-engine: ## Show calculation engine logs
	docker-compose logs -f calculation-engine

# Maintenance Commands
clean: ## Clean up build artifacts and dependencies
	@echo "Cleaning up..."
	cd services/data-collector && go clean
	cd services/calculation-engine && cargo clean
	cd frontend && rm -rf node_modules dist
	docker system prune -f

update-deps: ## Update all dependencies
	cd services/data-collector && go mod tidy && go mod download
	cd services/api-gateway && go mod tidy && go mod download
	cd services/calculation-engine && cargo update
	cd frontend && npm update

# Production Commands
build-prod: ## Build for production
	docker-compose -f docker-compose.yml -f docker-compose.prod.yml build

deploy-prod: ## Deploy to production
	docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Utility Commands
generate-docs: ## Generate API documentation
	cd services/api-gateway && swag init

benchmark: ## Run performance benchmarks
	cd services/calculation-engine && cargo bench

security-scan: ## Run security scans
	cd services/data-collector && go list -m all | nancy sleuth
	cd services/api-gateway && go list -m all | nancy sleuth
	cd services/calculation-engine && cargo audit
	cd frontend && npm audit

# Database Utilities
db-backup: ## Create database backup
	docker-compose exec postgres pg_dump -U tradecaptain_user tradecaptain > backup_$(shell date +%Y%m%d_%H%M%S).sql

db-restore: ## Restore database from backup (specify BACKUP_FILE)
	docker-compose exec -T postgres psql -U tradecaptain_user tradecaptain < $(BACKUP_FILE)

# Monitoring
monitor: ## Open monitoring dashboard
	@echo "Opening Grafana dashboard at http://localhost:3001"
	@echo "Username: admin, Password: admin"
	open http://localhost:3001 || xdg-open http://localhost:3001