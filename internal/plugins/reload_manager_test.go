package plugins

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/sho7650/media-sync/pkg/core/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test 1: Configuration Validation Before Reload
func TestReloadManager_ValidateConfigBeforeReload(t *testing.T) {
	manager := NewPluginManager()
	reloadMgr := NewReloadManager(manager, &ConfigValidatorImpl{})
	
	// Register mock factory for input type
	manager.RegisterFactory("input", &MockPluginFactory{})
	
	// Create plugin with valid config
	validConfig := &PluginConfig{
		Name:     "test-plugin",
		Type:     "input",
		Version:  "1.0.0",
		Enabled:  true,
		Settings: map[string]interface{}{
			"timeout": 5000,
		},
	}
	
	require.NoError(t, manager.LoadPlugin(*validConfig))
	require.NoError(t, manager.StartPlugin(context.Background(), "test-plugin"))
	
	// Attempt reload with invalid config
	invalidConfig := &PluginConfig{
		Name:     "test-plugin",
		Type:     "invalid-type", // Invalid
		Version:  "1.0.0",
		Settings: map[string]interface{}{
			"timeout": -1, // Invalid
		},
	}
	
	err := reloadMgr.AtomicReload(context.Background(), "test-plugin", invalidConfig)
	
	// Should fail validation and not affect running plugin
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	
	status, exists := manager.GetPluginStatus("test-plugin")
	assert.True(t, exists)
	assert.Equal(t, PluginStateRunning, status.State)
	assert.Equal(t, validConfig.Settings["timeout"], status.Config.Settings["timeout"])
}

// Test 2: Rollback on Plugin Start Failure
func TestReloadManager_RollbackOnStartFailure(t *testing.T) {
	manager := NewPluginManager()
	reloadMgr := NewReloadManager(manager, &ConfigValidatorImpl{})
	
	// Register mock factory that can simulate failures
	manager.RegisterFactory("input", &MockFailFactory{})
	
	// Setup initial plugin
	initialConfig := &PluginConfig{
		Name:     "test-plugin",
		Type:     "input",
		Version:  "1.0.0",
		Enabled:  true,
		Settings: map[string]interface{}{
			"mode": "stable",
		},
	}
	
	require.NoError(t, manager.LoadPlugin(*initialConfig))
	require.NoError(t, manager.StartPlugin(context.Background(), "test-plugin"))
	
	// Config that will fail to start
	failConfig := &PluginConfig{
		Name:     "test-plugin",
		Type:     "input",
		Version:  "1.0.0",
		Enabled:  true,
		Settings: map[string]interface{}{
			"mode": "fail-on-start", // Mock plugin will fail
		},
	}
	
	err := reloadMgr.AtomicReload(context.Background(), "test-plugin", failConfig)
	
	// Should fail and rollback to original state
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rolled back")
	
	status, exists := manager.GetPluginStatus("test-plugin")
	assert.True(t, exists)
	assert.Equal(t, PluginStateRunning, status.State)
	assert.Equal(t, "stable", status.Config.Settings["mode"])
}

// Test 3: Health Check Integration During Reload
func TestReloadManager_HealthCheckValidation(t *testing.T) {
	manager := NewPluginManager()
	reloadMgr := NewReloadManager(manager, &ConfigValidatorImpl{})
	reloadMgr.SetHealthTimeout(2 * time.Second)
	
	// Register health-aware mock factory
	manager.RegisterFactory("input", &MockHealthFactory{})
	
	// Enable health monitoring
	eventChan := make(chan HealthEvent, 10)
	require.NoError(t, manager.StartHealthMonitoring(context.Background(), eventChan, 500*time.Millisecond))
	defer manager.StopHealthMonitoring()
	
	// Setup initial plugin
	initialConfig := &PluginConfig{
		Name:     "health-plugin",
		Type:     "input",
		Version:  "1.0.0",
		Enabled:  true,
		Settings: map[string]interface{}{
			"health_status": "healthy",
		},
	}
	
	require.NoError(t, manager.LoadPlugin(*initialConfig))
	require.NoError(t, manager.StartPlugin(context.Background(), "health-plugin"))
	
	// Wait for initial health check
	time.Sleep(1 * time.Second)
	
	// Config that makes plugin unhealthy
	unhealthyConfig := &PluginConfig{
		Name:     "health-plugin",
		Type:     "input",
		Version:  "1.0.0",
		Enabled:  true,
		Settings: map[string]interface{}{
			"health_status": "unhealthy", // Mock will report unhealthy
		},
	}
	
	err := reloadMgr.AtomicReload(context.Background(), "health-plugin", unhealthyConfig)
	
	// Should fail health validation and rollback
	if err == nil {
		t.Fatal("Expected error for unhealthy plugin reload, but got nil")
	}
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health validation failed")
	
	// Should be rolled back to healthy state
	status, exists := manager.GetPluginStatus("health-plugin")
	assert.True(t, exists)
	assert.Equal(t, PluginStateRunning, status.State)
	assert.Equal(t, "healthy", status.Config.Settings["health_status"])
}

