# TDD Implementation Workflow for Media-Sync

## Overview
This document defines a comprehensive Test-Driven Development workflow for the media-sync Go project, implementing a plugin-based media synchronization platform using GitHub Flow and progressive development principles.

## Core Principles

### TDD Cycle: Red-Green-Refactor
1. **Red**: Write a failing test that defines the desired behavior
2. **Green**: Write the minimal code to make the test pass
3. **Refactor**: Improve code quality while keeping tests green

### Minimal Working Units
Each development unit must be:
- Independently testable
- Deployable as a working component
- Verifiable through automated tests
- Small enough to complete in 1-2 days

## Project Architecture Layers

### Layer 1: Core Interfaces & Types
**Purpose**: Define contracts and data structures
**Tests**: Interface compliance, type validation
**Examples**: `InputService`, `OutputService`, `Plugin` interfaces

### Layer 2: Infrastructure
**Purpose**: Database, configuration, logging, security
**Tests**: Connection handling, config parsing, key management
**Examples**: SQLite wrapper, config loader, keyring integration

### Layer 3: Plugin System
**Purpose**: Plugin loading, lifecycle management
**Tests**: Plugin discovery, loading, unloading, hot reload
**Examples**: Plugin manager, YAML config parser

### Layer 4: Services & Business Logic
**Purpose**: Core synchronization logic
**Tests**: Data transformation, sync algorithms, error handling
**Examples**: Media processor, sync coordinator

### Layer 5: External Integrations
**Purpose**: API clients, authentication
**Tests**: Mock external services, auth flows
**Examples**: Tumblr client, OAuth2 handler

## TDD Development Process

### Phase 1: Foundation (Weeks 1-2)

#### 1.1 Core Interfaces & Types
```bash
# Branch: feature/core-interfaces
git checkout -b feature/core-interfaces
```

**Test-First Approach**:
```go
// internal/core/interfaces_test.go
func TestInputServiceContract(t *testing.T) {
    // Test that InputService interface can be implemented
    var _ InputService = (*mockInputService)(nil)
}

func TestPluginMetadata(t *testing.T) {
    // Test plugin metadata structure
    metadata := PluginMetadata{
        Name:    "test-plugin",
        Version: "1.0.0",
        Type:    "input",
    }
    assert.Equal(t, "test-plugin", metadata.Name)
}
```

**Minimal Implementation**:
```go
// internal/core/interfaces.go
type InputService interface {
    FetchMedia(ctx context.Context, config map[string]interface{}) ([]MediaItem, error)
    GetMetadata() PluginMetadata
}

type MediaItem struct {
    ID          string
    URL         string
    ContentType string
    Metadata    map[string]interface{}
}
```

#### 1.2 Configuration System
```bash
# Branch: feature/config-system
git checkout main && git pull
git checkout -b feature/config-system
```

**Test Structure**:
```go
// internal/config/loader_test.go
func TestConfigLoader_LoadFromFile(t *testing.T) {
    // Test configuration loading from YAML
    tempFile := createTempConfigFile(t, validYAML)
    defer os.Remove(tempFile)
    
    config, err := NewLoader().LoadFromFile(tempFile)
    require.NoError(t, err)
    assert.Equal(t, "test-service", config.Services[0].Name)
}

func TestConfigLoader_HotReload(t *testing.T) {
    // Test hot reload functionality
    loader := NewLoader()
    reloadChan := make(chan ConfigChange, 1)
    
    err := loader.WatchForChanges(tempFile, reloadChan)
    require.NoError(t, err)
    
    // Modify file and verify reload signal
}
```

#### 1.3 Database Layer
```bash
# Branch: feature/database-layer
```

**Test Approach**:
```go
// internal/storage/sqlite_test.go
func TestSQLiteStore_CreateMedia(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    media := MediaItem{
        ID:  "test-123",
        URL: "https://example.com/image.jpg",
    }
    
    err := db.CreateMedia(context.Background(), media)
    require.NoError(t, err)
    
    // Verify media was stored
    retrieved, err := db.GetMedia(context.Background(), "test-123")
    require.NoError(t, err)
    assert.Equal(t, media.URL, retrieved.URL)
}
```

