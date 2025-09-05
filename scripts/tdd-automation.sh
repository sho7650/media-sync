#!/bin/bash

# TDD Automation Script for Hot Reload Development (Cycle 2.3.1)
# Automates the complete TDD Red-Green-Refactor cycle with quality gates

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COVERAGE_THRESHOLD=80
HOTRELOAD_COVERAGE_THRESHOLD=95
MAX_COMPLEXITY=15
TIMEOUT_DURATION="5m"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${CYAN}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_phase() {
    echo -e "${PURPLE}[TDD-PHASE]${NC} $1"
}

# Utility functions
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing_tools=()
    
    # Check required tools
    command -v go >/dev/null 2>&1 || missing_tools+=("go")
    command -v git >/dev/null 2>&1 || missing_tools+=("git")
    command -v make >/dev/null 2>&1 || missing_tools+=("make")
    
    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        exit 1
    fi
    
    # Check Go version
    local go_version
    go_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
    if [[ "$(printf '%s\n' "1.25" "$go_version" | sort -V | head -n1)" != "1.25" ]]; then
        log_warning "Go version $go_version may not be fully compatible. Recommended: 1.25+"
    fi
    
    log_success "Prerequisites check complete"
}

detect_tdd_phase() {
    log_info "Detecting TDD phase from git history..."
    
    local last_commit
    last_commit=$(git log --oneline -1 2>/dev/null || echo "")
    
    if [[ "$last_commit" =~ (RED|red|failing|fail) ]]; then
        echo "red"
    elif [[ "$last_commit" =~ (GREEN|green|pass|implement) ]]; then
        echo "green"  
    elif [[ "$last_commit" =~ (REFACTOR|refactor|improve|optimize) ]]; then
        echo "refactor"
    else
        echo "unknown"
    fi
}

setup_environment() {
    log_info "Setting up development environment..."
    
    cd "$PROJECT_ROOT"
    
    # Ensure go.mod exists
    if [[ ! -f "go.mod" ]]; then
        log_error "go.mod not found. Please run from project root."
        exit 1
    fi
    
    # Download dependencies
    log_info "Downloading Go dependencies..."
    go mod download
    go mod verify
    
    # Install development tools if needed
    log_info "Installing development tools..."
    
    local tools=(
        "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
        "github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"
        "github.com/fzipp/gocyclo/cmd/gocyclo@latest"
    )
    
    for tool in "${tools[@]}"; do
        if ! command -v "$(basename "${tool%%@*}")" >/dev/null 2>&1; then
            log_info "Installing $(basename "${tool%%@*}")..."
            go install "$tool" || log_warning "Failed to install $tool"
        fi
    done
    
    log_success "Environment setup complete"
}

run_red_phase() {
    log_phase "🔴 RED PHASE: Running tests (expecting failures)"
    
    local test_result=0
    
    # Run tests and capture result
    if go test ./... -v -timeout="$TIMEOUT_DURATION" 2>&1; then
        log_warning "All tests passed in RED phase - ensure you're writing failing tests first!"
        log_info "This might indicate:"
        log_info "  - Tests are not comprehensive enough"
        log_info "  - Implementation already exists"
        log_info "  - Not following TDD properly"
        test_result=1
    else
        log_success "Tests failing as expected in RED phase"
        test_result=0
    fi
    
    # Hot reload specific test check
    log_info "Checking hot reload specific tests..."
    if go test ./internal/plugins/... -run=".*[Hh]ot[Rr]eload.*|.*[Ff]ile[Ww]atch.*" -v 2>&1; then
        log_warning "Hot reload tests passed - ensure you have failing tests for new functionality"
    else
        log_success "Hot reload tests failing as expected"
    fi
    
    return $test_result
}

