#!/bin/bash

# TDD Project Initialization Script for Media-Sync
# This script sets up the complete TDD development environment

set -e

PROJECT_NAME="media-sync"
PROJECT_ROOT="/Users/sho/working/golang/work/media-sync"
GITHUB_USERNAME="${GITHUB_USERNAME:-sho7650}"  # Set your GitHub username

echo "🚀 Initializing TDD environment for ${PROJECT_NAME}"

# Check if we're in the right directory
if [ ! -f "LICENSE" ]; then
    echo "❌ Error: Must run from project root directory"
    exit 1
fi

# Initialize Go module
echo "📦 Initializing Go module..."
if [ ! -f "go.mod" ]; then
    go mod init github.com/${GITHUB_USERNAME}/${PROJECT_NAME}
else
    echo "   ✅ go.mod already exists"
fi

# Create project directory structure following Go best practices
echo "📁 Creating directory structure..."

# Core directories
mkdir -p cmd/${PROJECT_NAME}
mkdir -p internal/{core,config,storage,plugins,sync,auth}
mkdir -p pkg/{api,utils}
mkdir -p plugins/{tumblr,local}
mkdir -p configs/examples
mkdir -p scripts/{dev,deploy,test}
mkdir -p docs/{api,guides}
mkdir -p test/{fixtures,integration,e2e}

# Test directories mirroring source structure
mkdir -p internal/{core,config,storage,plugins,sync,auth}/testdata
mkdir -p plugins/{tumblr,local}/testdata

echo "   ✅ Directory structure created"

# Create essential configuration files
echo "⚙️  Creating configuration files..."

# .golangci.yml - Linting configuration
cat > .golangci.yml << 'EOF'
# Golangci-lint configuration for TDD workflow
run:
  timeout: 5m
  tests: true

linters-settings:
  cyclop:
    max-complexity: 10
  funlen:
    lines: 100
    statements: 50
  gocognit:
    min-complexity: 20
  goconst:
    min-len: 3
    min-occurrences: 3
  gocyclo:
    min-complexity: 15
  godot:
    scope: declarations
    capital: false
  gofmt:
    simplify: true
  goimports:
    local-prefixes: github.com/sho7650/media-sync
  gomnd:
    settings:
      mnd:
        checks: argument,case,condition,operation,return,assign
  govet:
    check-shadowing: true
  misspell:
    locale: US
  nolintlint:
    allow-leading-space: false
    require-explanation: true
    require-specific: true

linters:
  enable:
    - asciicheck
    - bidichk
    - bodyclose
    - cyclop
    - dupl
    - durationcheck
    - errcheck
    - errorlint
    - exhaustive
    - exportloopref
    - funlen
    - gochecknoglobals
    - gocognit
    - goconst
    - gocritic
    - gocyclo
    - godot
    - gofmt
    - goimports
    - gomnd
    - gomoddirectives
    - gomodguard
    - goprintffuncname
    - gosec
    - gosimple
    - govet
    - ineffassign
    - misspell
    - nakedret
    - nilerr
    - nolintlint
    - prealloc
    - predeclared
    - revive
    - staticcheck
    - stylecheck
    - typecheck
    - unconvert
    - unparam
    - unused
    - vet
    - vetshadow
    - whitespace

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gomnd
        - funlen
        - gocognit
        - cyclop
EOF

# Makefile for TDD workflow
cat > Makefile << 'EOF'
# Makefile for TDD Workflow

.PHONY: help test test-unit test-integration test-e2e coverage lint fmt vet security clean deps build run docker

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Testing targets
test: test-unit test-integration ## Run all tests

test-unit: ## Run unit tests with coverage
	@echo "🧪 Running unit tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report generated: coverage.html"

test-integration: ## Run integration tests
	@echo "🔗 Running integration tests..."
	go test -v -tags=integration ./test/integration/...

test-e2e: ## Run end-to-end tests
	@echo "🎭 Running E2E tests..."
	go test -v -tags=e2e ./test/e2e/...

test-watch: ## Watch for changes and run tests
	@echo "👀 Watching for changes..."
	find . -name "*.go" | entr -c make test-unit

coverage: ## Generate and display test coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1
	@echo "Open coverage.html in browser for detailed report"

benchmark: ## Run benchmarks
	@echo "⚡ Running benchmarks..."
	go test -bench=. -benchmem ./...

# Quality targets
lint: ## Run golangci-lint
	@echo "🔍 Running linter..."
	golangci-lint run

