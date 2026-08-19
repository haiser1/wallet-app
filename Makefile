.PHONY: run test test-unit test-integration docker-up docker-down migrate

# Start the application
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

# Start PostgreSQL container
docker-up:
	docker compose up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3

# Stop PostgreSQL container
docker-down:
	docker compose down

# Reset database (drop and recreate)
docker-reset:
	docker compose down -v
	docker compose up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
