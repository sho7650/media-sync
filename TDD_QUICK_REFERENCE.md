# TDD Quick Reference Guide

## Essential Commands

```bash
# Setup (run once)
./scripts/init-tdd-project.sh
make dev-setup

# Daily TDD Cycle
make tdd-red        # Write failing test
make test-unit      # Confirm failure (RED)
make tdd-green      # Write minimal code
make test-unit      # Confirm success (GREEN)
make tdd-refactor   # Improve code quality
make test-unit lint # Verify refactor (GREEN)

# Quality Gates
make test           # All tests
make coverage       # Coverage report  
make lint           # Code quality
make security       # Security scan
make ci-test        # Full CI pipeline
```

## TDD Patterns by Component

### 1. Interface Testing
```go
func TestInterfaceCompliance(t *testing.T) {
    // Verify interface implementation
    var _ InputService = (*TumblrInputPlugin)(nil)
    var _ OutputService = (*LocalFileOutput)(nil)
}

func TestInterfaceContract(t *testing.T) {
    mock := &MockInputService{}
    mock.On("FetchMedia", mock.Anything, mock.Anything).Return([]MediaItem{}, nil)
    
    media, err := mock.FetchMedia(context.Background(), map[string]interface{}{})
    require.NoError(t, err)
    mock.AssertExpectations(t)
}
```

### 2. Configuration Testing
```go
func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  Config
        wantErr bool
    }{
        {"valid", validConfig, false},
        {"invalid", invalidConfig, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            assert.Equal(t, tt.wantErr, err != nil)
        })
    }
}
```

### 3. Database Testing
```go
func TestDatabaseOperations(t *testing.T) {
    db := setupTestDB(t)  // Use :memory: SQLite
    defer cleanupTestDB(t, db)
    
    // Test CRUD operations
    err := db.Create(testItem)
    require.NoError(t, err)
    
    item, err := db.Get(testItem.ID)
    require.NoError(t, err)
    assert.Equal(t, testItem.Name, item.Name)
}
```

### 4. External API Testing
```go
func TestExternalAPI(t *testing.T) {
    // Mock server for external API
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(mockResponse)
    }))
    defer server.Close()
    
    client := NewClient(server.URL)
    result, err := client.FetchData()
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### 5. Plugin Testing
```go
func TestPluginLoading(t *testing.T) {
    pluginDir := t.TempDir()
    createTestPlugin(t, pluginDir, "test-plugin")
    
    manager := NewPluginManager()
    plugins, err := manager.LoadPlugins(pluginDir)
    
    require.NoError(t, err)
    assert.Len(t, plugins, 1)
    assert.Equal(t, "test-plugin", plugins[0].Name)
}
```

### 6. Hot Reload Testing
```go
func TestHotReload(t *testing.T) {
    configFile := createTempConfig(t, initialConfig)
    
    loader := NewConfigLoader()
    changes := make(chan ConfigChange, 1)
    
    go loader.Watch(configFile, changes)
    
    // Modify file
    updateConfig(t, configFile, modifiedConfig)
    
    // Wait for change notification
    select {
    case change := <-changes:
        assert.Equal(t, ConfigModified, change.Type)
    case <-time.After(1 * time.Second):
        t.Fatal("Config change not detected")
    }
}
```

## Test Organization Patterns

### 1. Test File Structure
```
internal/
├── core/
│   ├── interfaces.go
│   ├── interfaces_test.go      # Interface tests
│   └── testdata/              # Test fixtures
├── config/
│   ├── loader.go
│   ├── loader_test.go         # Unit tests
│   ├── integration_test.go    # Integration tests
│   └── testdata/
│       ├── valid.yaml
│       └── invalid.yaml
```

### 2. Test Categories
```go
// Unit tests (fast, isolated)
func TestFunction_ValidInput_ReturnsExpected(t *testing.T) { }

// Integration tests (slower, external dependencies)
//go:build integration
func TestDatabase_Integration(t *testing.T) { }

// End-to-end tests (slowest, full system)
//go:build e2e
func TestFullWorkflow_E2E(t *testing.T) { }
```

### 3. Mock Setup Patterns
```go
// Test helper functions
func setupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    
    // Setup schema
    setupSchema(t, db)
    
    t.Cleanup(func() { db.Close() })
    return db
}

