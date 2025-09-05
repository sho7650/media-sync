# TDD Hot Reload Implementation Strategy (Cycle 2.3.1)

## Overview

This document outlines the comprehensive Test-Driven Development strategy for implementing hot reload file system watching functionality. The implementation follows the project's TDD guidelines with Red-Green-Refactor cycles, using testify framework, interface compliance verification, and proper mocking patterns.

## Implementation Strategy

### Phase 1: RED Phase - Failing Tests Drive Design

The test file `/Users/sho/working/golang/work/media-sync/internal/plugins/hotreload_test.go` contains comprehensive failing test scenarios that define the expected behavior:

#### Core Test Scenarios

1. **Interface Compliance Tests**
   - Verifies `FileWatcher` and `PluginReloader` interface implementation
   - Ensures proper abstraction and contract definition

2. **Basic File Watching Tests**
   - Tests fundamental file system monitoring using fsnotify
   - Verifies event detection and propagation
   - Uses `t.TempDir()` for isolated test environments

3. **Plugin Integration Tests**
   - Tests integration with existing `PluginManager`
   - Verifies complete reload workflow: unload → load → start
   - Uses testify/mock for external dependency mocking

4. **Event Debouncing Tests**
   - Prevents rapid successive reloads from file system spam
   - Configurable debounce intervals for different scenarios
   - Table-driven test validation for timing behaviors

5. **Error Handling Tests**
   - Invalid JSON configurations
   - Missing plugin metadata
   - Permission denied scenarios
   - Recovery mechanisms and graceful degradation

6. **Concurrency Tests**
   - Multiple simultaneous file operations
   - Thread-safe event processing
   - Resource contention handling

7. **Performance & Metrics Tests**
   - Operational metrics collection
   - Resource cleanup verification
   - Memory leak detection

#### Running RED Phase Tests

```bash
# Verify tests fail initially (RED phase)
make test-unit
# Expected: All hotreload tests fail - interfaces and types don't exist
```

### Phase 2: GREEN Phase - Minimal Implementation

#### Step 1: Create Core Interfaces and Types

Create `/Users/sho/working/golang/work/media-sync/internal/plugins/hotreload.go`:

```go
package plugins

import (
    "context"
    "time"
    
    "github.com/fsnotify/fsnotify"
)

// FileWatcher interface - driven by tests
type FileWatcher interface {
    Watch(ctx context.Context, path string, eventChan chan<- FileEvent) error
    Stop() error
    IsWatching() bool
    SetDebounceInterval(duration time.Duration)
}

// PluginReloader interface - driven by tests
type PluginReloader interface {
    ReloadPlugin(ctx context.Context, config PluginConfig) error
    CanReload(pluginName string) bool
}

// HotReloadSystem interface - driven by tests
type HotReloadSystem interface {
    StartWatching(ctx context.Context, pluginDir string) error
    Shutdown(ctx context.Context) error
    GetMetrics() HotReloadMetrics
}

// FileEventType and FileEvent - driven by tests
type FileEventType int

const (
    FileEventCreated FileEventType = iota
    FileEventModified
    FileEventDeleted
    FileEventRenamed
)

type FileEvent struct {
    Type      FileEventType
    Path      string
    Timestamp time.Time
}

func (e FileEvent) IsPluginFile() bool {
    return filepath.Ext(e.Path) == ".json"
}

// HotReloadMetrics - driven by tests
type HotReloadMetrics struct {
    ReloadCount        uint64
    ErrorCount         uint64
    EventsProcessed    uint64
    AverageReloadTime  time.Duration
    LastReloadTime     time.Time
    DebounceHitCount   uint64
}
```

#### Step 2: Minimal HotReloadWatcher Implementation