run_green_phase() {
    log_phase "🟢 GREEN PHASE: Implementing minimal solution"
    
    # Check if hot reload dependencies are available
    log_info "Checking hot reload dependencies..."
    if ! go list -m github.com/fsnotify/fsnotify >/dev/null 2>&1; then
        log_info "Adding fsnotify dependency..."
        go get github.com/fsnotify/fsnotify@v1.7.0
        go mod tidy
    fi
    
    # Run tests to validate implementation
    log_info "Running tests to validate implementation..."
    if ! go test ./... -v -timeout="$TIMEOUT_DURATION"; then
        log_error "Tests still failing in GREEN phase"
        log_error "Implementation needs more work to make tests pass"
        return 1
    fi
    
    log_success "All tests passing in GREEN phase"
    
    # Specific hot reload test validation
    log_info "Validating hot reload functionality..."
    if ! go test ./internal/plugins/... -run=".*[Hh]ot[Rr]eload.*|.*[Ff]ile[Ww]atch.*" -v; then
        log_error "Hot reload tests still failing"
        return 1
    fi
    
    log_success "Hot reload tests passing"
    return 0
}

run_refactor_phase() {
    log_phase "🔵 REFACTOR PHASE: Improving code quality"
    
    local quality_issues=0
    
    # Ensure all tests still pass
    log_info "Ensuring all tests still pass..."
    if ! go test ./... -v -timeout="$TIMEOUT_DURATION"; then
        log_error "Tests failing after refactoring - fix before proceeding"
        return 1
    fi
    
    # Run code quality checks
    log_info "Running code quality checks..."
    
    # Format code
    log_info "Formatting code..."
    go fmt ./...
    
    # Run linting
    log_info "Running linting..."
    if command -v golangci-lint >/dev/null 2>&1; then
        if ! golangci-lint run --timeout=10m; then
            log_warning "Linting issues detected"
            quality_issues=$((quality_issues + 1))
        fi
    else
        log_warning "golangci-lint not available, skipping linting"
    fi
    
    # Check cyclomatic complexity
    log_info "Checking cyclomatic complexity..."
    if command -v gocyclo >/dev/null 2>&1; then
        local complex_funcs
        complex_funcs=$(gocyclo -over $MAX_COMPLEXITY . | wc -l)
        if [[ $complex_funcs -gt 0 ]]; then
            log_warning "Found $complex_funcs functions with high complexity (>$MAX_COMPLEXITY)"
            gocyclo -over $MAX_COMPLEXITY .
            quality_issues=$((quality_issues + 1))
        fi
    fi
    
    # Run security scan
    log_info "Running security scan..."
    if command -v gosec >/dev/null 2>&1; then
        if ! gosec ./... 2>/dev/null; then
            log_warning "Security issues detected"
            quality_issues=$((quality_issues + 1))
        fi
    else
        log_warning "gosec not available, skipping security scan"
    fi
    
    # Check test coverage
    log_info "Checking test coverage..."
    run_coverage_check || quality_issues=$((quality_issues + 1))
    
    if [[ $quality_issues -eq 0 ]]; then
        log_success "All quality checks passed"
        return 0
    else
        log_warning "$quality_issues quality issues detected"
        return 1
    fi
}

run_coverage_check() {
    log_info "Analyzing test coverage..."
    
    # Overall coverage
    go test -coverprofile=coverage.out ./... >/dev/null 2>&1
    
    if [[ -f "coverage.out" ]]; then
        local total_coverage
        total_coverage=$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}' | sed 's/%//')
        
        log_info "Total coverage: $total_coverage%"
        
        if (( $(echo "$total_coverage >= $COVERAGE_THRESHOLD" | bc -l) )); then
            log_success "Coverage threshold met: $total_coverage% >= $COVERAGE_THRESHOLD%"
        else
            log_error "Coverage below threshold: $total_coverage% < $COVERAGE_THRESHOLD%"
            return 1
        fi
    else
        log_error "Could not generate coverage report"
        return 1
    fi
    
    # Hot reload specific coverage
    log_info "Checking hot reload coverage..."
    if go test -coverprofile=hotreload-coverage.out ./internal/plugins/... >/dev/null 2>&1; then
        local hotreload_coverage
        hotreload_coverage=$(go tool cover -func=hotreload-coverage.out | tail -1 | awk '{print $3}' | sed 's/%//')
        
        log_info "Hot reload coverage: $hotreload_coverage%"
        
        if (( $(echo "$hotreload_coverage >= $HOTRELOAD_COVERAGE_THRESHOLD" | bc -l) )); then
            log_success "Hot reload coverage threshold met: $hotreload_coverage% >= $HOTRELOAD_COVERAGE_THRESHOLD%"
        else
            log_error "Hot reload coverage below threshold: $hotreload_coverage% < $HOTRELOAD_COVERAGE_THRESHOLD%"
            return 1
        fi
    else
        log_warning "Could not generate hot reload coverage report"
    fi
    
    return 0
}