### Phase 2: Plugin System (Weeks 3-4)

#### 2.1 Plugin Discovery & Loading
```bash
# Branch: feature/plugin-system
```

**Test Strategy**:
```go
// internal/plugins/manager_test.go
func TestPluginManager_LoadPlugin(t *testing.T) {
    manager := NewPluginManager()
    
    // Create test plugin directory structure
    pluginDir := setupTestPluginDir(t)
    defer cleanup(pluginDir)
    
    plugins, err := manager.LoadPlugins(pluginDir)
    require.NoError(t, err)
    assert.Len(t, plugins, 1)
    assert.Equal(t, "test-input-plugin", plugins[0].GetMetadata().Name)
}

func TestPluginManager_HotReload(t *testing.T) {
    // Test plugin hot reloading
    manager := NewPluginManager()
    reloadChan := make(chan PluginEvent, 1)
    
    err := manager.WatchPlugins(pluginDir, reloadChan)
    require.NoError(t, err)
    
    // Add new plugin and verify event
}
```

#### 2.2 Plugin Configuration
```go
// internal/plugins/config_test.go
func TestPluginConfig_Validation(t *testing.T) {
    config := PluginConfig{
        Name: "tumblr",
        Type: "input",
        Settings: map[string]interface{}{
            "api_key": "test-key",
        },
    }
    
    err := config.Validate()
    require.NoError(t, err)
}
```

### Phase 3: Core Services (Weeks 5-6)

#### 3.1 Synchronization Engine
```bash
# Branch: feature/sync-engine
```

**Test Approach**:
```go
// internal/sync/coordinator_test.go
func TestSyncCoordinator_SyncService(t *testing.T) {
    // Setup mocks
    mockInput := &mockInputService{}
    mockOutput := &mockOutputService{}
    mockStorage := &mockStorage{}
    
    coordinator := NewSyncCoordinator(mockStorage)
    coordinator.RegisterInput("test-input", mockInput)
    coordinator.RegisterOutput("test-output", mockOutput)
    
    // Configure mock responses
    mockInput.On("FetchMedia").Return([]MediaItem{testMedia}, nil)
    mockOutput.On("SendMedia").Return(nil)
    
    err := coordinator.SyncService(context.Background(), "test-service")
    require.NoError(t, err)
    
    // Verify all mocks were called correctly
    mockInput.AssertExpectations(t)
    mockOutput.AssertExpectations(t)
}
```

### Phase 4: Tumblr Plugin MVP (Weeks 7-8)

#### 4.1 Tumblr Input Plugin
```bash
# Branch: feature/tumblr-plugin
```

**Test Strategy with External Dependencies**:
```go
// plugins/tumblr/input_test.go
func TestTumblrInput_FetchMedia(t *testing.T) {
    // Use httptest for external API testing
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(mockTumblrResponse)
    }))
    defer server.Close()
    
    plugin := &TumblrInputPlugin{
        client: &http.Client{},
        apiURL: server.URL, // Override API URL for testing
    }
    
    config := map[string]interface{}{
        "username": "testuser",
        "api_key":  "test-key",
    }
    
    media, err := plugin.FetchMedia(context.Background(), config)
    require.NoError(t, err)
    assert.Len(t, media, 5)
}

func TestTumblrInput_Authentication(t *testing.T) {
    // Test OAuth2 flow with mock server
}
```

## GitHub Flow Integration

### Branch Naming Convention
```
feature/component-name     # New features
bugfix/issue-description   # Bug fixes
hotfix/critical-issue      # Production fixes
refactor/component-name    # Code improvements
```

### Pull Request Requirements

#### Pre-PR Checklist
```bash
# Run full test suite
go test ./...

# Check test coverage (minimum 80%)
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run linting
golangci-lint run

# Format code
go fmt ./...

# Check for race conditions
go test -race ./...

# Security scan
gosec ./...
```