```go
// HotReloadWatcher - minimal implementation to pass tests
type HotReloadWatcher struct {
    watcher     *fsnotify.Watcher
    watching    bool
    debounce    time.Duration
    ctx         context.Context
    cancel      context.CancelFunc
    mu          sync.RWMutex
}

func NewHotReloadWatcher() *HotReloadWatcher {
    return &HotReloadWatcher{
        debounce: 500 * time.Millisecond, // Default debounce
    }
}

func (w *HotReloadWatcher) Watch(ctx context.Context, path string, eventChan chan<- FileEvent) error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    
    w.mu.Lock()
    w.watcher = watcher
    w.watching = true
    w.ctx, w.cancel = context.WithCancel(ctx)
    w.mu.Unlock()
    
    if err := watcher.Add(path); err != nil {
        return err
    }
    
    go w.processEvents(eventChan)
    return nil
}

func (w *HotReloadWatcher) processEvents(eventChan chan<- FileEvent) {
    debounceTimer := make(map[string]*time.Timer)
    
    for {
        select {
        case event, ok := <-w.watcher.Events:
            if !ok {
                return
            }
            
            // Debouncing logic - minimal implementation
            if timer, exists := debounceTimer[event.Name]; exists {
                timer.Stop()
            }
            
            debounceTimer[event.Name] = time.AfterFunc(w.debounce, func() {
                fileEvent := FileEvent{
                    Path:      event.Name,
                    Timestamp: time.Now(),
                }
                
                if event.Op&fsnotify.Write == fsnotify.Write {
                    fileEvent.Type = FileEventModified
                } else if event.Op&fsnotify.Create == fsnotify.Create {
                    fileEvent.Type = FileEventCreated
                } else if event.Op&fsnotify.Remove == fsnotify.Remove {
                    fileEvent.Type = FileEventDeleted
                }
                
                select {
                case eventChan <- fileEvent:
                case <-w.ctx.Done():
                    return
                }
            })
            
        case <-w.ctx.Done():
            return
        }
    }
}

func (w *HotReloadWatcher) Stop() error {
    w.mu.Lock()
    defer w.mu.Unlock()
    
    if w.cancel != nil {
        w.cancel()
    }
    if w.watcher != nil {
        w.watcher.Close()
    }
    w.watching = false
    return nil
}

func (w *HotReloadWatcher) IsWatching() bool {
    w.mu.RLock()
    defer w.mu.RUnlock()
    return w.watching
}

func (w *HotReloadWatcher) SetDebounceInterval(duration time.Duration) {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.debounce = duration
}
```

#### Step 3: Add fsnotify Dependency

```bash
# Add fsnotify to go.mod
go get github.com/fsnotify/fsnotify@v1.7.0
go mod tidy
```

#### Step 4: Run GREEN Phase Tests

```bash
# Verify basic tests pass (GREEN phase)
make test-unit
# Expected: Basic interface and file watching tests pass
```

### Phase 3: REFACTOR Phase - Quality Improvements

#### Performance Optimizations

1. **Efficient Debouncing**
   ```go
   // Replace map-based debouncing with more efficient approach
   type debouncedEvent struct {
       event    FileEvent
       deadline time.Time
   }
   
   // Use priority queue for efficient timer management
   ```

2. **Memory Management**
   ```go
   // Add resource tracking and cleanup
   func (w *HotReloadWatcher) processEvents(eventChan chan<- FileEvent) {
       defer func() {
           // Cleanup resources
           close(eventChan)
       }()
       // ... processing logic
   }
   ```

#### Error Handling Improvements

1. **Graceful Degradation**
   ```go
   func (r *PluginReloader) ReloadPlugin(ctx context.Context, config PluginConfig) error {
       // Validate before attempting reload
       if err := config.Validate(); err != nil {
           return &ReloadError{
               PluginName: config.Name,
               Cause:      err,
               Recoverable: true,
           }
       }
       
       // Atomic reload operation
       return r.performAtomicReload(ctx, config)
   }
   ```

2. **Enhanced Logging and Observability**
   ```go
   // Add structured logging for debugging
   func (w *HotReloadWatcher) processEvents(eventChan chan<- FileEvent) {
       logger := log.WithField("component", "hotreload-watcher")
       
       for {
           select {
           case event := <-w.watcher.Events:
               logger.WithFields(log.Fields{
                   "file":      event.Name,
                   "operation": event.Op.String(),
               }).Debug("File system event received")
               // ... processing
           }
       }
   }
   ```

#### Concurrency Enhancements

1. **Worker Pool Pattern**
   ```go
   type HotReloadSystem struct {
       workerPool chan struct{} // Limit concurrent reloads
       // ... other fields
   }
   
   func (s *HotReloadSystem) processFileEvent(event FileEvent) {
       select {
       case s.workerPool <- struct{}{}:
           go func() {
               defer func() { <-s.workerPool }()
               s.handleReload(event)
           }()
       default:
           // Queue full, skip or queue for later
           s.metrics.SkippedReloads++
       }
   }
   ```

2. **Context-Aware Operations**
   ```go
   func (r *PluginReloader) ReloadPlugin(ctx context.Context, config PluginConfig) error {
       // Create timeout context for reload operations
       reloadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
       defer cancel()
       
       return r.performReloadWithContext(reloadCtx, config)
   }
   ```

#### Configuration and Flexibility

1. **Configurable Watch Options**
   ```go
   type WatchOptions struct {
       DebounceInterval    time.Duration
       MaxConcurrentReloads int
       FilePatterns        []string
       ExcludePatterns     []string
   }
   
   func (w *HotReloadWatcher) WithOptions(opts WatchOptions) *HotReloadWatcher {
       w.debounce = opts.DebounceInterval
       w.maxWorkers = opts.MaxConcurrentReloads
       // ... configure other options
       return w
   }
   ```