fmt: ## Format Go code
	@echo "🎨 Formatting code..."
	go fmt ./...
	goimports -w .

vet: ## Run go vet
	@echo "🔬 Running vet..."
	go vet ./...

security: ## Run security scan
	@echo "🛡️ Running security scan..."
	gosec ./...

# Build targets
deps: ## Download dependencies
	@echo "📥 Downloading dependencies..."
	go mod download
	go mod tidy

build: ## Build the application
	@echo "🔨 Building application..."
	go build -o bin/media-sync ./cmd/media-sync

build-plugins: ## Build plugin binaries
	@echo "🔌 Building plugins..."
	go build -buildmode=plugin -o plugins/tumblr.so ./plugins/tumblr
	go build -buildmode=plugin -o plugins/local.so ./plugins/local

run: build ## Run the application
	@echo "🚀 Running application..."
	./bin/media-sync

# Development targets
dev-setup: deps ## Set up development environment
	@echo "🛠️ Setting up development environment..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/vektra/mockery/v2@latest

generate-mocks: ## Generate test mocks
	@echo "🎭 Generating mocks..."
	mockery --all --dir=./internal/core --output=./internal/mocks

clean: ## Clean build artifacts
	@echo "🧹 Cleaning..."
	rm -rf bin/
	rm -rf *.out
	rm -rf coverage.html
	rm -rf plugins/*.so

# Git workflow helpers
git-hooks: ## Install git hooks
	@echo "🪝 Installing git hooks..."
	cp scripts/git-hooks/* .git/hooks/
	chmod +x .git/hooks/*

# TDD workflow helpers
tdd-red: ## Start TDD red phase (write failing test)
	@echo "🔴 TDD RED: Write your failing test"
	@echo "Run 'make test-unit' to confirm it fails"

tdd-green: ## TDD green phase (make test pass)
	@echo "🟢 TDD GREEN: Write minimal code to make test pass"
	@echo "Run 'make test-unit' to confirm it passes"

tdd-refactor: ## TDD refactor phase (improve code)
	@echo "🔄 TDD REFACTOR: Improve code while keeping tests green"
	@echo "Run 'make test-unit lint' to ensure quality"

# Docker targets (for CI/CD)
docker-build: ## Build Docker image
	@echo "🐳 Building Docker image..."
	docker build -t media-sync:latest .

docker-test: ## Run tests in Docker
	@echo "🐳 Running tests in Docker..."
	docker run --rm media-sync:latest make test

# CI/CD helpers
ci-setup: deps ## Setup for CI environment
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest

ci-test: test lint security ## Run all CI tests
	@echo "✅ All CI checks passed"

# Documentation
docs: ## Generate documentation
	@echo "📚 Generating documentation..."
	go doc -all ./... > docs/api/generated.md

# Database migrations (for future use)
migrate-up: ## Run database migrations
	@echo "⬆️ Running migrations..."
	# Add migration commands when database layer is implemented

migrate-down: ## Rollback database migrations
	@echo "⬇️ Rolling back migrations..."
	# Add rollback commands when database layer is implemented
EOF

# GitHub Actions CI workflow
mkdir -p .github/workflows
cat > .github/workflows/ci.yml << 'EOF'
name: CI/CD Pipeline

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

env:
  GO_VERSION: '1.21'
  COVERAGE_THRESHOLD: 80

jobs:
  test:
    name: Test Suite
    runs-on: ubuntu-latest
    
    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: ${{ env.GO_VERSION }}

    - name: Cache Go modules
      uses: actions/cache@v3
      with:
        path: |
          ~/go/pkg/mod
          ~/.cache/go-build
        key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
        restore-keys: |
          ${{ runner.os }}-go-

    - name: Download dependencies
      run: go mod download

    - name: Run unit tests
      run: |
        go test -v -race -coverprofile=coverage.out ./...
        go tool cover -func=coverage.out -o=coverage.txt

    - name: Check test coverage
      run: |
        coverage=$(tail -1 coverage.txt | awk '{print $3}' | sed 's/%//')
        echo "Coverage: ${coverage}%"
        if (( $(echo "$coverage < $COVERAGE_THRESHOLD" | bc -l) )); then
          echo "❌ Coverage ${coverage}% is below threshold ${COVERAGE_THRESHOLD}%"
          exit 1
        fi
        echo "✅ Coverage ${coverage}% meets threshold ${COVERAGE_THRESHOLD}%"

    - name: Upload coverage to Codecov
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage.out
        flags: unittests
        name: codecov-umbrella

  lint:
    name: Code Quality
    runs-on: ubuntu-latest
    
    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: ${{ env.GO_VERSION }}

    - name: Run golangci-lint
      uses: golangci/golangci-lint-action@v3
      with:
        version: latest
        args: --timeout=5m

  security:
    name: Security Scan
    runs-on: ubuntu-latest
    
    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: ${{ env.GO_VERSION }}

    - name: Run gosec
      run: |
        go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
        gosec ./...

  integration:
    name: Integration Tests
    runs-on: ubuntu-latest
    needs: [test, lint]
    
    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: ${{ env.GO_VERSION }}

    - name: Run integration tests
      run: go test -v -tags=integration ./test/integration/...

  build:
    name: Build Application
    runs-on: ubuntu-latest
    needs: [test, lint, security]
    
    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: ${{ env.GO_VERSION }}

    - name: Build application
      run: |
        go build -o bin/media-sync ./cmd/media-sync
        ls -la bin/

    - name: Upload build artifacts
      uses: actions/upload-artifact@v3
      with:
        name: media-sync-binary
        path: bin/media-sync
        retention-days: 7
EOF

# Create git hooks for TDD workflow
mkdir -p scripts/git-hooks
cat > scripts/git-hooks/pre-commit << 'EOF'
#!/bin/bash
# Pre-commit hook for TDD workflow

set -e

echo "🔍 Running pre-commit checks..."

# Check if there are any Go files to process
if ! git diff --cached --name-only --diff-filter=ACM | grep '\.go$' > /dev/null; then
    echo "No Go files to check"
    exit 0
fi

# Run gofmt
echo "🎨 Checking code format..."
unformatted=$(gofmt -l $(git diff --cached --name-only --diff-filter=ACM | grep '\.go$'))
if [ ! -z "$unformatted" ]; then
    echo "❌ The following files are not formatted:"
    echo "$unformatted"
    echo "Please run: go fmt ./..."
    exit 1
fi

# Run tests
echo "🧪 Running tests..."
if ! make test-unit > /dev/null 2>&1; then
    echo "❌ Tests are failing. Please fix before committing."
    exit 1
fi

# Run linter
echo "🔍 Running linter..."
if ! make lint > /dev/null 2>&1; then
    echo "❌ Linting issues found. Please fix before committing."
    exit 1
fi

echo "✅ All pre-commit checks passed!"
EOF

cat > scripts/git-hooks/pre-push << 'EOF'
#!/bin/bash
# Pre-push hook for TDD workflow

set -e

echo "🚀 Running pre-push checks..."

# Run full test suite
echo "🧪 Running full test suite..."
if ! make test > /dev/null 2>&1; then
    echo "❌ Full test suite failed. Push aborted."
    exit 1
fi

# Run security scan
echo "🛡️ Running security scan..."
if ! make security > /dev/null 2>&1; then
    echo "❌ Security issues found. Push aborted."
    exit 1
fi

echo "✅ All pre-push checks passed!"
EOF

chmod +x scripts/git-hooks/*

echo "   ✅ Configuration files created"

# Add essential Go dependencies
echo "📥 Adding essential dependencies..."

# Core dependencies
go get -u github.com/stretchr/testify/assert
go get -u github.com/stretchr/testify/require
go get -u github.com/stretchr/testify/mock
go get -u github.com/stretchr/testify/suite

# Configuration
go get -u gopkg.in/yaml.v3
go get -u github.com/spf13/viper
go get -u github.com/spf13/cobra

# Database
go get -u github.com/mattn/go-sqlite3
go get -u github.com/jmoiron/sqlx

# HTTP client and server
go get -u github.com/gorilla/mux
go get -u github.com/go-chi/chi/v5

# Authentication
go get -u golang.org/x/oauth2

# Utilities
go get -u github.com/google/uuid
go get -u github.com/rs/zerolog

# File watching (for hot reload)
go get -u github.com/fsnotify/fsnotify

echo "   ✅ Dependencies added"

# Create main.go entry point
echo "🎯 Creating main entry point..."

cat > cmd/${PROJECT_NAME}/main.go << 'EOF'
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sho7650/media-sync/internal/core"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down gracefully...")
		cancel()
	}()

	// Initialize application
	app, err := core.NewApplication()
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Start application
	if err := app.Start(ctx); err != nil {
		log.Fatalf("Application failed: %v", err)
	}

	fmt.Println("Media-sync terminated")
}
EOF

# Create basic application structure
cat > internal/core/app.go << 'EOF'
package core

import (
	"context"
	"fmt"
)

// Application represents the main application
type Application struct {
	// Will be populated as we implement TDD cycles
}

// NewApplication creates a new application instance
func NewApplication() (*Application, error) {
	return &Application{}, nil
}

// Start starts the application
func (a *Application) Start(ctx context.Context) error {
	fmt.Println("🚀 Media-sync starting...")
	fmt.Println("📝 Ready for TDD implementation!")
	
	// Wait for context cancellation
	<-ctx.Done()
	return nil
}
EOF

# Create first test file to demonstrate TDD
mkdir -p internal/core
cat > internal/core/app_test.go << 'EOF'
package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewApplication_CreatesInstance(t *testing.T) {
	// RED: This test passes immediately, demonstrating basic setup
	app, err := NewApplication()
	
	require.NoError(t, err)
	assert.NotNil(t, app)
}

func TestApplication_Start_RespondsToContext(t *testing.T) {
	// RED: Test that app responds to context cancellation
	app, err := NewApplication()
	require.NoError(t, err)
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// This should return when context is canceled
	err = app.Start(ctx)
	assert.NoError(t, err)
}

// Example of a failing test (RED phase)
func TestApplication_LoadConfiguration_FailsInitially(t *testing.T) {
	t.Skip("RED: This test will fail until we implement LoadConfiguration")
	
	app, err := NewApplication()
	require.NoError(t, err)
	
	// This method doesn't exist yet - will fail compilation
	// config, err := app.LoadConfiguration("config.yaml")
	// require.NoError(t, err)
	// assert.NotNil(t, config)
}
EOF

echo "   ✅ Basic application structure created"

# Create example configuration files
echo "📋 Creating example configurations..."

cat > configs/examples/basic.yaml << 'EOF'
# Basic Media-Sync Configuration
# This is an example configuration for the TDD implementation

version: "1.0"

services:
  - name: tumblr-input
    type: input
    plugin: tumblr
    enabled: true
    settings:
      username: example-user
      # api_key will be loaded from environment or keyring
      fetch_limit: 50
      media_types: ["photo", "video"]
    
  - name: local-output
    type: output
    plugin: local
    enabled: true
    settings:
      output_directory: "./media"
      organize_by_date: true
      preserve_metadata: true

storage:
  type: sqlite
  path: "./media-sync.db"
  
logging:
  level: info
  format: json
  
security:
  keyring_service: media-sync
  encrypt_config: false
EOF

cat > configs/examples/advanced.yaml << 'EOF'
# Advanced Media-Sync Configuration
# Shows more complex setups for TDD implementation

version: "1.0"

services:
  # Multiple input sources
  - name: tumblr-blog1
    type: input
    plugin: tumblr
    enabled: true
    settings:
      username: tech-blog
      api_key_env: TUMBLR_API_KEY_1
      fetch_limit: 100
      since: "2024-01-01"
      
  - name: tumblr-blog2
    type: input
    plugin: tumblr
    enabled: true
    settings:
      username: art-blog
      api_key_env: TUMBLR_API_KEY_2
      fetch_limit: 50
      
  # Multiple output destinations
  - name: local-organized
    type: output
    plugin: local
    enabled: true
    settings:
      output_directory: "./media/organized"
      directory_structure: "{source}/{year}/{month}"
      filename_template: "{timestamp}_{original_name}"
      
  - name: backup-storage
    type: output
    plugin: local
    enabled: false  # Can be enabled for backup scenarios
    settings:
      output_directory: "./media/backup"
      compress: true

# Hot-reload configuration
hot_reload:
  enabled: true
  watch_paths:
    - "./configs"
    - "./plugins"
  debounce_ms: 500

# Plugin configuration
plugins:
  directory: "./plugins"
  auto_load: true
  hot_reload: true
  
storage:
  type: sqlite
  path: "./data/media-sync.db"
  backup_interval: "1h"
  
sync:
  interval: "5m"
  parallel_workers: 3
  retry_attempts: 3
  retry_delay: "30s"
  
logging:
  level: debug
  format: structured
  outputs:
    - type: console
    - type: file
      path: "./logs/media-sync.log"
      rotate: true
      max_size: "100MB"
      
security:
  keyring_service: media-sync
  encrypt_config: true
  auth_timeout: "10m"
EOF

echo "   ✅ Example configurations created"

# Create development documentation
echo "📚 Creating development documentation..."

cat > README.md << 'EOF'
# Media-Sync

A Go-based media synchronization platform with plugin-based architecture, built using Test-Driven Development (TDD).

## Quick Start

### Prerequisites
- Go 1.21+
- Make
- Git

### Setup Development Environment
```bash
# Clone and setup
git clone https://github.com/sho7650/media-sync.git
cd media-sync

# Initialize TDD environment (if using the init script)
./scripts/init-tdd-project.sh

# Or setup manually
make dev-setup
make deps
```

### TDD Workflow

This project follows strict Test-Driven Development:

1. **RED**: Write a failing test
2. **GREEN**: Write minimal code to pass
3. **REFACTOR**: Improve code while keeping tests green

```bash
# Start TDD cycle
make tdd-red     # Write failing test
make test-unit   # Confirm it fails
make tdd-green   # Write minimal implementation
make test-unit   # Confirm it passes
make tdd-refactor # Improve code quality
make test-unit lint # Ensure tests still pass
```

### Development Commands

```bash
# Testing
make test           # Run all tests
make test-unit      # Unit tests only
make test-watch     # Watch for changes and test
make coverage       # Generate coverage report

# Code Quality
make lint           # Run linter
make fmt            # Format code
make security       # Security scan

# Building
make build          # Build application
make build-plugins  # Build plugin binaries
make run            # Build and run

# Development
make dev-setup      # Setup dev environment
make generate-mocks # Generate test mocks
make clean          # Clean build artifacts
```

## Architecture

- **Plugin-based**: Input and output services as plugins
- **Hot-reload**: Configuration and plugin changes without restart
- **TDD-driven**: Every feature built test-first
- **Concurrent**: Parallel processing with goroutines

## Project Structure

```
├── cmd/media-sync/          # Main application entry
├── internal/                # Private application code
│   ├── core/               # Core interfaces and types
│   ├── config/             # Configuration management
│   ├── storage/            # Database and storage
│   ├── plugins/            # Plugin system
│   ├── sync/               # Synchronization logic
│   └── auth/               # Authentication
├── pkg/                    # Public API packages
├── plugins/                # Plugin implementations
├── test/                   # Test suites
├── configs/                # Configuration files
└── examples/               # TDD examples and templates
```

## Contributing

1. Create feature branch: `git checkout -b feature/my-feature`
2. Follow TDD workflow: Write test → Make it pass → Refactor
3. Ensure quality: `make ci-test`
4. Submit pull request

## TDD Examples

See `examples/tdd_examples.go` for concrete examples of:
- Interface testing
- Configuration hot-reload testing
- Database transaction testing
- Plugin system testing
- External API testing with mocks

## License

MIT License - see LICENSE file for details.
EOF

echo "   ✅ Documentation created"

# Install git hooks
echo "🪝 Installing git hooks..."
if [ -d ".git" ]; then
    make git-hooks > /dev/null 2>&1
    echo "   ✅ Git hooks installed"
else
    echo "   ⚠️  Git hooks skipped (not a git repository)"
fi

# Run initial setup
echo "🛠️  Running initial setup..."
make dev-setup > /dev/null 2>&1 || echo "   ⚠️  Some development tools may need manual installation"

# Build and test
echo "🧪 Running initial tests..."
if make test-unit > /dev/null 2>&1; then
    echo "   ✅ Initial tests passing"
else
    echo "   ⚠️  Initial tests need attention"
fi

# Final instructions
echo ""
echo "🎉 TDD Environment Setup Complete!"
echo ""
echo "Next Steps:"
echo "1. Review TDD_WORKFLOW.md for detailed implementation guide"
echo "2. Examine examples/tdd_examples.go for concrete TDD patterns"
echo "3. Start with Phase 1: Core Interfaces"
echo ""
echo "TDD Cycle Commands:"
echo "  make tdd-red      # Write failing test"
echo "  make test-unit    # Run tests"
echo "  make tdd-green    # Implement minimal code"
echo "  make tdd-refactor # Improve code quality"
echo ""
echo "Development Commands:"
echo "  make help         # Show all available commands"
echo "  make test-watch   # Continuous testing"
echo "  make dev-setup    # Setup development tools"
echo ""
echo "Ready to start TDD implementation! 🚀"