#### PR Template
```markdown
## Summary
Brief description of changes

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing completed

## TDD Cycle Verification
- [ ] Red: Test written first and failed
- [ ] Green: Minimal code written to pass
- [ ] Refactor: Code improved without breaking tests

## Quality Gates
- [ ] Test coverage >= 80%
- [ ] All lints pass
- [ ] No race conditions detected
- [ ] Security scan clean
```

### CI/CD Pipeline (.github/workflows/ci.yml)
```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: '1.21'
    
    - name: Download dependencies
      run: go mod download
    
    - name: Run tests
      run: go test -v -race -coverprofile=coverage.out ./...
    
    - name: Check coverage
      run: |
        coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
        if (( $(echo "$coverage < 80" | bc -l) )); then
          echo "Coverage $coverage% is below 80%"
          exit 1
        fi
    
    - name: Run linter
      uses: golangci/golangci-lint-action@v3
    
    - name: Security scan
      run: |
        go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
        gosec ./...
```

## Testing Strategies by Component

### 1. Interface Testing
```go
// Test interface compliance
func TestInterfaceCompliance(t *testing.T) {
    var _ InputService = (*TumblrInputPlugin)(nil)
    var _ OutputService = (*LocalFileOutput)(nil)
}
```

### 2. Configuration Testing
```go
// Test configuration validation
func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name    string
        config  ServiceConfig
        wantErr bool
    }{
        {
            name: "valid config",
            config: ServiceConfig{
                Name: "test",
                Type: "input",
                Plugin: "tumblr",
            },
            wantErr: false,
        },
        {
            name: "missing name",
            config: ServiceConfig{
                Type: "input",
                Plugin: "tumblr",
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 3. Database Testing
```go
// Use in-memory SQLite for fast tests
func setupTestDB(t *testing.T) *SQLiteStore {
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    
    store := &SQLiteStore{db: db}
    err = store.Migrate()
    require.NoError(t, err)
    
    t.Cleanup(func() {
        db.Close()
    })
    
    return store
}
```

### 4. External API Testing
```go
// Mock external services
func TestExternalAPI(t *testing.T) {
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/api/posts":
            json.NewEncoder(w).Encode(mockPostsResponse)
        case "/oauth/token":
            json.NewEncoder(w).Encode(mockTokenResponse)
        default:
            w.WriteStatus(404)
        }
    }))
    defer mockServer.Close()
    
    // Test with mock server
}
```

### 5. Authentication Testing
```go
// Test OAuth2 flow
func TestOAuth2Flow(t *testing.T) {
    // Mock OAuth2 provider
    mockProvider := setupMockOAuth2Provider(t)
    defer mockProvider.Close()
    
    auth := NewOAuth2Authenticator(OAuth2Config{
        AuthURL:  mockProvider.URL + "/auth",
        TokenURL: mockProvider.URL + "/token",
        ClientID: "test-client",
    })
    
    // Test authorization URL generation
    authURL := auth.GetAuthorizationURL("state123")
    assert.Contains(t, authURL, "client_id=test-client")
    
    // Test token exchange
    token, err := auth.ExchangeCodeForToken(context.Background(), "test-code")
    require.NoError(t, err)
    assert.NotEmpty(t, token.AccessToken)
}
```

### 6. Plugin Hot Reload Testing
```go
func TestPluginHotReload(t *testing.T) {
    // Setup plugin directory
    pluginDir := t.TempDir()
    manager := NewPluginManager()
    
    eventChan := make(chan PluginEvent, 10)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    go manager.WatchPlugins(ctx, pluginDir, eventChan)
    
    // Create plugin file
    pluginPath := filepath.Join(pluginDir, "test-plugin.so")
    createTestPlugin(t, pluginPath)
    
    // Wait for load event
    select {
    case event := <-eventChan:
        assert.Equal(t, PluginLoaded, event.Type)
        assert.Equal(t, "test-plugin", event.Plugin.Name)
    case <-time.After(5 * time.Second):
        t.Fatal("Plugin load event not received")
    }
    
    // Modify plugin and test reload
    time.Sleep(100 * time.Millisecond) // Ensure different mtime
    updateTestPlugin(t, pluginPath)
    
    // Wait for reload event
    select {
    case event := <-eventChan:
        assert.Equal(t, PluginReloaded, event.Type)
    case <-time.After(5 * time.Second):
        t.Fatal("Plugin reload event not received")
    }
}
```

## Quality Gates & Metrics

### Test Coverage Requirements
- **Minimum**: 80% overall coverage
- **Critical paths**: 95% coverage (authentication, data sync)
- **New features**: 90% coverage required

### Performance Benchmarks
```go
func BenchmarkMediaSync(b *testing.B) {
    // Benchmark sync performance
    coordinator := setupBenchmarkCoordinator()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        err := coordinator.SyncService(context.Background(), "test-service")
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### Memory Leak Detection
```bash
# Run tests with memory profiling
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof
```

## Development Phases & Milestones

### Phase 1: Foundation (Weeks 1-2)
**Deliverables**:
- [ ] Core interfaces defined and tested
- [ ] Configuration system with hot reload
- [ ] SQLite storage layer
- [ ] Basic logging and error handling

**Quality Gates**:
- [ ] 85% test coverage
- [ ] All interfaces have at least one implementation
- [ ] Hot reload works for configuration changes

### Phase 2: Plugin System (Weeks 3-4)
**Deliverables**:
- [ ] Plugin discovery and loading
- [ ] Plugin lifecycle management
- [ ] Hot reload for plugins
- [ ] Plugin configuration validation

**Quality Gates**:
- [ ] Plugin hot reload works without service interruption
- [ ] Plugin isolation prevents crashes
- [ ] Configuration validation catches all invalid states

### Phase 3: Core Services (Weeks 5-6)
**Deliverables**:
- [ ] Synchronization coordinator
- [ ] Data transformation pipeline
- [ ] Error recovery mechanisms
- [ ] Rate limiting and backpressure

**Quality Gates**:
- [ ] Sync process handles failures gracefully
- [ ] Memory usage stays constant during long-running syncs
- [ ] Rate limiting prevents API throttling

### Phase 4: Tumblr Plugin MVP (Weeks 7-8)
**Deliverables**:
- [ ] Tumblr API client
- [ ] OAuth2 authentication
- [ ] Media fetching and processing
- [ ] End-to-end working example

**Quality Gates**:
- [ ] Complete OAuth2 flow works
- [ ] Can fetch and store Tumblr media
- [ ] Handles API rate limits correctly
- [ ] Secure credential storage

## Dependency Management for Testing

### Mock Generation
```bash
# Generate mocks using mockery
go install github.com/vektra/mockery/v2@latest

# Generate mocks for interfaces
mockery --name=InputService --dir=internal/core --output=internal/mocks
mockery --name=OutputService --dir=internal/core --output=internal/mocks
```

### Test Containers for Integration Tests
```go
// Use testcontainers for database integration tests
func TestWithRealDatabase(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    ctx := context.Background()
    container, err := sqlite.RunContainer(ctx)
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    db, err := sql.Open("sqlite3", container.ConnectionString())
    require.NoError(t, err)
    
    // Run integration tests with real database
}
```

### Environment-Based Test Configuration
```go
// Allow switching between mock and real services
func NewTestInputService() InputService {
    if os.Getenv("USE_REAL_TUMBLR_API") == "true" {
        return NewTumblrInputPlugin()
    }
    return &MockInputService{}
}
```

## Continuous Improvement

### Metrics Collection
- Test execution time trends
- Code coverage trends
- Bug discovery rate by phase
- Time from red to green in TDD cycles

### Regular Reviews
- Weekly TDD retrospectives
- Monthly architecture reviews
- Quality gate effectiveness analysis
- Development velocity tracking

This workflow ensures that each component is built with tests first, maintains high quality through continuous validation, and integrates smoothly with the overall system architecture.