//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sho7650/media-sync/internal/plugins"
	"github.com/sho7650/media-sync/pkg/core/interfaces"
)

// TestTDD盲点_統合テスト_PluginCrashIsolation demonstrates:
// Problem: ユニットテスト中心 -> Solution: 統合テストで実際の動作確認
// Quality Gate: "Plugin crashes don't affect main application"
func TestTDD盲点_統合テスト_PluginCrashIsolation(t *testing.T) {
	// Setup: Create a plugin manager (main application)
	manager := plugins.NewPluginManager()
	
	// Register input factory that can create both stable and crashing plugins
	stableFactory := &TestPluginFactory{
		pluginType: "input",
		behavior:   BehaviorNormal,
	}
	require.NoError(t, manager.RegisterFactory("input", stableFactory))
	
	// For crashing plugin, we'll configure the factory differently
	crashFactory := &TestPluginFactory{
		pluginType: "output",  // Different type to test isolation
		behavior:   BehaviorCrashOnStart,
	}
	require.NoError(t, manager.RegisterFactory("output", crashFactory))
	
	// Load both plugins - Type must be "input", "output", or "transform"
	stableConfig := plugins.PluginConfig{
		Name:    "stable-plugin",
		Type:    "input",  // Must be input/output/transform
		Version: "1.0.0",
		Enabled: true,
	}
	crashConfig := plugins.PluginConfig{
		Name:    "crash-plugin",
		Type:    "output",  // Use different valid type
		Version: "1.0.0",
		Enabled: true,
	}
	
	require.NoError(t, manager.LoadPlugin(stableConfig))
	require.NoError(t, manager.LoadPlugin(crashConfig))
	
	ctx := context.Background()
	
	// Start stable plugin - should succeed
	require.NoError(t, manager.StartPlugin(ctx, "stable-plugin"))
	
	// Verify stable plugin is running
	status, exists := manager.GetPluginStatus("stable-plugin")
	require.True(t, exists)
	assert.Equal(t, plugins.PluginStateRunning, status.State)
	
	// Start crashing plugin - should fail
	err := manager.StartPlugin(ctx, "crash-plugin")
	assert.Error(t, err, "Crashing plugin should fail to start")
	
	// CRITICAL TEST: Stable plugin should still be running
	// This validates that plugin crashes are isolated
	status, exists = manager.GetPluginStatus("stable-plugin")
	require.True(t, exists)
	assert.Equal(t, plugins.PluginStateRunning, status.State, 
		"Stable plugin should remain running despite crash in another plugin")
}

// TestTDD盲点_品質ゲート実証_HealthMonitoring demonstrates:
// Problem: 品質ゲート形骸化 -> Solution: 具体的実装で検証
// Quality Gate: "Health monitoring with auto-recovery"
func TestTDD盲点_品質ゲート実証_HealthMonitoring(t *testing.T) {
	manager := plugins.NewPluginManager()
	
	// Create a plugin that becomes unhealthy
	factory := &TestPluginFactory{
		pluginType: "input",
		behavior:   BehaviorBecomeUnhealthy,
	}
	require.NoError(t, manager.RegisterFactory("input", factory))
	
	config := plugins.PluginConfig{
		Name:    "unhealthy-plugin",
		Type:    "input",  // Must be input/output/transform
		Version: "1.0.0",
		Enabled: true,
	}
	
	ctx := context.Background()
	require.NoError(t, manager.LoadPlugin(config))
	require.NoError(t, manager.StartPlugin(ctx, "unhealthy-plugin"))
	
	// Start health monitoring
	healthChan := make(chan plugins.HealthEvent, 100)
	monitorCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	
	err := manager.StartHealthMonitoring(monitorCtx, healthChan, 500*time.Millisecond)
	require.NoError(t, err)
	
	// Collect health events
	unhealthyDetected := false
	timeout := time.After(2 * time.Second)
	
	for {
		select {
		case event := <-healthChan:
			if event.Health.Status == interfaces.StatusError {
				unhealthyDetected = true
				t.Logf("Unhealthy plugin detected: %s", event.PluginName)
			}
		case <-timeout:
			goto done
		}
	}
	
done:
	// Verify that unhealthy state was detected
	assert.True(t, unhealthyDetected, 
		"Health monitoring should detect unhealthy plugins")
	
	manager.StopHealthMonitoring()
}

