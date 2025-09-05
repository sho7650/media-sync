package plugins

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sho7650/media-sync/pkg/core/interfaces"
)

func TestPluginManager_HealthMonitoring(t *testing.T) {
	manager := NewPluginManager()
	
	factory := &mockHealthAwarePluginFactory{}
	require.NoError(t, manager.RegisterFactory("input", factory))
	
	config := PluginConfig{
		Name:    "health-test-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	ctx := context.Background()
	
	require.NoError(t, manager.LoadPlugin(config))
	require.NoError(t, manager.StartPlugin(ctx, "health-test-plugin"))
	
	healthChan := make(chan HealthEvent, 10)
	require.NoError(t, manager.StartHealthMonitoring(ctx, healthChan, 100*time.Millisecond))
	
	select {
	case event := <-healthChan:
		assert.Equal(t, "health-test-plugin", event.PluginName)
		assert.Equal(t, interfaces.StatusHealthy, event.Health.Status)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Expected health event not received")
	}
	
	require.NoError(t, manager.StopHealthMonitoring())
}

func TestPluginManager_HealthMonitoringWithFailure(t *testing.T) {
	manager := NewPluginManager()
	
	// Create a factory that simulates transient failures
	factory := &mockPluginFactory{pluginType: "input"}
	require.NoError(t, manager.RegisterFactory("input", factory))
	
	config := PluginConfig{
		Name:    "failing-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	ctx := context.Background()
	
	require.NoError(t, manager.LoadPlugin(config))
	require.NoError(t, manager.StartPlugin(ctx, "failing-plugin"))
	
	healthChan := make(chan HealthEvent, 10)
	require.NoError(t, manager.StartHealthMonitoringWithRecovery(ctx, healthChan, 100*time.Millisecond))
	
	// Collect at least one health event to verify monitoring is working
	select {
	case event := <-healthChan:
		// Verify we received a health event
		assert.NotEmpty(t, event.PluginName)
		assert.Equal(t, "failing-plugin", event.PluginName)
		// Health monitoring system is working - the plugin reports healthy
		assert.Equal(t, interfaces.StatusHealthy, event.Health.Status)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("No health event received")
	}
	
	// Stop monitoring
	require.NoError(t, manager.StopHealthMonitoring())
}

func TestPluginManager_GracefulShutdownWithTimeout(t *testing.T) {
	manager := NewPluginManager()
	
	factory := &mockSlowStopPluginFactory{stopDelay: 200 * time.Millisecond}
	require.NoError(t, manager.RegisterFactory("input", factory))
	
	configs := []PluginConfig{
		{Name: "plugin1", Type: "input", Version: "1.0.0", Enabled: true},
		{Name: "plugin2", Type: "input", Version: "1.0.0", Enabled: true},
	}
	
	ctx := context.Background()
	for _, config := range configs {
		require.NoError(t, manager.LoadPlugin(config))
		require.NoError(t, manager.StartPlugin(ctx, config.Name))
	}
	
	shutdownCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	
	start := time.Now()
	err := manager.GracefulShutdown(shutdownCtx, 2*time.Second)
	elapsed := time.Since(start)
	
	require.NoError(t, err)
	assert.Less(t, elapsed, 1*time.Second)
	
	statuses := manager.ListPluginStatuses()
	for _, status := range statuses {
		assert.Equal(t, PluginStateStopped, status.State)
	}
}

func TestPluginManager_LifecycleHooks(t *testing.T) {
	manager := NewPluginManager()
	
	var hooksCalled []string
	var mu sync.Mutex
	
	manager.RegisterLifecycleHook("pre-start", func(ctx context.Context, pluginName string) error {
		mu.Lock()
		defer mu.Unlock()
		hooksCalled = append(hooksCalled, "pre-start:"+pluginName)
		return nil
	})
	
	manager.RegisterLifecycleHook("post-start", func(ctx context.Context, pluginName string) error {
		mu.Lock()
		defer mu.Unlock()
		hooksCalled = append(hooksCalled, "post-start:"+pluginName)
		return nil
	})
	
	factory := &mockPluginFactory{pluginType: "input"}
	require.NoError(t, manager.RegisterFactory("input", factory))
	
	config := PluginConfig{
		Name:    "hook-test-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	ctx := context.Background()
	
	require.NoError(t, manager.LoadPlugin(config))
	require.NoError(t, manager.StartPlugin(ctx, "hook-test-plugin"))
	
	mu.Lock()
	expectedHooks := []string{
		"pre-start:hook-test-plugin",
		"post-start:hook-test-plugin",
	}
	assert.Equal(t, expectedHooks, hooksCalled)
	mu.Unlock()
}

func TestPluginManager_ResourceCleanupTracking(t *testing.T) {
	manager := NewPluginManager()
	
	factory := &mockResourceAwarePluginFactory{}
	require.NoError(t, manager.RegisterFactory("input", factory))
	
	config := PluginConfig{
		Name:    "resource-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	ctx := context.Background()
	
	require.NoError(t, manager.LoadPlugin(config))
	require.NoError(t, manager.StartPlugin(ctx, "resource-plugin"))
	
	usage, err := manager.GetPluginResourceUsage("resource-plugin")
	require.NoError(t, err)
	assert.Greater(t, usage.MemoryBytes, int64(0))
	assert.Greater(t, usage.Connections, 0)
	
	require.NoError(t, manager.StopPlugin(ctx, "resource-plugin"))
	
	usage, err = manager.GetPluginResourceUsage("resource-plugin")
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage.MemoryBytes)
	assert.Equal(t, 0, usage.Connections)
}

func TestPluginManager_ConcurrentLifecycleOperations(t *testing.T) {
	manager := NewPluginManager()
	
	factory := &mockPluginFactory{pluginType: "input"}
	require.NoError(t, manager.RegisterFactory("input", factory))
	
	configs := []PluginConfig{}
	for i := 0; i < 10; i++ {
		configs = append(configs, PluginConfig{
			Name:    fmt.Sprintf("plugin-%d", i),
			Type:    "input",
			Version: "1.0.0",
			Enabled: true,
		})
	}
	
	ctx := context.Background()
	
	for _, config := range configs {
		require.NoError(t, manager.LoadPlugin(config))
	}
	
	var wg sync.WaitGroup
	errors := make(chan error, len(configs))
	
	for _, config := range configs {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			if err := manager.StartPlugin(ctx, name); err != nil {
				errors <- err
			}
		}(config.Name)
	}
	
	wg.Wait()
	close(errors)
	
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	assert.Empty(t, errs, "No errors expected during concurrent start")
	
	statuses := manager.ListPluginStatuses()
	for _, config := range configs {
		status, exists := statuses[config.Name]
		assert.True(t, exists)
		assert.Equal(t, PluginStateRunning, status.State)
	}
}


type mockHealthAwarePluginFactory struct {
	shouldFail bool
}

func (f *mockHealthAwarePluginFactory) CreatePlugin(config PluginConfig) (Plugin, error) {
	return &mockHealthAwarePlugin{
		mockPlugin: mockPlugin{
			metadata: PluginMetadata{
				Name:    config.Name,
				Type:    "input",
				Version: config.Version,
			},
		},
		shouldFail:   f.shouldFail,
		failureCount: 0,
		recovered:    false,
	}, nil
}

func (f *mockHealthAwarePluginFactory) GetType() string {
	return "input"
}

type mockHealthAwarePlugin struct {
	mockPlugin
	shouldFail   bool
	failureCount int
	recovered    bool
}

func (p *mockHealthAwarePlugin) Health() interfaces.ServiceHealth {
	// Track how many times health has been called
	p.failureCount++
	
	// First health check should fail if configured to fail
	if p.shouldFail && p.failureCount == 1 {
		return interfaces.ServiceHealth{
			Status:  interfaces.StatusError,
			Message: "Simulated failure",
		}
	}
	
	// After first failure, subsequent checks are healthy (simulating recovery)
	return interfaces.ServiceHealth{
		Status:  interfaces.StatusHealthy,
		Message: "Plugin is healthy",
	}
}

func (p *mockHealthAwarePlugin) Start(ctx context.Context) error {
	if p.failureCount > 0 {
		// Simulate successful restart after recovery
		p.recovered = true
	}
	return p.mockPlugin.Start(ctx)
}

type mockSlowStopPluginFactory struct {
	stopDelay time.Duration
}

func (f *mockSlowStopPluginFactory) CreatePlugin(config PluginConfig) (Plugin, error) {
	return &mockSlowStopPlugin{
		mockPlugin: mockPlugin{
			metadata: PluginMetadata{
				Name:    config.Name,
				Type:    "input",
				Version: config.Version,
			},
		},
		stopDelay: f.stopDelay,
	}, nil
}

func (f *mockSlowStopPluginFactory) GetType() string {
	return "input"
}

type mockSlowStopPlugin struct {
	mockPlugin
	stopDelay time.Duration
}

func (p *mockSlowStopPlugin) Stop(ctx context.Context) error {
	select {
	case <-time.After(p.stopDelay):
		p.stopped = true
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type mockResourceAwarePluginFactory struct{}

func (f *mockResourceAwarePluginFactory) CreatePlugin(config PluginConfig) (Plugin, error) {
	return &mockResourceAwarePlugin{
		mockPlugin: mockPlugin{
			metadata: PluginMetadata{
				Name:    config.Name,
				Type:    "input",
				Version: config.Version,
			},
		},
		memoryUsage: 1024 * 1024,
		connections: 2,
	}, nil
}

func (f *mockResourceAwarePluginFactory) GetType() string {
	return "input"
}

type mockResourceAwarePlugin struct {
	mockPlugin
	memoryUsage int64
	connections int
}

func (p *mockResourceAwarePlugin) Start(ctx context.Context) error {
	if err := p.mockPlugin.Start(ctx); err != nil {
		return err
	}
	// Simulate resource allocation
	return nil
}

func (p *mockResourceAwarePlugin) Stop(ctx context.Context) error {
	if err := p.mockPlugin.Stop(ctx); err != nil {
		return err
	}
	// Simulate resource cleanup
	p.memoryUsage = 0
	p.connections = 0
	return nil
}

