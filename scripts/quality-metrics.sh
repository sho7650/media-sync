#!/bin/bash

# Quality Metrics Collection and Analysis Script for Hot Reload TDD Cycle 2.3.1
# Provides automated collection and analysis of quality metrics throughout development

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Metric thresholds
COVERAGE_TARGET=95
PERFORMANCE_TARGET_NS=1000000  # 1ms in nanoseconds
COMPLEXITY_TARGET=10
SECURITY_TARGET=0
MAINTAINABILITY_TARGET=8

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Metric storage
declare -A METRICS
METRICS_FILE="quality-metrics.json"

log_metric() {
    echo -e "${CYAN}[METRIC]${NC} $1"
}

log_analysis() {
    echo -e "${PURPLE}[ANALYSIS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# ============================================================================
# COVERAGE METRICS
# ============================================================================

collect_coverage_metrics() {
    log_metric "Collecting coverage metrics..."
    
    cd "$PROJECT_ROOT"
    
    # Overall project coverage
    go test -coverprofile=coverage.out ./... >/dev/null 2>&1
    local overall_coverage=0
    if [[ -f "coverage.out" ]]; then
        overall_coverage=$(go tool cover -func=coverage.out | tail -1 | awk '{print $3}' | sed 's/%//')
        METRICS["overall_coverage"]=$overall_coverage
    fi
    
    # Hot reload specific coverage
    go test -coverprofile=hotreload-coverage.out ./internal/plugins/... >/dev/null 2>&1
    local hotreload_coverage=0
    if [[ -f "hotreload-coverage.out" ]]; then
        hotreload_coverage=$(go tool cover -func=hotreload-coverage.out | tail -1 | awk '{print $3}' | sed 's/%//')
        METRICS["hotreload_coverage"]=$hotreload_coverage
    fi
    
    # Line-by-line coverage analysis
    local uncovered_lines=0
    if [[ -f "hotreload-coverage.out" ]]; then
        uncovered_lines=$(go tool cover -func=hotreload-coverage.out | grep -c ":.*0.0%" || echo "0")
        METRICS["uncovered_functions"]=$uncovered_lines
    fi
    
    # Integration test coverage
    if [[ -d "tests/integration" ]]; then
        go test -coverprofile=integration-coverage.out ./tests/integration/... >/dev/null 2>&1 || true
        local integration_coverage=0
        if [[ -f "integration-coverage.out" ]]; then
            integration_coverage=$(go tool cover -func=integration-coverage.out | tail -1 | awk '{print $3}' | sed 's/%//')
            METRICS["integration_coverage"]=$integration_coverage
        fi
    fi
    
    log_metric "Overall coverage: ${overall_coverage}%"
    log_metric "Hot reload coverage: ${hotreload_coverage}%"
    log_metric "Uncovered functions: $uncovered_lines"
}

analyze_coverage_quality() {
    log_analysis "Analyzing coverage quality..."
    
    local overall=${METRICS["overall_coverage"]:-0}
    local hotreload=${METRICS["hotreload_coverage"]:-0}
    
    # Coverage trend analysis (compare with historical data)
    local coverage_trend="stable"
    if [[ -f ".coverage-history" ]]; then
        local prev_coverage
        prev_coverage=$(tail -1 .coverage-history | cut -d',' -f2)
        if (( $(echo "$hotreload > $prev_coverage + 5" | bc -l) )); then
            coverage_trend="improving"
        elif (( $(echo "$hotreload < $prev_coverage - 5" | bc -l) )); then
            coverage_trend="declining"
        fi
    fi
    
    # Record current coverage
    echo "$(date -u),${hotreload}" >> .coverage-history
    
    METRICS["coverage_trend"]=$coverage_trend
    
    # Quality assessment
    if (( $(echo "$hotreload >= $COVERAGE_TARGET" | bc -l) )); then
        log_success "Hot reload coverage meets target (${hotreload}% >= ${COVERAGE_TARGET}%)"
        METRICS["coverage_quality"]="excellent"
    elif (( $(echo "$hotreload >= $COVERAGE_TARGET - 10" | bc -l) )); then
        log_warning "Hot reload coverage near target (${hotreload}% vs ${COVERAGE_TARGET}%)"
        METRICS["coverage_quality"]="good"
    else
        log_error "Hot reload coverage below target (${hotreload}% vs ${COVERAGE_TARGET}%)"
        METRICS["coverage_quality"]="poor"
    fi
    
    # Identify coverage gaps
    identify_coverage_gaps
}

identify_coverage_gaps() {
    log_analysis "Identifying coverage gaps..."
    
    if [[ -f "hotreload-coverage.out" ]]; then
        local gaps
        gaps=$(go tool cover -func=hotreload-coverage.out | grep -E ":.*[0-4][0-9]\.[0-9]%" | head -5)
        
        if [[ -n "$gaps" ]]; then
            log_warning "Functions with low coverage:"
            echo "$gaps" | while read -r line; do
                echo "  $line"
            done
        fi
        
        # Count critical gaps (functions with 0% coverage)
        local critical_gaps
        critical_gaps=$(echo "$gaps" | grep -c "0.0%" || echo "0")
        METRICS["critical_coverage_gaps"]=$critical_gaps
    fi
}

# ============================================================================
# PERFORMANCE METRICS
# ============================================================================

collect_performance_metrics() {
    log_metric "Collecting performance metrics..."
    
    cd "$PROJECT_ROOT"
    
    # Run benchmarks for hot reload components
    local benchmark_file="benchmark-results.txt"
    go test -bench=BenchmarkHotReload -benchmem -run=^$ ./internal/plugins/... \
        -benchtime=5s -timeout=10m > "$benchmark_file" 2>&1 || true
    
    if [[ ! -f "$benchmark_file" ]]; then
        log_warning "No benchmark results available"
        return
    fi
    
    # Extract performance metrics
    local avg_ns_per_op=0
    local avg_allocs_per_op=0
    local avg_bytes_per_op=0
    
    # Parse benchmark results
    while read -r line; do
        if [[ "$line" =~ BenchmarkHotReload.*-[0-9]+[[:space:]]+([0-9]+)[[:space:]]+([0-9]+)[[:space:]]+ns/op ]]; then
            local ops="${BASH_REMATCH[1]}"
            local ns="${BASH_REMATCH[2]}"
            avg_ns_per_op=$((avg_ns_per_op + ns))
            
            # Extract allocation info if present
            if [[ "$line" =~ ([0-9]+)[[:space:]]+allocs/op ]]; then
                local allocs="${BASH_REMATCH[1]}"
                avg_allocs_per_op=$((avg_allocs_per_op + allocs))
            fi
            
            if [[ "$line" =~ ([0-9]+)[[:space:]]+B/op ]]; then
                local bytes="${BASH_REMATCH[1]}"
                avg_bytes_per_op=$((avg_bytes_per_op + bytes))
            fi
        fi
    done < "$benchmark_file"
    
    METRICS["avg_ns_per_op"]=$avg_ns_per_op
    METRICS["avg_allocs_per_op"]=$avg_allocs_per_op
    METRICS["avg_bytes_per_op"]=$avg_bytes_per_op
    
    log_metric "Average ns/op: $avg_ns_per_op"
    log_metric "Average allocs/op: $avg_allocs_per_op"
    log_metric "Average B/op: $avg_bytes_per_op"
    
    # Memory usage analysis
    collect_memory_metrics
    
    # File system performance
    collect_filesystem_metrics
}

collect_memory_metrics() {
    log_metric "Collecting memory metrics..."
    
    # Analyze memory allocation patterns
    if [[ -f "benchmark-results.txt" ]]; then
        local high_alloc_benchmarks
        high_alloc_benchmarks=$(grep -c "allocs/op.*[0-9][0-9][0-9]" benchmark-results.txt || echo "0")
        METRICS["high_alloc_operations"]=$high_alloc_benchmarks
        
        local total_memory_usage
        total_memory_usage=$(grep "B/op" benchmark-results.txt | awk '{sum += $4} END {print sum}')
        METRICS["total_benchmark_memory"]=${total_memory_usage:-0}
    fi
}

collect_filesystem_metrics() {
    log_metric "Collecting file system performance metrics..."
    
    # Test file I/O performance
    local test_dir
    test_dir=$(mktemp -d)
    local test_file="$test_dir/perf_test.json"
    
    # Measure file write performance
    local write_start
    write_start=$(date +%s%N)
    for i in {1..100}; do
        echo '{"test": "data", "iteration": '$i'}' > "$test_file"
    done
    local write_end
    write_end=$(date +%s%N)
    local write_time_ns=$((write_end - write_start))
    
    # Measure file read performance
    local read_start
    read_start=$(date +%s%N)
    for i in {1..100}; do
        cat "$test_file" > /dev/null
    done
    local read_end
    read_end=$(date +%s%N)
    local read_time_ns=$((read_end - read_start))
    
    METRICS["file_write_perf_ns"]=$write_time_ns
    METRICS["file_read_perf_ns"]=$read_time_ns
    
    rm -rf "$test_dir"
    
    log_metric "File write performance: ${write_time_ns}ns for 100 operations"
    log_metric "File read performance: ${read_time_ns}ns for 100 operations"
}

analyze_performance_quality() {
    log_analysis "Analyzing performance quality..."
    
    local avg_ns=${METRICS["avg_ns_per_op"]:-0}
    local high_allocs=${METRICS["high_alloc_operations"]:-0}
    
    # Performance trend analysis
    local perf_trend="stable"
    if [[ -f ".performance-history" ]]; then
        local prev_perf
        prev_perf=$(tail -1 .performance-history | cut -d',' -f2)
        if (( avg_ns < prev_perf - 100000 )); then
            perf_trend="improving"
        elif (( avg_ns > prev_perf + 100000 )); then
            perf_trend="declining"
        fi
    fi
    
    # Record current performance
    echo "$(date -u),$avg_ns" >> .performance-history
    
    METRICS["performance_trend"]=$perf_trend
    
    # Quality assessment
    if (( avg_ns <= PERFORMANCE_TARGET_NS )); then
        log_success "Performance meets target (${avg_ns}ns <= ${PERFORMANCE_TARGET_NS}ns)"
        METRICS["performance_quality"]="excellent"
    elif (( avg_ns <= PERFORMANCE_TARGET_NS * 2 )); then
        log_warning "Performance near target (${avg_ns}ns vs ${PERFORMANCE_TARGET_NS}ns)"
        METRICS["performance_quality"]="acceptable"
    else
        log_error "Performance below target (${avg_ns}ns vs ${PERFORMANCE_TARGET_NS}ns)"
        METRICS["performance_quality"]="poor"
    fi
    
    # Memory allocation analysis
    if (( high_allocs > 5 )); then
        log_warning "$high_allocs operations have high memory allocations"
        METRICS["memory_efficiency"]="poor"
    elif (( high_allocs > 0 )); then
        log_warning "$high_allocs operations have moderate memory allocations"
        METRICS["memory_efficiency"]="acceptable"
    else
        log_success "Memory allocation efficiency is good"
        METRICS["memory_efficiency"]="excellent"
    fi
}

# ============================================================================
# CODE QUALITY METRICS
# ============================================================================

collect_code_quality_metrics() {
    log_metric "Collecting code quality metrics..."
    
    cd "$PROJECT_ROOT"
    
    # Cyclomatic complexity
    if command -v gocyclo >/dev/null 2>&1; then
        local complex_funcs
        complex_funcs=$(gocyclo -over $COMPLEXITY_TARGET ./internal/plugins | wc -l)
        METRICS["high_complexity_functions"]=$complex_funcs
        
        local avg_complexity
        avg_complexity=$(gocyclo ./internal/plugins | awk '{sum += $1; count++} END {print (count > 0) ? sum/count : 0}')
        METRICS["average_complexity"]=${avg_complexity:-0}
    fi
    
    # Lines of code metrics
    local total_lines
    total_lines=$(find ./internal/plugins -name "*.go" -not -name "*_test.go" | xargs wc -l | tail -1 | awk '{print $1}')
    METRICS["total_lines_of_code"]=$total_lines
    
    local test_lines
    test_lines=$(find ./internal/plugins -name "*_test.go" | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}' || echo "0")
    METRICS["total_test_lines"]=$test_lines
    
    # Test to code ratio
    local test_ratio=0
    if [[ $total_lines -gt 0 ]]; then
        test_ratio=$(echo "scale=2; $test_lines / $total_lines * 100" | bc)
    fi
    METRICS["test_to_code_ratio"]=$test_ratio
    
    # Comment density
    local comment_lines
    comment_lines=$(find ./internal/plugins -name "*.go" | xargs grep -c "^[[:space:]]*//\|^[[:space:]]*/\*" | awk -F: '{sum += $2} END {print sum}' || echo "0")
    METRICS["comment_lines"]=$comment_lines
    
    local comment_density=0
    if [[ $total_lines -gt 0 ]]; then
        comment_density=$(echo "scale=2; $comment_lines / $total_lines * 100" | bc)
    fi
    METRICS["comment_density"]=$comment_density
    
    log_metric "Total lines of code: $total_lines"
    log_metric "Total test lines: $test_lines"
    log_metric "Test to code ratio: ${test_ratio}%"
    log_metric "Comment density: ${comment_density}%"
}

analyze_code_quality() {
    log_analysis "Analyzing code quality..."
    
    local complex_funcs=${METRICS["high_complexity_functions"]:-0}
    local test_ratio=${METRICS["test_to_code_ratio"]:-0}
    local comment_density=${METRICS["comment_density"]:-0}
    
    # Complexity assessment
    if [[ $complex_funcs -eq 0 ]]; then
        log_success "All functions meet complexity target"
        METRICS["complexity_quality"]="excellent"
    elif [[ $complex_funcs -le 3 ]]; then
        log_warning "$complex_funcs functions exceed complexity target"
        METRICS["complexity_quality"]="acceptable"
    else
        log_error "$complex_funcs functions exceed complexity target"
        METRICS["complexity_quality"]="poor"
    fi
    
    # Test coverage assessment
    if (( $(echo "$test_ratio >= 100" | bc -l) )); then
        log_success "Excellent test coverage ratio (${test_ratio}%)"
        METRICS["test_ratio_quality"]="excellent"
    elif (( $(echo "$test_ratio >= 70" | bc -l) )); then
        log_success "Good test coverage ratio (${test_ratio}%)"
        METRICS["test_ratio_quality"]="good"
    else
        log_warning "Low test coverage ratio (${test_ratio}%)"
        METRICS["test_ratio_quality"]="poor"
    fi
    
    # Documentation assessment
    if (( $(echo "$comment_density >= 20" | bc -l) )); then
        log_success "Good documentation density (${comment_density}%)"
        METRICS["documentation_quality"]="excellent"
    elif (( $(echo "$comment_density >= 10" | bc -l) )); then
        log_warning "Moderate documentation density (${comment_density}%)"
        METRICS["documentation_quality"]="acceptable"
    else
        log_warning "Low documentation density (${comment_density}%)"
        METRICS["documentation_quality"]="poor"
    fi
}

# ============================================================================
# SECURITY METRICS
# ============================================================================

collect_security_metrics() {
    log_metric "Collecting security metrics..."
    
    cd "$PROJECT_ROOT"
    
    # Install gosec if needed
    if ! command -v gosec >/dev/null 2>&1; then
        log_metric "Installing gosec security scanner..."
        go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
    fi
    
    # Run security scan
    local security_report="security-scan.json"
    gosec -fmt=json -out="$security_report" ./... 2>/dev/null || true
    
    if [[ -f "$security_report" ]]; then
        # Parse security results
        local total_issues
        total_issues=$(jq '.Issues | length' "$security_report" 2>/dev/null || echo "0")
        METRICS["total_security_issues"]=$total_issues
        
        local high_severity
        high_severity=$(jq '[.Issues[] | select(.severity == "HIGH")] | length' "$security_report" 2>/dev/null || echo "0")
        METRICS["high_severity_issues"]=$high_severity
        
        local medium_severity
        medium_severity=$(jq '[.Issues[] | select(.severity == "MEDIUM")] | length' "$security_report" 2>/dev/null || echo "0")
        METRICS["medium_severity_issues"]=$medium_severity
        
        log_metric "Total security issues: $total_issues"
        log_metric "High severity: $high_severity"
        log_metric "Medium severity: $medium_severity"
    fi
    
    # Check for security best practices
    check_security_patterns
}

check_security_patterns() {
    log_metric "Checking security patterns..."
    
    # Check for path traversal protection
    local path_cleaning
    path_cleaning=$(grep -r "filepath\.Clean\|filepath\.Abs" ./internal/plugins --include="*.go" | wc -l)
    METRICS["path_cleaning_usage"]=$path_cleaning
    
    # Check for input validation
    local input_validation
    input_validation=$(grep -r "strings\.Contains\|regexp\.Match\|validation\." ./internal/plugins --include="*.go" | wc -l)
    METRICS["input_validation_patterns"]=$input_validation
    
    # Check for error handling
    local error_handling
    error_handling=$(grep -r "if err != nil\|errors\.New\|fmt\.Errorf" ./internal/plugins --include="*.go" | wc -l)
    METRICS["error_handling_patterns"]=$error_handling
    
    log_metric "Path cleaning patterns: $path_cleaning"
    log_metric "Input validation patterns: $input_validation"
    log_metric "Error handling patterns: $error_handling"
}

analyze_security_quality() {
    log_analysis "Analyzing security quality..."
    
    local total_issues=${METRICS["total_security_issues"]:-0}
    local high_severity=${METRICS["high_severity_issues"]:-0}
    
    # Security assessment
    if [[ $total_issues -eq 0 ]]; then
        log_success "No security issues detected"
        METRICS["security_quality"]="excellent"
    elif [[ $high_severity -eq 0 && $total_issues -le 3 ]]; then
        log_warning "$total_issues low-severity security issues detected"
        METRICS["security_quality"]="acceptable"
    else
        log_error "$total_issues security issues detected (${high_severity} high severity)"
        METRICS["security_quality"]="poor"
    fi
    
    # Pattern analysis
    local path_cleaning=${METRICS["path_cleaning_usage"]:-0}
    local input_validation=${METRICS["input_validation_patterns"]:-0}
    
    if [[ $path_cleaning -gt 0 ]]; then
        log_success "Path traversal protection implemented"
    else
        log_warning "Path traversal protection not detected"
    fi
    
    if [[ $input_validation -gt 5 ]]; then
        log_success "Input validation patterns detected"
    else
        log_warning "Limited input validation detected"
    fi
}

# ============================================================================
# MAINTAINABILITY METRICS
# ============================================================================

collect_maintainability_metrics() {
    log_metric "Collecting maintainability metrics..."
    
    cd "$PROJECT_ROOT"
    
    # Function count and size analysis
    local total_functions
    total_functions=$(grep -r "^func " ./internal/plugins --include="*.go" | wc -l)
    METRICS["total_functions"]=$total_functions
    
    # Average function length
    local avg_function_length=0
    local function_files
    function_files=$(find ./internal/plugins -name "*.go" -not -name "*_test.go")
    
    if [[ -n "$function_files" ]]; then
        local total_func_lines=0
        local func_count=0
        
        for file in $function_files; do
            while read -r line; do
                if [[ "$line" =~ ^func ]]; then
                    local line_count=0
                    local brace_count=0
                    local started=false
                    
                    while read -r func_line; do
                        if [[ "$func_line" == *"{"* ]]; then
                            started=true
                            brace_count=$((brace_count + 1))
                        fi
                        if [[ "$started" == "true" ]]; then
                            line_count=$((line_count + 1))
                        fi
                        if [[ "$func_line" == *"}"* ]] && [[ "$started" == "true" ]]; then
                            brace_count=$((brace_count - 1))
                            if [[ $brace_count -eq 0 ]]; then
                                break
                            fi
                        fi
                    done <<< "$(tail -n +$(grep -n "$line" "$file" | cut -d: -f1) "$file")"
                    
                    total_func_lines=$((total_func_lines + line_count))
                    func_count=$((func_count + 1))
                fi
            done < "$file"
        done
        
        if [[ $func_count -gt 0 ]]; then
            avg_function_length=$((total_func_lines / func_count))
        fi
    fi
    
    METRICS["average_function_length"]=$avg_function_length
    
    # Package coupling analysis
    local import_statements
    import_statements=$(grep -r "^import\|^\timport" ./internal/plugins --include="*.go" | wc -l)
    METRICS["import_statements"]=$import_statements
    
    # Interface usage
    local interface_count
    interface_count=$(grep -r "type.*interface" ./internal/plugins --include="*.go" | wc -l)
    METRICS["interface_count"]=$interface_count
    
    log_metric "Total functions: $total_functions"
    log_metric "Average function length: $avg_function_length lines"
    log_metric "Import statements: $import_statements"
    log_metric "Interface count: $interface_count"
}

analyze_maintainability() {
    log_analysis "Analyzing maintainability..."
    
    local avg_func_length=${METRICS["average_function_length"]:-0}
    local interface_count=${METRICS["interface_count"]:-0}
    local total_functions=${METRICS["total_functions"]:-1}
    
    # Function size assessment
    if [[ $avg_func_length -le 20 ]]; then
        log_success "Functions are appropriately sized (avg: ${avg_func_length} lines)"
        METRICS["function_size_quality"]="excellent"
    elif [[ $avg_func_length -le 40 ]]; then
        log_warning "Functions are moderately sized (avg: ${avg_func_length} lines)"
        METRICS["function_size_quality"]="acceptable"
    else
        log_error "Functions are too large (avg: ${avg_func_length} lines)"
        METRICS["function_size_quality"]="poor"
    fi
    
    # Interface design assessment
    local interface_ratio
    interface_ratio=$(echo "scale=2; $interface_count / $total_functions * 100" | bc)
    
    if (( $(echo "$interface_ratio >= 10" | bc -l) )); then
        log_success "Good interface usage (${interface_ratio}%)"
        METRICS["interface_design_quality"]="excellent"
    elif (( $(echo "$interface_ratio >= 5" | bc -l) )); then
        log_warning "Moderate interface usage (${interface_ratio}%)"
        METRICS["interface_design_quality"]="acceptable"
    else
        log_warning "Low interface usage (${interface_ratio}%)"
        METRICS["interface_design_quality"]="poor"
    fi
}

# ============================================================================
# OVERALL QUALITY SCORE CALCULATION
# ============================================================================

calculate_overall_quality_score() {
    log_analysis "Calculating overall quality score..."
    
    # Quality score weights
    local coverage_weight=25
    local performance_weight=20
    local security_weight=25
    local maintainability_weight=20
    local code_quality_weight=10
    
    # Convert quality ratings to numeric scores
    local coverage_score=0
    local performance_score=0
    local security_score=0
    local maintainability_score=0
    local code_quality_score=0
    
    # Coverage score
    case "${METRICS["coverage_quality"]:-poor}" in
        excellent) coverage_score=10 ;;
        good) coverage_score=8 ;;
        acceptable) coverage_score=6 ;;
        *) coverage_score=3 ;;
    esac
    
    # Performance score
    case "${METRICS["performance_quality"]:-poor}" in
        excellent) performance_score=10 ;;
        acceptable) performance_score=6 ;;
        *) performance_score=3 ;;
    esac
    
    # Security score
    case "${METRICS["security_quality"]:-poor}" in
        excellent) security_score=10 ;;
        acceptable) security_score=6 ;;
        *) security_score=2 ;;
    esac
    
    # Maintainability score
    local func_quality="${METRICS["function_size_quality"]:-poor}"
    local interface_quality="${METRICS["interface_design_quality"]:-poor}"
    
    local func_score=0
    local interface_score=0
    
    case "$func_quality" in
        excellent) func_score=5 ;;
        acceptable) func_score=3 ;;
        *) func_score=1 ;;
    esac
    
    case "$interface_quality" in
        excellent) interface_score=5 ;;
        acceptable) interface_score=3 ;;
        *) interface_score=1 ;;
    esac
    
    maintainability_score=$((func_score + interface_score))
    
    # Code quality score
    case "${METRICS["complexity_quality"]:-poor}" in
        excellent) code_quality_score=10 ;;
        acceptable) code_quality_score=6 ;;
        *) code_quality_score=3 ;;
    esac
    
    # Calculate weighted overall score
    local total_score
    total_score=$(echo "scale=1; \
        ($coverage_score * $coverage_weight + \
         $performance_score * $performance_weight + \
         $security_score * $security_weight + \
         $maintainability_score * $maintainability_weight + \
         $code_quality_score * $code_quality_weight) / 100" | bc)
    
    METRICS["overall_quality_score"]=$total_score
    
    # Quality rating
    if (( $(echo "$total_score >= 8.5" | bc -l) )); then
        METRICS["overall_quality_rating"]="Excellent"
        log_success "Overall Quality Score: ${total_score}/10 (Excellent)"
    elif (( $(echo "$total_score >= 7.0" | bc -l) )); then
        METRICS["overall_quality_rating"]="Good"
        log_success "Overall Quality Score: ${total_score}/10 (Good)"
    elif (( $(echo "$total_score >= 5.5" | bc -l) )); then
        METRICS["overall_quality_rating"]="Acceptable"
        log_warning "Overall Quality Score: ${total_score}/10 (Acceptable)"
    else
        METRICS["overall_quality_rating"]="Poor"
        log_error "Overall Quality Score: ${total_score}/10 (Poor)"
    fi
}