run_integration_tests() {
    log_info "Running integration tests..."
    
    # Check if integration tests exist
    if [[ -d "tests/integration" ]]; then
        if ! go test -v ./tests/integration/... -timeout="$TIMEOUT_DURATION" -tags=integration; then
            log_error "Integration tests failed"
            return 1
        fi
        log_success "Integration tests passed"
    else
        log_info "No integration tests found, skipping"
    fi
    
    return 0
}

run_performance_check() {
    log_info "Running performance benchmarks..."
    
    # Run benchmarks for hot reload components
    if go test -bench=. -benchmem -run=^$ ./internal/plugins/... -benchtime=3s -timeout=5m > benchmark-results.txt 2>&1; then
        log_info "Benchmark results:"
        grep -E "Benchmark.*ops|ns/op" benchmark-results.txt | head -10 || true
        
        # Check for obvious performance issues
        if grep -q "ns/op.*[0-9][0-9][0-9][0-9][0-9]" benchmark-results.txt; then
            log_warning "Potential performance concern - operations taking >10μs detected"
        fi
        
        log_success "Performance benchmarking complete"
    else
        log_warning "Benchmark execution had issues, check benchmark-results.txt"
    fi
}

validate_hot_reload_functionality() {
    log_info "Validating hot reload specific functionality..."
    
    local validation_passed=true
    
    # Check for required interfaces
    if ! grep -r "FileWatcher\|PluginReloader\|HotReloadSystem" internal/plugins/ --include="*.go" >/dev/null 2>&1; then
        log_error "Hot reload interfaces not found"
        validation_passed=false
    fi
    
    # Check for fsnotify usage
    if ! grep -r "fsnotify" internal/plugins/ --include="*.go" >/dev/null 2>&1; then
        log_warning "fsnotify usage not detected - ensure file system watching is implemented"
        validation_passed=false
    fi
    
    # Check for event debouncing logic
    if ! grep -r "debounce\|Debounce" internal/plugins/ --include="*.go" >/dev/null 2>&1; then
        log_warning "Debouncing logic not detected - important for preventing reload spam"
    fi
    
    # Check for proper resource cleanup
    if ! grep -r "Close\|Stop\|Shutdown" internal/plugins/ --include="*.go" >/dev/null 2>&1; then
        log_warning "Resource cleanup methods not detected - important for preventing leaks"
    fi
    
    if $validation_passed; then
        log_success "Hot reload functionality validation passed"
        return 0
    else
        log_error "Hot reload functionality validation failed"
        return 1
    fi
}

