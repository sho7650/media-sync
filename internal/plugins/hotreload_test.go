package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TDD Cycle 2.3.1: Hot Reload File System Watching Implementation Strategy
// This file implements comprehensive test scenarios for hot reload functionality
// following the project's TDD guidelines with RED-GREEN-REFACTOR cycles.

// RED Phase Test Scenarios - These tests will initially fail and drive implementation

// TestHotReloadWatcher_FileWatchingInterface tests interface compliance
func TestHotReloadWatcher_FileWatchingInterface(t *testing.T) {
	// Interface compliance verification
	var _ FileWatcher = (*HotReloadWatcher)(nil)
	var _ PluginReloaderService = (*PluginReloaderImpl)(nil)
}

// TestHotReloadWatcher_BasicFileWatching tests fundamental file watching capability
func TestHotReloadWatcher_BasicFileWatching(t *testing.T) {
	// Setup test directory using t.TempDir() as per project guidelines
	testDir := t.TempDir()
	pluginFile := filepath.Join(testDir, "test-plugin.json")

	// Create initial plugin configuration
	config := PluginConfig{
		Name:    "test-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	configData, err := json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pluginFile, configData, 0644))

	// Create watcher
	watcher := NewHotReloadWatcher()
	defer watcher.Stop()

	// Setup event channel
	eventChan := make(chan FileEvent, 10)
	
	// Start watching
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err = watcher.Watch(ctx, testDir, eventChan)
	require.NoError(t, err)

	// Modify the plugin file to trigger event
	modifiedConfig := config
	modifiedConfig.Version = "1.0.1"
	modifiedData, err := json.Marshal(modifiedConfig)
	require.NoError(t, err)
	
	require.NoError(t, os.WriteFile(pluginFile, modifiedData, 0644))

	// Verify file event is received
	select {
	case event := <-eventChan:
		assert.Equal(t, FileEventModified, event.Type)
		assert.Equal(t, pluginFile, event.Path)
		assert.True(t, event.IsPluginFile())
	case <-time.After(2 * time.Second):
		t.Fatal("Expected file modification event not received")
	}
}

// TestHotReloadWatcher_PluginReloadIntegration tests integration with plugin manager
func TestHotReloadWatcher_PluginReloadIntegration(t *testing.T) {
	testDir := t.TempDir()
	
	// Create mock plugin manager
	mockManager := &MockPluginManager{}
	
	// Setup expectations
	mockManager.On("GetPluginStatus", "test-plugin").Return(PluginStatus{
		State: PluginStateRunning,
	}, true)
	mockManager.On("StopPlugin", mock.Anything, "test-plugin").Return(nil)
	mockManager.On("UnloadPlugin", mock.Anything, "test-plugin").Return(nil)
	mockManager.On("LoadPlugin", mock.MatchedBy(func(config PluginConfig) bool {
		return config.Name == "test-plugin" && config.Version == "1.0.1"
	})).Return(nil)
	mockManager.On("StartPlugin", mock.Anything, "test-plugin").Return(nil)

	// Create hot reload system with mock manager  
	reloader := NewPluginReloaderImpl(mockManager)
	watcher := NewHotReloadWatcher()
	hotReload := NewHotReloadSystem(watcher, reloader)
	
	defer hotReload.Shutdown(context.Background())

	// Create and modify plugin file
	pluginFile := filepath.Join(testDir, "test-plugin.json")
	initialConfig := PluginConfig{
		Name:    "test-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	configData, err := json.Marshal(initialConfig)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pluginFile, configData, 0644))

	// Start hot reload system
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err = hotReload.StartWatching(ctx, testDir)
	require.NoError(t, err)

	// Simulate plugin file change
	modifiedConfig := initialConfig
	modifiedConfig.Version = "1.0.1"
	modifiedData, err := json.Marshal(modifiedConfig)
	require.NoError(t, err)
	
	require.NoError(t, os.WriteFile(pluginFile, modifiedData, 0644))

	// Allow time for processing
	time.Sleep(500 * time.Millisecond)

	// Verify plugin manager methods were called
	mockManager.AssertExpectations(t)
}

// TestHotReloadWatcher_EventDebouncing tests event debouncing to prevent rapid reloads
func TestHotReloadWatcher_EventDebouncing(t *testing.T) {
	testDir := t.TempDir()
	pluginFile := filepath.Join(testDir, "debounce-plugin.json")

	// Create initial plugin file
	config := PluginConfig{
		Name:    "debounce-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	configData, err := json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pluginFile, configData, 0644))

	// Create watcher with debouncing
	watcher := NewHotReloadWatcher()
	watcher.SetDebounceInterval(200 * time.Millisecond)
	defer watcher.Stop()

	eventChan := make(chan FileEvent, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err = watcher.Watch(ctx, testDir, eventChan)
	require.NoError(t, err)

	// Rapidly modify file multiple times
	for i := 0; i < 5; i++ {
		config.Version = fmt.Sprintf("1.0.%d", i+1)
		data, err := json.Marshal(config)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(pluginFile, data, 0644))
		time.Sleep(50 * time.Millisecond) // Rapid succession
	}

	// Should receive only one debounced event
	eventCount := 0
	timeout := time.After(1 * time.Second)
	
	for {
		select {
		case <-eventChan:
			eventCount++
		case <-timeout:
			goto done
		case <-time.After(100 * time.Millisecond):
			if eventCount > 0 {
				goto done
			}
		}
	}
	
done:
	// Should have received exactly one debounced event
	assert.Equal(t, 1, eventCount, "Expected exactly one debounced event")
}

// TestHotReloadWatcher_ErrorHandling tests error scenarios and recovery
func TestHotReloadWatcher_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		setupError    func(t *testing.T, dir string) string
		expectedError string
		shouldRecover bool
	}{
		{
			name: "invalid plugin file format",
			setupError: func(t *testing.T, dir string) string {
				pluginFile := filepath.Join(dir, "invalid.json")
				require.NoError(t, os.WriteFile(pluginFile, []byte("invalid json"), 0644))
				return pluginFile
			},
			expectedError: "invalid JSON",
			shouldRecover: true,
		},
		{
			name: "missing plugin metadata",
			setupError: func(t *testing.T, dir string) string {
				pluginFile := filepath.Join(dir, "incomplete.json")
				incompleteConfig := map[string]interface{}{
					"name": "test",
					// Missing required fields
				}
				data, _ := json.Marshal(incompleteConfig)
				require.NoError(t, os.WriteFile(pluginFile, data, 0644))
				return pluginFile
			},
			expectedError: "validation failed",
			shouldRecover: true,
		},
		{
			name: "permission denied on file",
			setupError: func(t *testing.T, dir string) string {
				pluginFile := filepath.Join(dir, "readonly.json")
				config := PluginConfig{
					Name:    "readonly-plugin",
					Type:    "input",
					Version: "1.0.0",
					Enabled: true,
				}
				data, _ := json.Marshal(config)
				require.NoError(t, os.WriteFile(pluginFile, data, 0000)) // No permissions
				return pluginFile
			},
			expectedError: "permission denied",
			shouldRecover: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := t.TempDir()
			errorFile := tt.setupError(t, testDir)

			mockManager := &MockPluginManager{}
			if tt.shouldRecover {
				// Should attempt reload despite error
				mockManager.On("UnloadPlugin", mock.Anything, mock.Anything).Return(nil).Maybe()
				mockManager.On("LoadPlugin", mock.Anything).Return(errors.New(tt.expectedError)).Maybe()
			}

			reloader := NewPluginReloaderImpl(mockManager)
			watcher := NewHotReloadWatcher()
			hotReload := NewHotReloadSystem(watcher, reloader)
			
			defer hotReload.Shutdown(context.Background())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			
			err := hotReload.StartWatching(ctx, testDir)
			require.NoError(t, err)

			// Trigger reload by modifying the error-prone file
			time.Sleep(100 * time.Millisecond)
			require.NoError(t, os.Chtimes(errorFile, time.Now(), time.Now()))

			// Allow time for error handling
			time.Sleep(300 * time.Millisecond)

			// Get error metrics
			metrics := hotReload.GetMetrics()
			if tt.shouldRecover {
				assert.Greater(t, metrics.ErrorCount, uint64(0))
			}
		})
	}
}