# ============================================================================
# REPORTING AND PERSISTENCE
# ============================================================================

save_metrics() {
    log_metric "Saving metrics to $METRICS_FILE..."
    
    # Add timestamp
    METRICS["timestamp"]=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    METRICS["commit_hash"]=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    METRICS["branch"]=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
    
    # Convert associative array to JSON
    local json_content="{"
    local first=true
    
    for key in "${!METRICS[@]}"; do
        if [[ "$first" == "true" ]]; then
            first=false
        else
            json_content+=","
        fi
        
        json_content+="\"$key\":\"${METRICS[$key]}\""
    done
    
    json_content+="}"
    
    echo "$json_content" | jq . > "$METRICS_FILE"
    log_success "Metrics saved to $METRICS_FILE"
}

generate_quality_report() {
    local report_file="quality-report.md"
    
    cat > "$report_file" << EOF
# Quality Metrics Report - Hot Reload TDD Cycle 2.3.1

Generated: $(date -u)
Commit: ${METRICS["commit_hash"]}
Branch: ${METRICS["branch"]}

## Overall Quality Assessment

**Overall Score: ${METRICS["overall_quality_score"]}/10 (${METRICS["overall_quality_rating"]})**

## Coverage Metrics

- **Overall Coverage**: ${METRICS["overall_coverage"]:-N/A}%
- **Hot Reload Coverage**: ${METRICS["hotreload_coverage"]:-N/A}%
- **Integration Coverage**: ${METRICS["integration_coverage"]:-N/A}%
- **Uncovered Functions**: ${METRICS["uncovered_functions"]:-N/A}
- **Coverage Quality**: ${METRICS["coverage_quality"]:-N/A}
- **Coverage Trend**: ${METRICS["coverage_trend"]:-N/A}

## Performance Metrics

- **Average ns/op**: ${METRICS["avg_ns_per_op"]:-N/A}
- **Average allocs/op**: ${METRICS["avg_allocs_per_op"]:-N/A}
- **Average B/op**: ${METRICS["avg_bytes_per_op"]:-N/A}
- **High Alloc Operations**: ${METRICS["high_alloc_operations"]:-N/A}
- **Performance Quality**: ${METRICS["performance_quality"]:-N/A}
- **Performance Trend**: ${METRICS["performance_trend"]:-N/A}
- **Memory Efficiency**: ${METRICS["memory_efficiency"]:-N/A}

## Code Quality Metrics

- **Total Lines of Code**: ${METRICS["total_lines_of_code"]:-N/A}
- **Total Test Lines**: ${METRICS["total_test_lines"]:-N/A}
- **Test to Code Ratio**: ${METRICS["test_to_code_ratio"]:-N/A}%
- **Comment Density**: ${METRICS["comment_density"]:-N/A}%
- **High Complexity Functions**: ${METRICS["high_complexity_functions"]:-N/A}
- **Average Complexity**: ${METRICS["average_complexity"]:-N/A}
- **Complexity Quality**: ${METRICS["complexity_quality"]:-N/A}

## Security Metrics

- **Total Security Issues**: ${METRICS["total_security_issues"]:-N/A}
- **High Severity Issues**: ${METRICS["high_severity_issues"]:-N/A}
- **Medium Severity Issues**: ${METRICS["medium_severity_issues"]:-N/A}
- **Path Cleaning Usage**: ${METRICS["path_cleaning_usage"]:-N/A}
- **Input Validation Patterns**: ${METRICS["input_validation_patterns"]:-N/A}
- **Security Quality**: ${METRICS["security_quality"]:-N/A}

## Maintainability Metrics

- **Total Functions**: ${METRICS["total_functions"]:-N/A}
- **Average Function Length**: ${METRICS["average_function_length"]:-N/A} lines
- **Interface Count**: ${METRICS["interface_count"]:-N/A}
- **Function Size Quality**: ${METRICS["function_size_quality"]:-N/A}
- **Interface Design Quality**: ${METRICS["interface_design_quality"]:-N/A}

## Recommendations

EOF
    
    # Add recommendations based on quality scores
    local score=${METRICS["overall_quality_score"]:-0}
    
    if (( $(echo "$score < 7.0" | bc -l) )); then
        cat >> "$report_file" << EOF
### Priority Actions Required

- **Coverage**: Increase hot reload test coverage to meet 95% target
- **Performance**: Optimize operations that exceed 1ms threshold
- **Security**: Address any high/medium severity security issues
- **Code Quality**: Refactor high complexity functions

EOF
    fi
    
    cat >> "$report_file" << EOF
### General Recommendations

- Monitor coverage trends and maintain upward trajectory
- Regular performance benchmarking to detect regressions
- Continuous security scanning integration
- Maintain good documentation and interface design practices
- Consider implementing additional integration tests for edge cases

## Historical Trends

EOF
    
    # Add trend information if available
    if [[ -f ".coverage-history" ]]; then
        echo "### Coverage Trend" >> "$report_file"
        echo '```' >> "$report_file"
        tail -5 .coverage-history >> "$report_file"
        echo '```' >> "$report_file"
    fi
    
    if [[ -f ".performance-history" ]]; then
        echo "### Performance Trend" >> "$report_file"
        echo '```' >> "$report_file"
        tail -5 .performance-history >> "$report_file"
        echo '```' >> "$report_file"
    fi
    
    log_success "Quality report generated: $report_file"
}