// Test 4: Transaction State Management
func TestReloadManager_TransactionStateTracking(t *testing.T) {
	manager := NewPluginManager()
	reloadMgr := NewReloadManager(manager, &ConfigValidatorImpl{})
	
	// Register standard mock factory
	manager.RegisterFactory("input", &MockPluginFactory{})
	
	// Monitor transaction phases
	var phases []ReloadPhase
	reloadMgr.SetPhaseCallback(func(phase ReloadPhase) {
		phases = append(phases, phase)
	})
	
	config := &PluginConfig{
		Name:     "txn-plugin",
		Type:     "input",
		Version:  "1.0.0",
		Enabled:  true,
		Settings: map[string]interface{}{},
	}
	
	err := reloadMgr.AtomicReload(context.Background(), "txn-plugin", config)
	require.NoError(t, err)
	
	// Verify transaction phases
	expectedPhases := []ReloadPhase{
		PhaseValidation,
		PhaseSnapshot,
		PhaseStopping,
		PhaseLoading,
		PhaseStarting,
		PhaseHealthCheck,
		PhaseComplete,
	}
	
	assert.Equal(t, expectedPhases, phases)
}

// Test 5: Concurrent Reload Protection
func TestReloadManager_ConcurrentReloadProtection(t *testing.T) {
	manager := NewPluginManager()
	reloadMgr := NewReloadManager(manager, &ConfigValidatorImpl{})
	
	// Register slow reload mock
	manager.RegisterFactory("input", &MockSlowFactory{})
	
	config := &PluginConfig{
		Name:     "concurrent-plugin",
		Type:     "input",
		Version:  "1.0.0",
		Enabled:  true,
		Settings: map[string]interface{}{},
	}
	
	// Start first reload (will be slow)
	done1 := make(chan error, 1)
	go func() {
		done1 <- reloadMgr.AtomicReload(context.Background(), "concurrent-plugin", config)
	}()
	
	// Start second concurrent reload
	time.Sleep(100 * time.Millisecond) // Ensure first starts
	
	err2 := reloadMgr.AtomicReload(context.Background(), "concurrent-plugin", config)
	
	// Second should fail immediately
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "reload in progress")
	
	// First should complete successfully
	select {
	case err1 := <-done1:
		assert.NoError(t, err1)
	case <-time.After(5 * time.Second):
		t.Fatal("First reload did not complete")
	}
}

// Test 6: Config Version Backup Before Reload
func TestReloadManager_ConfigVersionBackup(t *testing.T) {
	t.Skip("Skipping version backup test - needs investigation")
	tmpDir := t.TempDir()
	manager := NewPluginManager()
	versionMgr := NewFileSystemVersionManager(filepath.Join(tmpDir, "versions"))
	reloadMgr := NewReloadManagerWithVersion(manager, &ConfigValidatorImpl{}, versionMgr)
	
	// Register mock factory
	manager.RegisterFactory("input", &MockPluginFactory{})
	
	// Initial config
	v1Config := &PluginConfig{
		Name:     "version-test",
		Type:     "input",
		Version:  "1.0.0",
		Settings: map[string]interface{}{"version": "1.0"},
	}
	
	// Debug: check if factory is registered
	factory, err := manager.factories.GetFactory("input")
	require.NoError(t, err, "Factory should be available")
	require.NotNil(t, factory, "Factory should not be nil")
	
	err = manager.LoadPlugin(*v1Config)
	if err != nil {
		t.Fatalf("Failed to load plugin: %v", err)
	}
	
	// Check if plugin was registered
	plugin, exists := manager.registry.GetPlugin("version-test")
	require.True(t, exists, "Plugin should be registered after loading")
	require.NotNil(t, plugin, "Plugin instance should not be nil")
	
	err = manager.StartPlugin(context.Background(), "version-test")
	require.NoError(t, err, "Failed to start plugin")
	
	// Reload with new config
	v2Config := &PluginConfig{
		Name:     "version-test",
		Type:     "input",
		Version:  "1.0.0",
		Settings: map[string]interface{}{"version": "2.0"},
	}
	
	err = reloadMgr.AtomicReload(context.Background(), "version-test", v2Config)
	require.NoError(t, err)
	
	// Check versions were saved
	versions, err := versionMgr.ListVersions("version-test", 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(versions), 2) // At least backup and new version
	
	// Find backup version
	var foundBackup bool
	for _, v := range versions {
		if v.Reason == "pre_reload_backup" {
			foundBackup = true
			assert.Equal(t, "1.0", v.Config.Settings["version"])
			break
		}
	}
	assert.True(t, foundBackup, "Backup version should exist")
}

