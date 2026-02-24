.PHONY: build run test lint vet sqlc migrate-up migrate-down migrate-create \
        clean tidy docker-up docker-down setup mqtt-passwd

BINARY_NAME=conduit
DATABASE_URL?=postgres://conduit:password@localhost:5432/conduit?sslmode=disable
DEPLOY_DIR=deploy

# Load .env if it exists (for targets that need env vars directly)
-include .env
export

# =============================================================================
# Build & Run
# =============================================================================

build:
	go build -o bin/$(BINARY_NAME) ./cmd/conduit

build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY_NAME)-linux ./cmd/conduit

run: build
	./bin/$(BINARY_NAME)

clean:
	rm -rf bin/

tidy:
	go mod tidy

# =============================================================================
# Testing & Quality
# =============================================================================

test:
	go test -race ./...

test-verbose:
	go test -race -v ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

# =============================================================================
# Code Generation
# =============================================================================

sqlc:
	sqlc generate

# =============================================================================
# Database Migrations
# =============================================================================

migrate-up:
	migrate -database "$(DATABASE_URL)" -path migrations up

migrate-down:
	migrate -database "$(DATABASE_URL)" -path migrations down

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

# =============================================================================
# Docker Infrastructure
# =============================================================================

docker-up:
	docker compose -f $(DEPLOY_DIR)/docker-compose.yml up -d

docker-down:
	docker compose -f $(DEPLOY_DIR)/docker-compose.yml down

# =============================================================================
# First-time Setup
# =============================================================================

# mqtt-passwd: generate the Mosquitto password file for the 'server' user.
# Uses Docker so mosquitto_passwd doesn't need to be installed locally.
# Usage:  make mqtt-passwd MQTT_PASSWORD=<your-password>
#         If MQTT_PASSWORD is set in .env it will be picked up automatically.
mqtt-passwd:
	@if [ -z "$(MQTT_PASSWORD)" ]; then \
		echo "Error: MQTT_PASSWORD is not set. Run: make mqtt-passwd MQTT_PASSWORD=<password>"; \
		exit 1; \
	fi
	@echo "Generating Mosquitto password file for user 'server'..."
	@docker run --rm \
		-v "$(shell pwd)/$(DEPLOY_DIR)/mosquitto:/mosquitto/config" \
		eclipse-mosquitto:2 \
		mosquitto_passwd -c -b /mosquitto/config/passwd server "$(MQTT_PASSWORD)"
	@echo "Done. $(DEPLOY_DIR)/mosquitto/passwd has been updated."

# setup: full first-time setup sequence.
#   1. Generates Mosquitto passwd file
#   2. Starts Docker infrastructure
#   3. Waits for Postgres to be ready
#   4. Runs database migrations
setup: mqtt-passwd docker-up
	@echo "Waiting for Postgres to be ready..."
	@until docker exec conduit-postgres pg_isready -U conduit -d conduit > /dev/null 2>&1; do \
		printf '.'; sleep 1; \
	done
	@echo ""
	@echo "Running migrations..."
	@$(MAKE) migrate-up
	@echo ""
	@echo "Setup complete. Start the server with: make run"