cleanup_temp_files() {
    local temp_files=(
        "coverage.out"
        "hotreload-coverage.out"
        "integration-coverage.out"
        "benchmark-results.txt"
        "security-scan.json"
    )
    
    for file in "${temp_files[@]}"; do
        [[ -f "$file" ]] && rm -f "$file"
    done
}

show_usage() {
    cat << EOF
Usage: $0 [options]

Quality Metrics Collection and Analysis for Hot Reload TDD Cycle 2.3.1

Options:
  --collect-only    - Collect metrics only (no analysis)
  --analyze-only    - Analyze existing metrics only
  --report-only     - Generate report from existing metrics
  --cleanup         - Clean up temporary files
  --help, -h        - Show this help message

Environment Variables:
  COVERAGE_TARGET           - Coverage target percentage (default: 95)
  PERFORMANCE_TARGET_NS     - Performance target in nanoseconds (default: 1000000)
  COMPLEXITY_TARGET         - Complexity target (default: 10)
  SECURITY_TARGET           - Maximum security issues (default: 0)
  MAINTAINABILITY_TARGET    - Maintainability target score (default: 8)

Examples:
  $0                        # Full metrics collection and analysis
  $0 --collect-only         # Collect metrics without analysis
  $0 --report-only          # Generate report from existing data

EOF
}