func setupMockServer(t *testing.T, responses map[string]interface{}) *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if response, ok := responses[r.URL.Path]; ok {
            json.NewEncoder(w).Encode(response)
        } else {
            http.NotFound(w, r)
        }
    }))
}
```

## Git Workflow

### Branch Naming
```
feature/component-name     # New features
bugfix/issue-description   # Bug fixes  
refactor/component-name    # Refactoring
test/test-improvement      # Test improvements
```

### Commit Messages
```
Add user authentication with JWT tokens

- Implement JWT token generation and validation
- Add login/logout endpoints
- Integrate with user service
- Add comprehensive test coverage (95%)

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

### Pull Request Workflow
```bash
# Before creating PR
make ci-test                # All quality checks
git push origin feature/my-feature
gh pr create --title "Add authentication" --body "$(cat PR_TEMPLATE.md)"

# PR must pass:
# - All tests (unit + integration)
# - Coverage >= 80%
# - All linters
# - Security scan
# - Code review approval
```

## Development Phases

### Phase 1: Foundation (Weeks 1-2)
- [ ] Core interfaces (`InputService`, `OutputService`, `MediaItem`)
- [ ] Configuration system with hot reload
- [ ] SQLite storage layer with transactions
- [ ] Basic application structure

### Phase 2: Plugin System (Weeks 3-4)  
- [ ] Plugin discovery and loading
- [ ] Plugin lifecycle management
- [ ] Hot reload for plugins
- [ ] Plugin configuration validation

### Phase 3: Core Services (Weeks 5-6)
- [ ] Synchronization coordinator
- [ ] Data transformation pipeline
- [ ] Concurrency and worker pools
- [ ] Error recovery mechanisms

### Phase 4: Tumblr Plugin MVP (Weeks 7-8)
- [ ] Tumblr API client
- [ ] OAuth2 authentication flow
- [ ] Media fetching and processing
- [ ] End-to-end integration

## Quality Gates

### Daily Quality Check
```bash
# Coverage check
make coverage
# Should show >= 80% overall, >= 95% for critical paths

# Complexity check  
gocyclo -over 10 .
# Should show no functions over complexity 10

# Security check
make security
# Should show no high/medium security issues

# Performance check
make benchmark
# Should show no significant regressions
```

### Pre-PR Quality Gates
```bash
make ci-test    # Must pass completely
make build      # Must build successfully
make docs       # Documentation up to date
```

### Phase Completion Gates
```bash
# All planned features implemented
make test           # All tests pass
make coverage       # Coverage targets met
make lint          # No linting issues
make security      # No security issues
make benchmark     # Performance acceptable
```

## Troubleshooting

### Test Failures
```bash
# Run specific test
go test -v -run TestSpecificFunction ./internal/core

# Run with race detection
go test -race ./...

# Run with verbose output
go test -v ./...

# Debug failing test
go test -v -run TestFailing ./... > debug.log 2>&1
```

### Coverage Issues
```bash
# Generate detailed coverage report
make coverage
open coverage.html  # View in browser

# Find uncovered code
go tool cover -func=coverage.out | grep -v "100.0%"
```

### Linting Issues
```bash
# Run specific linter
golangci-lint run --disable-all --enable=errcheck

# Fix formatting
make fmt

# Show all issues
golangci-lint run --issues-exit-code=0
```

### Plugin Issues
```bash
# Check plugin compilation
go build -buildmode=plugin -o test.so ./plugins/test

# Debug plugin loading
go test -v -run TestPluginLoading ./internal/plugins
```

## Key Files Reference

- `TDD_WORKFLOW.md` - Comprehensive workflow guide
- `examples/tdd_examples.go` - Concrete TDD examples
- `IMPLEMENTATION_ROADMAP.md` - Phase-by-phase implementation plan
- `Makefile` - All development commands
- `.golangci.yml` - Linting configuration
- `.github/workflows/ci.yml` - CI/CD pipeline

## Best Practices Summary

1. **Always Red-Green-Refactor**: Never skip the failing test
2. **Test-First**: Write tests before implementation
3. **Minimal Implementation**: Write just enough code to pass
4. **Refactor Fearlessly**: Improve code while tests are green
5. **Quality Gates**: Never compromise on coverage or linting
6. **Fast Feedback**: Keep test execution time under 10 seconds
7. **Clear Tests**: Test names should describe expected behavior
8. **Mock External Dependencies**: Don't test external services
9. **Test Edge Cases**: Happy path + error cases + boundary conditions
10. **Continuous Integration**: Every commit must pass CI

This quick reference provides everything needed for effective TDD implementation of the media-sync project.