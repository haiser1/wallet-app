.PHONY: run test test-unit test-integration docker-up docker-down docker-reset docker-build swagger

# help
help:
	@echo "Usage: make <command>"
	@echo ""
	@echo "Commands:"
	@echo "  run                Start the application locally"
	@echo "  test               Run all tests"
	@echo "  test-unit          Run unit tests only"
	@echo "  test-integration   Run integration tests only"
	@echo "  swagger            Generate OpenAPI / Swagger docs"
	@echo "  docker-up          Start app and PostgreSQL containers"
	@echo "  docker-down        Stop containers"
	@echo "  docker-reset       Reset containers and volumes"
	@echo "  docker-build       Build Docker image for the app"

# Start the application locally
run:
	go run cmd/server/main.go

# Run all tests
test: test-unit test-integration

# Run unit tests only
test-unit:
	go test ./tests/unit/... -v -count=1

# Run integration tests (requires PostgreSQL)
test-integration:
	go test ./tests/integration/... -v -count=1 -timeout=120s

# Generate Swagger docs
swagger:
	swag init -g cmd/server/main.go

# Build Docker image
docker-build:
	docker build -t wallet-app:latest .

# Start containers (App + PostgreSQL)
docker-up:
	docker compose up -d --build
	@echo "Waiting for services to be ready..."
	@sleep 3

# Stop containers
docker-down:
	docker compose down

# Reset database (drop and recreate)
docker-reset:
	docker compose down -v
	docker compose up -d --build
	@echo "Waiting for services to be ready..."
	@sleep 3