main() {
    cd "$PROJECT_ROOT"
    
    case "${1:-}" in
        "--help"|"-h")
            show_usage
            exit 0
            ;;
        "--collect-only")
            collect_coverage_metrics
            collect_performance_metrics
            collect_code_quality_metrics
            collect_security_metrics
            collect_maintainability_metrics
            save_metrics
            ;;
        "--analyze-only")
            analyze_coverage_quality
            analyze_performance_quality
            analyze_code_quality
            analyze_security_quality
            analyze_maintainability
            calculate_overall_quality_score
            save_metrics
            generate_quality_report
            ;;
        "--report-only")
            generate_quality_report
            ;;
        "--cleanup")
            cleanup_temp_files
            ;;
        *)
            log_metric "Starting comprehensive quality metrics collection..."
            
            # Collection phase
            collect_coverage_metrics
            collect_performance_metrics
            collect_code_quality_metrics
            collect_security_metrics
            collect_maintainability_metrics
            
            # Analysis phase
            analyze_coverage_quality
            analyze_performance_quality
            analyze_code_quality
            analyze_security_quality
            analyze_maintainability
            calculate_overall_quality_score
            
            # Reporting phase
            save_metrics
            generate_quality_report
            
            # Cleanup
            if [[ "${KEEP_ARTIFACTS:-}" != "true" ]]; then
                cleanup_temp_files
            fi
            
            log_success "Quality metrics analysis complete!"
            
            # Exit with appropriate code based on overall quality
            local score=${METRICS["overall_quality_score"]:-0}
            if (( $(echo "$score >= $MAINTAINABILITY_TARGET" | bc -l) )); then
                exit 0
            else
                exit 1
            fi
            ;;
    esac
}

# Execute main function
main "$@"