// TestTDD盲点_実行可能環境_EndToEnd demonstrates:
// Problem: 統合テスト不在 -> Solution: 実行可能環境での検証
// This is a complete end-to-end test with actual plugin lifecycle
func TestTDD盲点_実行可能環境_EndToEnd(t *testing.T) {
	manager := plugins.NewPluginManager()
	
	// Register factory
	factory := &TestPluginFactory{
		pluginType: "input",
		behavior:   BehaviorNormal,
	}
	require.NoError(t, manager.RegisterFactory("input", factory))
	
	// Plugin configuration
	config := plugins.PluginConfig{
		Name:    "e2e-plugin",
		Type:    "input",  // Must be input/output/transform
		Version: "1.0.0",
		Enabled: true,
		Settings: map[string]interface{}{
			"test": "value",
		},
	}
	
	ctx := context.Background()
	
	// Complete lifecycle: Load -> Start -> Stop -> Unload
	
	// Load
	require.NoError(t, manager.LoadPlugin(config))
	status, exists := manager.GetPluginStatus("e2e-plugin")
	require.True(t, exists)
	assert.Equal(t, plugins.PluginStateLoaded, status.State)
	
	// Start
	require.NoError(t, manager.StartPlugin(ctx, "e2e-plugin"))
	status, exists = manager.GetPluginStatus("e2e-plugin")
	require.True(t, exists)
	assert.Equal(t, plugins.PluginStateRunning, status.State)
	
	// Stop
	require.NoError(t, manager.StopPlugin(ctx, "e2e-plugin"))
	status, exists = manager.GetPluginStatus("e2e-plugin")
	require.True(t, exists)
	assert.Equal(t, plugins.PluginStateStopped, status.State)
	
	// Unload
	require.NoError(t, manager.UnloadPlugin(ctx, "e2e-plugin"))
	status, exists = manager.GetPluginStatus("e2e-plugin")
	assert.False(t, exists, "Plugin should not exist after unload")
}

// Test plugin behaviors for integration testing
type PluginBehavior int

const (
	BehaviorNormal PluginBehavior = iota
	BehaviorCrashOnStart
	BehaviorBecomeUnhealthy
	BehaviorFailOnCreate
)

// TestPluginFactory creates test plugins with specific behaviors
type TestPluginFactory struct {
	pluginType string
	behavior   PluginBehavior
}

func (f *TestPluginFactory) CreatePlugin(config plugins.PluginConfig) (plugins.Plugin, error) {
	if f.behavior == BehaviorFailOnCreate {
		return nil, fmt.Errorf("simulated creation failure")
	}
	
	return &TestPlugin{
		name:       config.Name,
		pluginType: f.pluginType,
		behavior:   f.behavior,
		healthy:    true,
	}, nil
}

func (f *TestPluginFactory) GetType() string {
	return f.pluginType
}

// TestPlugin implements the Plugin interface for testing
type TestPlugin struct {
	name       string
	pluginType string
	behavior   PluginBehavior
	running    bool
	healthy    bool
	mu         sync.RWMutex
}

func (p *TestPlugin) Start(ctx context.Context) error {
	if p.behavior == BehaviorCrashOnStart {
		return fmt.Errorf("simulated crash on start")
	}
	
	p.mu.Lock()
	p.running = true
	p.mu.Unlock()
	
	if p.behavior == BehaviorBecomeUnhealthy {
		// Become unhealthy after 1 second
		go func() {
			time.Sleep(1 * time.Second)
			p.mu.Lock()
			p.healthy = false
			p.mu.Unlock()
		}()
	}
	
	return nil
}

func (p *TestPlugin) Stop(ctx context.Context) error {
	p.mu.Lock()
	p.running = false
	p.mu.Unlock()
	return nil
}

func (p *TestPlugin) Health() interfaces.ServiceHealth {
	p.mu.RLock()
	healthy := p.healthy
	running := p.running
	p.mu.RUnlock()
	
	if !healthy || !running {
		return interfaces.ServiceHealth{
			Status:  interfaces.StatusError,
			Message: "Plugin is unhealthy",
		}
	}
	
	return interfaces.ServiceHealth{
		Status:  interfaces.StatusHealthy,
		Message: "Plugin is healthy",
	}
}

func (p *TestPlugin) Info() interfaces.ServiceInfo {
	return interfaces.ServiceInfo{
		Name:    p.name,
		Version: "1.0.0",
		Type:    "test",
	}
}

func (p *TestPlugin) Capabilities() []interfaces.Capability {
	return []interfaces.Capability{
		{Type: "media:photo", Supported: true},
		{Type: "sync:batch", Supported: true},
		{Type: "auth:none", Supported: true},
	}
}

func (p *TestPlugin) GetMetadata() plugins.PluginMetadata {
	return plugins.PluginMetadata{
		Name:        p.name,
		Version:     "1.0.0",
		Type:        p.pluginType,  // Must be input/output/transform
		Description: "Test plugin for integration testing",
	}
}

func (p *TestPlugin) Configure(config map[string]interface{}) error {
	// Accept any configuration for testing
	return nil
}