generate_automation_report() {
    log_info "Generating automation report..."
    
    local report_file="tdd-automation-report.md"
    
    cat > "$report_file" << EOF
# TDD Automation Report - Hot Reload Cycle 2.3.1

Generated: $(date -u)
Commit: $(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
Branch: $(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

## Execution Summary

EOF
    
    # Add phase results if available
    if [[ -f ".tdd-phase-results" ]]; then
        cat .tdd-phase-results >> "$report_file"
    fi
    
    # Add coverage information
    if [[ -f "coverage.out" ]]; then
        echo "" >> "$report_file"
        echo "## Coverage Summary" >> "$report_file"
        echo '```' >> "$report_file"
        go tool cover -func=coverage.out | tail -5 >> "$report_file"
        echo '```' >> "$report_file"
    fi
    
    # Add benchmark results
    if [[ -f "benchmark-results.txt" ]]; then
        echo "" >> "$report_file"
        echo "## Performance Benchmarks" >> "$report_file"
        echo '```' >> "$report_file"
        head -20 benchmark-results.txt >> "$report_file"
        echo '```' >> "$report_file"
    fi
    
    echo "" >> "$report_file"
    echo "## Next Steps" >> "$report_file"
    echo "- Review any quality issues identified above" >> "$report_file"
    echo "- Ensure hot reload functionality is thoroughly tested" >> "$report_file"
    echo "- Consider performance optimizations for file system operations" >> "$report_file"
    echo "- Validate integration with existing plugin system" >> "$report_file"
    
    log_success "Automation report generated: $report_file"
}

cleanup() {
    log_info "Cleaning up temporary files..."
    rm -f coverage.out hotreload-coverage.out benchmark-results.txt .tdd-phase-results
}

# Main execution function
main() {
    log_info "Starting TDD Automation for Hot Reload Cycle 2.3.1"
    
    # Setup
    check_prerequisites
    setup_environment
    
    # Detect TDD phase or run specified phase
    local phase="${1:-auto}"
    
    if [[ "$phase" == "auto" ]]; then
        phase=$(detect_tdd_phase)
        log_info "Auto-detected TDD phase: $phase"
    fi
    
    # Execute based on phase
    local phase_result=0
    echo "Phase: $phase" > .tdd-phase-results
    
    case "$phase" in
        "red")
            run_red_phase || phase_result=$?
            echo "RED phase result: $(if [[ $phase_result -eq 0 ]]; then echo "✅ Pass"; else echo "❌ Fail"; fi)" >> .tdd-phase-results
            ;;
        "green")
            run_green_phase || phase_result=$?
            echo "GREEN phase result: $(if [[ $phase_result -eq 0 ]]; then echo "✅ Pass"; else echo "❌ Fail"; fi)" >> .tdd-phase-results
            ;;
        "refactor")
            run_refactor_phase || phase_result=$?
            echo "REFACTOR phase result: $(if [[ $phase_result -eq 0 ]]; then echo "✅ Pass"; else echo "❌ Fail"; fi)" >> .tdd-phase-results
            ;;
        "full")
            log_info "Running complete TDD cycle..."
            run_red_phase || true  # RED phase is allowed to fail
            run_green_phase || phase_result=$?
            run_refactor_phase || phase_result=$?
            ;;
        *)
            log_info "Running general validation..."
            run_green_phase || phase_result=$?
            run_refactor_phase || phase_result=$?
            ;;
    esac
    
    # Additional validations
    if [[ "$phase" =~ (green|refactor|full|unknown) ]]; then
        validate_hot_reload_functionality || phase_result=$?
        run_integration_tests || phase_result=$?
        run_performance_check || true  # Performance check is informational
    fi
    
    # Generate report
    generate_automation_report
    
    # Cleanup
    if [[ "${KEEP_ARTIFACTS:-}" != "true" ]]; then
        cleanup
    fi
    
    if [[ $phase_result -eq 0 ]]; then
        log_success "TDD Automation completed successfully"
    else
        log_error "TDD Automation completed with issues"
        exit 1
    fi
}

# Script usage information
show_usage() {
    cat << EOF
Usage: $0 [phase]

TDD Automation Script for Hot Reload Development (Cycle 2.3.1)

Phases:
  red       - Run RED phase (tests should fail)
  green     - Run GREEN phase (implement minimal solution)
  refactor  - Run REFACTOR phase (improve code quality)
  full      - Run complete TDD cycle
  auto      - Auto-detect phase from git history (default)

Environment Variables:
  COVERAGE_THRESHOLD              - Minimum coverage percentage (default: 80)
  HOTRELOAD_COVERAGE_THRESHOLD   - Hot reload coverage threshold (default: 95)
  MAX_COMPLEXITY                 - Maximum cyclomatic complexity (default: 15)
  TIMEOUT_DURATION               - Test timeout duration (default: 5m)
  KEEP_ARTIFACTS                 - Keep temporary files (default: false)

Examples:
  $0                    # Auto-detect phase and run
  $0 red               # Run RED phase specifically
  $0 full              # Run complete TDD cycle
  COVERAGE_THRESHOLD=85 $0 refactor  # Run refactor with higher coverage

EOF
}

# Handle script arguments
if [[ "${1:-}" == "--help" ]] || [[ "${1:-}" == "-h" ]]; then
    show_usage
    exit 0
fi

# Trap to ensure cleanup on exit
trap cleanup EXIT

# Execute main function
main "$@"