// TestHotReloadWatcher_ConcurrentFileOperations tests concurrent file operations
func TestHotReloadWatcher_ConcurrentFileOperations(t *testing.T) {
	testDir := t.TempDir()
	
	mockManager := &MockPluginManager{}
	mockManager.On("UnloadPlugin", mock.Anything, mock.Anything).Return(nil)
	mockManager.On("LoadPlugin", mock.Anything).Return(nil)
	mockManager.On("StartPlugin", mock.Anything, mock.Anything).Return(nil)

	reloader := NewPluginReloaderImpl(mockManager)
	watcher := NewHotReloadWatcher()
	hotReload := NewHotReloadSystem(watcher, reloader)
	
	defer hotReload.Shutdown(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := hotReload.StartWatching(ctx, testDir)
	require.NoError(t, err)

	// Concurrently modify multiple plugin files
	var wg sync.WaitGroup
	numFiles := 5
	
	for i := 0; i < numFiles; i++ {
		wg.Add(1)
		go func(fileIndex int) {
			defer wg.Done()
			
			pluginFile := filepath.Join(testDir, fmt.Sprintf("plugin-%d.json", fileIndex))
			config := PluginConfig{
				Name:    fmt.Sprintf("plugin-%d", fileIndex),
				Type:    "input",
				Version: "1.0.0",
				Enabled: true,
			}
			
			// Create file
			data, err := json.Marshal(config)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(pluginFile, data, 0644))
			
			// Modify file multiple times
			for j := 1; j <= 3; j++ {
				config.Version = fmt.Sprintf("1.0.%d", j)
				data, err := json.Marshal(config)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(pluginFile, data, 0644))
				time.Sleep(50 * time.Millisecond)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Allow time for all events to be processed
	time.Sleep(1 * time.Second)

	metrics := hotReload.GetMetrics()
	assert.Greater(t, metrics.ReloadCount, uint64(0))
}

// TestHotReloadWatcher_ResourceCleanup tests proper resource cleanup
func TestHotReloadWatcher_ResourceCleanup(t *testing.T) {
	testDir := t.TempDir()
	
	watcher := NewHotReloadWatcher()
	eventChan := make(chan FileEvent, 10)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := watcher.Watch(ctx, testDir, eventChan)
	require.NoError(t, err)

	// Verify watcher is active
	assert.True(t, watcher.IsWatching())
	
	// Stop watcher
	err = watcher.Stop()
	require.NoError(t, err)
	
	// Verify cleanup
	assert.False(t, watcher.IsWatching())
	
	// Verify no goroutine leaks by ensuring channel is closed
	select {
	case _, open := <-eventChan:
		if open {
			t.Error("Event channel should be closed after stopping watcher")
		}
	case <-time.After(100 * time.Millisecond):
		// Expected - channel should be closed
	}
}

// TestHotReloadWatcher_PerformanceMetrics tests performance monitoring
func TestHotReloadWatcher_PerformanceMetrics(t *testing.T) {
	testDir := t.TempDir()
	
	mockManager := &MockPluginManager{}
	mockManager.On("UnloadPlugin", mock.Anything, mock.Anything).Return(nil)
	mockManager.On("LoadPlugin", mock.Anything).Return(nil)
	mockManager.On("StartPlugin", mock.Anything, mock.Anything).Return(nil)

	reloader := NewPluginReloaderImpl(mockManager)
	watcher := NewHotReloadWatcher()
	hotReload := NewHotReloadSystem(watcher, reloader)
	
	defer hotReload.Shutdown(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := hotReload.StartWatching(ctx, testDir)
	require.NoError(t, err)

	// Create and modify plugin file
	pluginFile := filepath.Join(testDir, "perf-plugin.json")
	config := PluginConfig{
		Name:    "perf-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	data, err := json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pluginFile, data, 0644))

	// Modify multiple times to generate metrics
	for i := 1; i <= 3; i++ {
		config.Version = fmt.Sprintf("1.0.%d", i)
		data, err := json.Marshal(config)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(pluginFile, data, 0644))
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(500 * time.Millisecond)

	// Verify metrics collection
	metrics := hotReload.GetMetrics()
	assert.Greater(t, metrics.ReloadCount, uint64(0))
	assert.Greater(t, metrics.EventsProcessed, uint64(0))
	assert.NotZero(t, metrics.AverageReloadTime)
	assert.NotZero(t, metrics.LastReloadTime)
}

// MOCK IMPLEMENTATIONS for testing

type MockPluginManager struct {
	mock.Mock
}

func (m *MockPluginManager) LoadPlugin(config PluginConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockPluginManager) UnloadPlugin(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockPluginManager) StartPlugin(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockPluginManager) StopPlugin(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockPluginManager) GetPluginStatus(name string) (PluginStatus, bool) {
	args := m.Called(name)
	return args.Get(0).(PluginStatus), args.Bool(1)
}

// INTERFACES and TYPES are now defined in hotreload.go

// HotReloadMetrics is now defined in hotreload.go

// IMPLEMENTATION moved to hotreload.go - tests can now use actual implementation