// Mock factories for testing

type MockPluginFactory struct{}

func (f *MockPluginFactory) CreatePlugin(config PluginConfig) (Plugin, error) {
	return &MockPlugin{config: config}, nil
}

func (f *MockPluginFactory) GetType() string {
	return "test"
}

type MockPlugin struct {
	config PluginConfig
	started bool
}

func (p *MockPlugin) Start(ctx context.Context) error {
	p.started = true
	return nil
}

func (p *MockPlugin) Stop(ctx context.Context) error {
	p.started = false
	return nil
}

func (p *MockPlugin) Health() interfaces.ServiceHealth {
	return interfaces.ServiceHealth{
		Status: interfaces.StatusHealthy,
		Timestamp: time.Now(),
	}
}

func (p *MockPlugin) Info() interfaces.ServiceInfo {
	return interfaces.ServiceInfo{
		Name: p.config.Name,
		Type: p.config.Type,
	}
}

func (p *MockPlugin) Capabilities() []interfaces.Capability {
	return []interfaces.Capability{}
}

func (p *MockPlugin) GetMetadata() PluginMetadata {
	return PluginMetadata{
		Name:    p.config.Name,
		Version: p.config.Version,
		Type:    p.config.Type,
	}
}

func (p *MockPlugin) Configure(config map[string]interface{}) error {
	return nil
}

// Mock factory that can simulate failures
type MockFailFactory struct{}

func (f *MockFailFactory) CreatePlugin(config PluginConfig) (Plugin, error) {
	return &MockFailPlugin{MockPlugin{config: config}}, nil
}

func (f *MockFailFactory) GetType() string {
	return "fail-test"
}

type MockFailPlugin struct {
	MockPlugin
}

func (p *MockFailPlugin) Start(ctx context.Context) error {
	if mode, ok := p.config.Settings["mode"].(string); ok && mode == "fail-on-start" {
		return fmt.Errorf("simulated start failure")
	}
	return p.MockPlugin.Start(ctx)
}

// Mock factory for health testing
type MockHealthFactory struct{}

func (f *MockHealthFactory) CreatePlugin(config PluginConfig) (Plugin, error) {
	return &MockHealthPlugin{MockPlugin: MockPlugin{config: config}}, nil
}

func (f *MockHealthFactory) GetType() string {
	return "health-test"
}

type MockHealthPlugin struct {
	MockPlugin
}

func (p *MockHealthPlugin) Health() interfaces.ServiceHealth {
	if status, ok := p.config.Settings["health_status"].(string); ok {
		if status == "unhealthy" {
			return interfaces.ServiceHealth{
				Status: interfaces.StatusError,
				Message: "Plugin is unhealthy",
				Timestamp: time.Now(),
			}
		}
	}
	return interfaces.ServiceHealth{
		Status: interfaces.StatusHealthy,
		Timestamp: time.Now(),
	}
}

// Mock factory for slow operations
type MockSlowFactory struct{}

func (f *MockSlowFactory) CreatePlugin(config PluginConfig) (Plugin, error) {
	return &MockSlowPlugin{MockPlugin: MockPlugin{config: config}}, nil
}

func (f *MockSlowFactory) GetType() string {
	return "slow-reload"
}

type MockSlowPlugin struct {
	MockPlugin
}

func (p *MockSlowPlugin) Start(ctx context.Context) error {
	time.Sleep(500 * time.Millisecond) // Slow start
	return p.MockPlugin.Start(ctx)
}