#### Run REFACTOR Phase Tests

```bash
# Verify all tests pass with improved implementation
make test-unit
make lint
make coverage
# Expected: All tests pass, coverage > 80%, no linting issues
```

## Integration Points

### Existing Plugin System Integration

The hot reload system integrates with existing components:

1. **PluginManager Integration**
   - Uses existing `LoadPlugin`, `UnloadPlugin`, `StartPlugin` methods
   - Respects plugin lifecycle states
   - Leverages health monitoring system

2. **Configuration System Integration**
   - Watches plugin configuration files
   - Validates configuration changes
   - Supports hot reload of plugin settings

3. **Error Handling Integration**
   - Uses existing `PluginError` types
   - Integrates with plugin status tracking
   - Supports recovery mechanisms

### File System Integration Patterns

```go
// Integration with existing config manager
func (h *HotReloadSystem) integrateWithConfigManager(configManager *config.Manager) {
    configManager.OnConfigChange(func(pluginName string, newConfig map[string]interface{}) {
        // Trigger plugin reload when configuration changes
        h.scheduleReload(pluginName, newConfig)
    })
}
```

## Testing Strategy Summary

### Test Coverage Requirements

- **Unit Tests**: 95% coverage for core functionality
- **Integration Tests**: End-to-end workflow testing
- **Performance Tests**: Verify debouncing and resource usage
- **Error Scenario Tests**: All error paths tested

### Test Data Management

```go
// Use table-driven tests for validation scenarios
func TestHotReloadWatcher_ValidationScenarios(t *testing.T) {
    tests := []struct {
        name        string
        setupFunc   func(t *testing.T, dir string) string
        expectError bool
        errorType   string
    }{
        // Test cases defined by requirements
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Mock Strategy

- Use testify/mock for external dependencies
- Mock file system operations for deterministic testing
- Mock plugin manager for isolation testing
- Real integration tests with temporary directories

## Quality Gates

Before considering this TDD cycle complete:

1. **✅ All Tests Pass**: Unit, integration, and performance tests
2. **✅ Code Coverage**: Minimum 80% overall, 95% for critical paths
3. **✅ Linting Clean**: No golangci-lint issues
4. **✅ Performance**: Debouncing works under load
5. **✅ Resource Cleanup**: No goroutine or file descriptor leaks
6. **✅ Error Handling**: All error scenarios have recovery paths
7. **✅ Documentation**: Interfaces and critical functions documented

## Implementation Commands

```bash
# Complete TDD cycle execution
git checkout main
git pull origin main
git checkout -b feature/hot-reload-filesystem-watching

# RED Phase - Verify tests fail
make tdd-red
make test-unit  # Should show failing tests

# GREEN Phase - Minimal implementation
# Implement interfaces and basic functionality
make tdd-green
make test-unit  # Should pass basic tests

# REFACTOR Phase - Quality improvements
make tdd-refactor
make test-unit lint coverage  # All quality checks pass

# Commit and create PR
git add .
git commit -m "feat: Implement hot reload file system watching (TDD Cycle 2.3.1)

- File system monitoring with fsnotify
- Event debouncing to prevent rapid reloads
- Plugin manager integration for hot reload
- Comprehensive error handling and recovery
- Performance monitoring and metrics collection
- Thread-safe concurrent operations
- Resource cleanup and leak prevention

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>"

git push -u origin feature/hot-reload-filesystem-watching
gh pr create --title "feat: Hot Reload File System Watching (TDD Cycle 2.3.1)" --body "$(cat <<'EOF'
## Summary
- Implements comprehensive hot reload file system watching using fsnotify
- Provides event debouncing to prevent rapid successive reloads
- Integrates with existing plugin manager for seamless hot reload operations
- Includes comprehensive error handling and graceful recovery mechanisms
- Performance monitoring with operational metrics collection

## Test Plan
- [ ] File system monitoring detects plugin configuration changes
- [ ] Event debouncing prevents reload spam during rapid file modifications  
- [ ] Plugin manager integration performs complete reload workflow
- [ ] Error scenarios handled gracefully with appropriate recovery
- [ ] Concurrent file operations processed safely
- [ ] Resource cleanup prevents memory and file descriptor leaks
- [ ] Performance metrics collected for operational monitoring

🤖 Generated with [Claude Code](https://claude.ai/code)
EOF
)"
```

This comprehensive TDD strategy provides a complete roadmap for implementing robust hot reload file system watching functionality while maintaining high code quality and thorough test coverage.