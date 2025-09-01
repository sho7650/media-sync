.PHONY: test test-unit test-integration lint fmt tidy clean build

# TDD commands
tdd-red:
	@echo "🔴 RED: Running tests (should fail initially)"
	go test ./... -v

tdd-green:
	@echo "🟢 GREEN: Running tests (should pass)"
	go test ./... -v

tdd-refactor:
	@echo "🔵 REFACTOR: Running full quality checks"
	$(MAKE) fmt lint test

# Testing
test: test-unit test-integration

test-unit:
	go test ./pkg/... -v -race -coverprofile=coverage.out

test-integration:
	go test ./tests/integration/... -v

# Code quality
lint:
	golangci-lint run

fmt:
	go fmt ./...

tidy:
	go mod tidy

# Build
build:
	go build -o bin/media-sync ./cmd/daemon/
	go build -o bin/media-sync-cli ./cmd/cli/

# Clean
clean:
	rm -rf bin/ coverage.out

# Development workflow
dev-setup: tidy fmt test-unit lint
	@echo "✅ Development environment ready"

# Coverage
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report: coverage.html"