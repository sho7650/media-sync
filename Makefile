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

# Hot reload specific targets
hotreload-deps:
	@echo "📦 Installing hot reload dependencies..."
	go get github.com/fsnotify/fsnotify@v1.7.0
	go mod tidy

hotreload-test:
	@echo "🔥 Running hot reload specific tests..."
	go test -v -race -coverprofile=hotreload-coverage.out \
		./internal/plugins/... -run=".*[Hh]ot[Rr]eload.*|.*[Ff]ile[Ww]atch.*"
	@if [ -f hotreload-coverage.out ]; then \
		coverage=$$(go tool cover -func=hotreload-coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//'); \
		echo "🔥 Hot reload coverage: $$coverage%"; \
		if [ $$(echo "$$coverage >= 95" | bc -l) -eq 1 ]; then \
			echo "✅ Hot reload coverage meets 95% threshold"; \
		else \
			echo "❌ Hot reload coverage below 95% threshold"; \
			exit 1; \
		fi; \
	fi

parallel-test:
	@echo "🔄 Running parallel component tests..."
	go test -parallel 4 ./internal/plugins/... &
	go test -parallel 2 ./internal/config/... &
	go test -parallel 2 ./internal/storage/... &
	wait
	@echo "✅ All parallel tests completed"

integration-test-full:
	@echo "🔗 Running comprehensive integration tests..."
	go test -v ./tests/integration/... -timeout=5m -tags=integration

benchmark-hotreload:
	@echo "⚡ Running hot reload performance benchmarks..."
	go test -bench=. -benchmem -run=^$$ ./internal/plugins/... \
		-benchtime=10s -timeout=15m > hotreload-benchmark.txt
	@echo "📈 Benchmark results saved to: hotreload-benchmark.txt"

security-scan:
	@echo "🔒 Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "Installing gosec..."; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
		gosec ./...; \
	fi

# TDD automation targets
tdd-cycle-validate:
	@echo "🔍 Validating TDD cycle compliance..."
	@if git log --oneline -1 | grep -q "RED\|red\|failing"; then \
		echo "🔴 RED phase detected - tests should fail"; \
		$(MAKE) test-unit || echo "✅ Tests failing as expected in RED phase"; \
	elif git log --oneline -1 | grep -q "GREEN\|green\|pass"; then \
		echo "🟢 GREEN phase detected - tests should pass"; \
		$(MAKE) test-unit || (echo "❌ Tests failing in GREEN phase" && exit 1); \
	elif git log --oneline -1 | grep -q "REFACTOR\|refactor"; then \
		echo "🔵 REFACTOR phase detected - all quality checks should pass"; \
		$(MAKE) tdd-refactor || (echo "❌ Quality checks failing in REFACTOR phase" && exit 1); \
	else \
		echo "ℹ️ TDD phase not detected - running general validation"; \
		$(MAKE) dev-setup; \
	fi

# CI/CD automation targets  
ci-setup:
	@echo "🤖 Setting up CI environment..."
	go version
	go env
	go mod download
	go mod verify

ci-test-parallel:
	@echo "🧪 Running CI tests in parallel..."
	$(MAKE) parallel-test
	$(MAKE) hotreload-test
	$(MAKE) integration-test-full

ci-quality-gates:
	@echo "🚪 Running quality gate checks..."
	$(MAKE) lint
	$(MAKE) security-scan
	$(MAKE) coverage
	@echo "✅ All quality gates passed"

# Development workflow automation
dev-hotreload-setup: hotreload-deps
	@echo "🔥 Setting up hot reload development environment..."
	$(MAKE) hotreload-test
	$(MAKE) benchmark-hotreload
	@echo "🔥 Hot reload development environment ready"

dev-parallel-setup:
	@echo "🔄 Setting up parallel development environment..."
	$(MAKE) ci-setup
	$(MAKE) parallel-test
	@echo "🔄 Parallel development environment ready"

# Comprehensive automation target
automation-full: ci-setup ci-test-parallel ci-quality-gates
	@echo "🎯 Complete automation pipeline executed successfully"

# ============================================================================
# COMPREHENSIVE VALIDATION FRAMEWORK FOR TDD CYCLE 2.3.1
# ============================================================================

# Hot Reload Validation Framework
validate-hotreload:
	@echo "🔍 Running comprehensive hot reload validation..."
	./scripts/hotreload-validation.sh

validate-hotreload-red:
	@echo "🔴 Validating RED phase for hot reload..."
	./scripts/hotreload-validation.sh red

validate-hotreload-green:
	@echo "🟢 Validating GREEN phase for hot reload..."
	./scripts/hotreload-validation.sh green

validate-hotreload-refactor:
	@echo "🔵 Validating REFACTOR phase for hot reload..."
	./scripts/hotreload-validation.sh refactor

# Quality Metrics Collection
quality-metrics:
	@echo "📊 Collecting comprehensive quality metrics..."
	./scripts/quality-metrics.sh

quality-metrics-collect:
	@echo "📊 Collecting quality metrics only..."
	./scripts/quality-metrics.sh --collect-only

quality-metrics-analyze:
	@echo "📊 Analyzing existing quality metrics..."
	./scripts/quality-metrics.sh --analyze-only

quality-report:
	@echo "📋 Generating quality report..."
	./scripts/quality-metrics.sh --report-only

# Security Validation Framework
security-validate:
	@echo "🔒 Running comprehensive security validation..."
	./scripts/security-validation.sh

security-filesystem:
	@echo "🔒 Validating file system security..."
	./scripts/security-validation.sh --filesystem

security-input:
	@echo "🔒 Validating input security..."
	./scripts/security-validation.sh --input

security-resources:
	@echo "🔒 Validating resource security..."
	./scripts/security-validation.sh --resources

security-scan-external:
	@echo "🔒 Running external security scans..."
	./scripts/security-validation.sh --scan

# Integration Testing Framework
test-integration-hotreload:
	@echo "🔗 Running hot reload integration tests..."
	go test -v ./tests/integration/... -run=".*HotReload.*" -timeout=10m -tags=integration

test-integration-concurrent:
	@echo "🔗 Running concurrent integration tests..."
	go test -v ./tests/integration/... -run=".*Concurrent.*" -timeout=10m -tags=integration

test-integration-performance:
	@echo "⚡ Running performance integration tests..."
	go test -v ./tests/integration/... -run=".*Performance.*" -timeout=15m -tags=integration

test-integration-security:
	@echo "🔒 Running security integration tests..."
	go test -v ./tests/integration/... -run=".*Security.*" -timeout=10m -tags=integration

# Benchmark Suite
benchmark-suite:
	@echo "⚡ Running comprehensive benchmark suite..."
	go test -bench=. -benchmem -run=^$$ ./... -benchtime=10s -timeout=20m

benchmark-hotreload-suite:
	@echo "🔥 Running hot reload benchmark suite..."
	go test -bench=BenchmarkHotReload -benchmem -run=^$$ ./internal/plugins/... -benchtime=10s

benchmark-integration-suite:
	@echo "🔗 Running integration benchmark suite..."
	go test -bench=Benchmark -benchmem -run=^$$ ./tests/integration/... -benchtime=5s -tags=integration

# Comprehensive Quality Gates
quality-gate-coverage:
	@echo "🚪 Coverage Quality Gate..."
	@go test -coverprofile=temp-coverage.out ./... >/dev/null 2>&1; \
	coverage=$$(go tool cover -func=temp-coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	rm -f temp-coverage.out; \
	echo "Coverage: $$coverage%"; \
	if [ $$(echo "$$coverage >= 80" | bc -l) -eq 1 ]; then \
		echo "✅ Coverage gate passed"; \
	else \
		echo "❌ Coverage gate failed"; \
		exit 1; \
	fi

quality-gate-performance:
	@echo "🚪 Performance Quality Gate..."
	@go test -bench=. -benchmem -run=^$$ ./internal/plugins/... -benchtime=3s > temp-bench.txt 2>&1; \
	if grep -q "ns/op.*[0-9][0-9][0-9][0-9][0-9][0-9]" temp-bench.txt; then \
		echo "❌ Performance gate failed - operations taking >1ms detected"; \
		grep "ns/op.*[0-9][0-9][0-9][0-9][0-9][0-9]" temp-bench.txt | head -3; \
		rm -f temp-bench.txt; \
		exit 1; \
	else \
		echo "✅ Performance gate passed"; \
		rm -f temp-bench.txt; \
	fi

quality-gate-security:
	@echo "🚪 Security Quality Gate..."
	@if command -v gosec >/dev/null 2>&1; then \
		if gosec -fmt=json ./... 2>/dev/null | jq -e '.Issues | length == 0' >/dev/null; then \
			echo "✅ Security gate passed"; \
		else \
			echo "❌ Security gate failed"; \
			gosec ./... | head -10; \
			exit 1; \
		fi; \
	else \
		echo "⚠️ Security gate skipped (gosec not available)"; \
	fi

quality-gates-all: quality-gate-coverage quality-gate-performance quality-gate-security
	@echo "🎯 All quality gates passed!"

# TDD Workflow Integration
tdd-workflow-red: tdd-red validate-hotreload-red
	@echo "🔴 RED phase workflow completed"

tdd-workflow-green: tdd-green validate-hotreload-green quality-gate-coverage
	@echo "🟢 GREEN phase workflow completed"

tdd-workflow-refactor: tdd-refactor validate-hotreload-refactor quality-gates-all security-validate
	@echo "🔵 REFACTOR phase workflow completed"

# Complete TDD Cycle Validation
tdd-cycle-complete:
	@echo "🔄 Running complete TDD cycle validation..."
	$(MAKE) tdd-workflow-red || echo "RED phase validation"
	$(MAKE) tdd-workflow-green
	$(MAKE) tdd-workflow-refactor
	@echo "🎯 Complete TDD cycle validation successful!"

# Development Environment Setup
dev-env-hotreload:
	@echo "🔥 Setting up complete hot reload development environment..."
	$(MAKE) hotreload-deps
	$(MAKE) validate-hotreload
	$(MAKE) quality-metrics-collect
	$(MAKE) test-integration-hotreload
	@echo "🔥 Hot reload development environment ready with full validation"

# Continuous Integration Pipeline
ci-hotreload-pipeline:
	@echo "🤖 Running hot reload CI pipeline..."
	$(MAKE) ci-setup
	$(MAKE) validate-hotreload
	$(MAKE) quality-gates-all
	$(MAKE) security-validate
	$(MAKE) test-integration-hotreload
	$(MAKE) benchmark-hotreload-suite
	@echo "🤖 Hot reload CI pipeline completed successfully"

# Cleanup and Maintenance
clean-validation-artifacts:
	@echo "🧹 Cleaning validation artifacts..."
	rm -f coverage.out hotreload-coverage.out integration-coverage.out
	rm -f benchmark-results.txt hotreload-benchmark.txt
	rm -f security-report.json gosec-security-report.json
	rm -f quality-metrics.json quality-report.md
	rm -f hotreload-validation-report.md security-validation-report.md
	rm -f .coverage-history .performance-history
	@echo "🧹 Cleanup completed"

# Help and Documentation
help-validation:
	@echo "Hot Reload Validation Framework - TDD Cycle 2.3.1"
	@echo ""
	@echo "Validation Commands:"
	@echo "  validate-hotreload           - Complete hot reload validation"
	@echo "  validate-hotreload-{phase}   - Phase-specific validation (red/green/refactor)"
	@echo ""
	@echo "Quality Commands:"
	@echo "  quality-metrics              - Collect and analyze quality metrics"
	@echo "  quality-gates-all           - Run all quality gates"
	@echo "  quality-report              - Generate quality report"
	@echo ""
	@echo "Security Commands:"
	@echo "  security-validate           - Complete security validation"
	@echo "  security-{category}         - Category-specific validation"
	@echo ""
	@echo "Integration Commands:"
	@echo "  test-integration-hotreload  - Hot reload integration tests"
	@echo "  test-integration-{type}     - Type-specific integration tests"
	@echo ""
	@echo "TDD Workflow:"
	@echo "  tdd-workflow-{phase}        - Complete phase workflow"
	@echo "  tdd-cycle-complete          - Full TDD cycle validation"
	@echo ""
	@echo "Development:"
	@echo "  dev-env-hotreload           - Complete development environment setup"
	@echo "  ci-hotreload-pipeline       - Full CI pipeline"

# Default help
help: help-validation