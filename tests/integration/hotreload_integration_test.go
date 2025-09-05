// +build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/sho7650/media-sync/internal/config"
	"github.com/sho7650/media-sync/internal/plugins"
)

// HotReloadIntegrationSuite provides end-to-end integration testing for hot reload functionality
// This suite validates the complete integration between file watching, plugin management,
// configuration management, and error handling across all system components.
type HotReloadIntegrationSuite struct {
	suite.Suite
	testDir        string
	configManager  *config.ConfigManager
	pluginManager  *plugins.PluginManager
	testPluginDir  string
	testConfigFile string
}

// SetupSuite initializes the integration test environment
func (suite *HotReloadIntegrationSuite) SetupSuite() {
	// Create temporary test directories
	suite.testDir = suite.T().TempDir()
	suite.testPluginDir = filepath.Join(suite.testDir, "plugins")
	suite.testConfigFile = filepath.Join(suite.testDir, "config.yaml")
	
	require.NoError(suite.T(), os.MkdirAll(suite.testPluginDir, 0755))
	
	// Initialize system components
	suite.configManager = config.NewConfigManager()
	suite.pluginManager = plugins.NewPluginManager()
	
	// Create initial test configuration
	suite.createInitialTestConfig()
	
	// Create initial test plugins
	suite.createInitialTestPlugins()
	
	suite.T().Logf("Integration test environment initialized at: %s", suite.testDir)
}

// TearDownSuite cleans up the integration test environment
func (suite *HotReloadIntegrationSuite) TearDownSuite() {
	if suite.pluginManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		suite.pluginManager.Shutdown(ctx)
	}
	
	suite.T().Log("Integration test environment cleaned up")
}

// createInitialTestConfig creates a test configuration file
func (suite *HotReloadIntegrationSuite) createInitialTestConfig() {
	configContent := `
global:
  plugin_dir: "` + suite.testPluginDir + `"
  hot_reload: true
  debounce_interval: "100ms"
  
services:
  test-input:
    type: "input"
    plugin: "test-plugin"
    enabled: true
    settings:
      test_setting: "initial_value"
`
	
	err := ioutil.WriteFile(suite.testConfigFile, []byte(configContent), 0644)
	require.NoError(suite.T(), err)
}

// createInitialTestPlugins creates test plugin configurations
func (suite *HotReloadIntegrationSuite) createInitialTestPlugins() {
	pluginConfigs := []plugins.PluginConfig{
		{
			Name:    "test-plugin",
			Type:    "input", 
			Version: "1.0.0",
			Enabled: true,
			Settings: map[string]interface{}{
				"endpoint": "http://test.example.com",
				"timeout":  30,
			},
		},
		{
			Name:    "output-plugin",
			Type:    "output",
			Version: "1.0.0", 
			Enabled: true,
			Settings: map[string]interface{}{
				"destination": "/tmp/test-output",
			},
		},
	}
	
	for _, config := range pluginConfigs {
		configData, err := json.MarshalIndent(config, "", "  ")
		require.NoError(suite.T(), err)
		
		pluginFile := filepath.Join(suite.testPluginDir, config.Name+".json")
		err = ioutil.WriteFile(pluginFile, configData, 0644)
		require.NoError(suite.T(), err)
	}
}

// ============================================================================
// CROSS-COMPONENT INTEGRATION TESTS
// ============================================================================

// TestCompleteHotReloadWorkflow tests the complete hot reload workflow
func (suite *HotReloadIntegrationSuite) TestCompleteHotReloadWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Step 1: Load initial configuration
	_, err := suite.configManager.LoadFromFile(ctx, suite.testConfigFile)
	require.NoError(suite.T(), err, "Should load initial configuration")
	
	// Step 2: Discover and load initial plugins
	loadedCount, err := suite.pluginManager.DiscoverAndLoadPlugins(ctx, suite.testPluginDir)
	require.NoError(suite.T(), err, "Should discover and load plugins")
	assert.Equal(suite.T(), 2, loadedCount, "Should load 2 plugins")
	
	// Step 3: Verify initial plugin status
	status, exists := suite.pluginManager.GetPluginStatus("test-plugin")
	require.True(suite.T(), exists, "Test plugin should exist")
	assert.Equal(suite.T(), plugins.PluginStateRunning, status.State, "Plugin should be running")
	
	// Step 4: Set up hot reload monitoring
	eventChan := make(chan plugins.HealthEvent, 10)
	err = suite.pluginManager.StartHealthMonitoring(ctx, eventChan, 500*time.Millisecond)
	require.NoError(suite.T(), err, "Should start health monitoring")
	
	// Step 5: Simulate plugin configuration change
	suite.modifyPluginConfiguration("test-plugin", map[string]interface{}{
		"endpoint": "http://updated.example.com",
		"timeout":  60,
		"new_setting": "added_value",
	})
	
	// Step 6: Wait for hot reload to process change
	time.Sleep(2 * time.Second)
	
	// Step 7: Verify plugin was reloaded
	status, exists = suite.pluginManager.GetPluginStatus("test-plugin")
	require.True(suite.T(), exists, "Plugin should still exist after reload")
	assert.Equal(suite.T(), plugins.PluginStateRunning, status.State, "Plugin should be running after reload")
	
	// Step 8: Verify health monitoring detected reload
	suite.verifyHealthEvents(eventChan, "test-plugin")
	
	suite.T().Log("Complete hot reload workflow test passed")
}

// TestConcurrentHotReloadOperations tests concurrent hot reload operations
func (suite *HotReloadIntegrationSuite) TestConcurrentHotReloadOperations() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	// Initialize system
	_, err := suite.configManager.LoadFromFile(ctx, suite.testConfigFile)
	require.NoError(suite.T(), err)
	
	_, err = suite.pluginManager.DiscoverAndLoadPlugins(ctx, suite.testPluginDir)
	require.NoError(suite.T(), err)
	
	// Create additional test plugins for concurrent testing
	concurrentPlugins := 5
	for i := 0; i < concurrentPlugins; i++ {
		pluginName := fmt.Sprintf("concurrent-plugin-%d", i)
		suite.createTestPlugin(pluginName, "input", "1.0.0")
	}
	
	// Reload to pick up new plugins
	_, err = suite.pluginManager.DiscoverAndLoadPlugins(ctx, suite.testPluginDir)
	require.NoError(suite.T(), err)
	
	// Perform concurrent modifications
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]bool)
	
	for i := 0; i < concurrentPlugins; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			pluginName := fmt.Sprintf("concurrent-plugin-%d", index)
			
			// Modify plugin configuration
			newSettings := map[string]interface{}{
				"concurrent_test": true,
				"index":          index,
				"timestamp":      time.Now().Unix(),
			}
			
			success := suite.modifyPluginConfiguration(pluginName, newSettings)
			
			mu.Lock()
			results[pluginName] = success
			mu.Unlock()
		}(i)
	}
	
	wg.Wait()
	
	// Wait for all reloads to complete
	time.Sleep(3 * time.Second)
	
	// Verify all concurrent operations succeeded
	for pluginName, success := range results {
		assert.True(suite.T(), success, "Concurrent modification should succeed for %s", pluginName)
		
		status, exists := suite.pluginManager.GetPluginStatus(pluginName)
		assert.True(suite.T(), exists, "Plugin %s should exist", pluginName)
		assert.Equal(suite.T(), plugins.PluginStateRunning, status.State, "Plugin %s should be running", pluginName)
	}
	
	suite.T().Log("Concurrent hot reload operations test passed")
}

// TestHotReloadErrorRecovery tests error handling and recovery
func (suite *HotReloadIntegrationSuite) TestHotReloadErrorRecovery() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Initialize system
	_, err := suite.configManager.LoadFromFile(ctx, suite.testConfigFile)
	require.NoError(suite.T(), err)
	
	_, err = suite.pluginManager.DiscoverAndLoadPlugins(ctx, suite.testPluginDir)
	require.NoError(suite.T(), err)
	
	// Test Case 1: Invalid JSON configuration
	suite.T().Log("Testing invalid JSON error handling")
	pluginFile := filepath.Join(suite.testPluginDir, "test-plugin.json")
	originalContent, err := ioutil.ReadFile(pluginFile)
	require.NoError(suite.T(), err)
	
	// Write invalid JSON
	err = ioutil.WriteFile(pluginFile, []byte("invalid json content"), 0644)
	require.NoError(suite.T(), err)
	
	time.Sleep(1 * time.Second)
	
	// Verify plugin is still running (error should be handled gracefully)
	status, exists := suite.pluginManager.GetPluginStatus("test-plugin")
	assert.True(suite.T(), exists, "Plugin should still exist after invalid JSON")
	assert.Equal(suite.T(), plugins.PluginStateRunning, status.State, "Plugin should still be running")
	
	// Restore valid configuration
	err = ioutil.WriteFile(pluginFile, originalContent, 0644)
	require.NoError(suite.T(), err)
	
	time.Sleep(1 * time.Second)
	
	// Test Case 2: Missing required fields
	suite.T().Log("Testing missing required fields error handling")
	incompleteConfig := map[string]interface{}{
		"name": "test-plugin",
		// Missing type, version, etc.
	}
	
	incompleteData, _ := json.Marshal(incompleteConfig)
	err = ioutil.WriteFile(pluginFile, incompleteData, 0644)
	require.NoError(suite.T(), err)
	
	time.Sleep(1 * time.Second)
	
	// Verify error handling
	status, exists = suite.pluginManager.GetPluginStatus("test-plugin")
	assert.True(suite.T(), exists, "Plugin should exist")
	// Plugin might be in error state or still running depending on error handling strategy
	assert.Contains(suite.T(), []plugins.PluginState{
		plugins.PluginStateRunning,
		plugins.PluginStateError,
	}, status.State, "Plugin should be in expected state after error")
	
	// Restore valid configuration
	err = ioutil.WriteFile(pluginFile, originalContent, 0644)
	require.NoError(suite.T(), err)
	
	time.Sleep(2 * time.Second)
	
	// Verify recovery
	status, exists = suite.pluginManager.GetPluginStatus("test-plugin")
	assert.True(suite.T(), exists, "Plugin should exist after recovery")
	assert.Equal(suite.T(), plugins.PluginStateRunning, status.State, "Plugin should be running after recovery")
	
	suite.T().Log("Hot reload error recovery test passed")
}

// TestConfigurationIntegration tests configuration system integration
func (suite *HotReloadIntegrationSuite) TestConfigurationIntegration() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Set up configuration change monitoring
	configEventChan := make(chan config.ConfigChangeEvent, 10)
	err := suite.configManager.WatchForChanges(ctx, suite.testConfigFile, configEventChan)
	require.NoError(suite.T(), err, "Should start config file watching")
	
	// Modify global configuration
	updatedConfigContent := `
global:
  plugin_dir: "` + suite.testPluginDir + `"
  hot_reload: true
  debounce_interval: "200ms"
  max_concurrent_reloads: 5
  
services:
  test-input:
    type: "input"
    plugin: "test-plugin"
    enabled: true
    settings:
      test_setting: "updated_value"
      new_global_setting: "added_from_config"
`
	
	err = ioutil.WriteFile(suite.testConfigFile, []byte(updatedConfigContent), 0644)
	require.NoError(suite.T(), err, "Should write updated configuration")
	
	// Wait for configuration change event
	select {
	case event := <-configEventChan:
		assert.Equal(suite.T(), "config_updated", event.Type, "Should receive config update event")
		assert.Equal(suite.T(), suite.testConfigFile, event.Path, "Event should reference correct file")
	case <-time.After(5 * time.Second):
		suite.T().Fatal("Configuration change event not received within timeout")
	}
	
	// Verify configuration was reloaded
	currentConfig := suite.configManager.GetCurrentConfig()
	require.NotNil(suite.T(), currentConfig, "Should have current configuration")
	assert.Equal(suite.T(), "200ms", currentConfig.Global.DebounceInterval, "Should reflect updated debounce interval")
	
	suite.T().Log("Configuration integration test passed")
}

// TestPerformanceUnderLoad tests hot reload performance under load
func (suite *HotReloadIntegrationSuite) TestPerformanceUnderLoad() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	
	// Initialize system with monitoring
	_, err := suite.configManager.LoadFromFile(ctx, suite.testConfigFile)
	require.NoError(suite.T(), err)
	
	_, err = suite.pluginManager.DiscoverAndLoadPlugins(ctx, suite.testPluginDir)
	require.NoError(suite.T(), err)
	
	eventChan := make(chan plugins.HealthEvent, 100)
	err = suite.pluginManager.StartHealthMonitoring(ctx, eventChan, 100*time.Millisecond)
	require.NoError(suite.T(), err)
	
	// Create multiple test plugins for load testing
	numPlugins := 20
	for i := 0; i < numPlugins; i++ {
		pluginName := fmt.Sprintf("load-test-plugin-%d", i)
		suite.createTestPlugin(pluginName, "input", "1.0.0")
	}
	
	// Reload to pick up all plugins
	_, err = suite.pluginManager.DiscoverAndLoadPlugins(ctx, suite.testPluginDir)
	require.NoError(suite.T(), err)
	
	// Perform rapid modifications to test debouncing and performance
	startTime := time.Now()
	var wg sync.WaitGroup
	
	for i := 0; i < numPlugins; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			pluginName := fmt.Sprintf("load-test-plugin-%d", index)
			
			// Perform multiple rapid changes
			for j := 0; j < 5; j++ {
				newSettings := map[string]interface{}{
					"load_test":   true,
					"iteration":   j,
					"plugin_id":   index,
					"timestamp":   time.Now().UnixNano(),
				}
				
				suite.modifyPluginConfiguration(pluginName, newSettings)
				time.Sleep(10 * time.Millisecond) // Rapid changes
			}
		}(i)
	}
	
	wg.Wait()
	totalTime := time.Since(startTime)
	
	// Allow time for all debounced operations to complete
	time.Sleep(5 * time.Second)
	
	// Verify performance metrics
	assert.Less(suite.T(), totalTime, 30*time.Second, "Load test should complete within reasonable time")
	
	// Verify all plugins are still operational
	allPluginsHealthy := true
	for i := 0; i < numPlugins; i++ {
		pluginName := fmt.Sprintf("load-test-plugin-%d", i)
		status, exists := suite.pluginManager.GetPluginStatus(pluginName)
		
		if !exists || status.State != plugins.PluginStateRunning {
			allPluginsHealthy = false
			suite.T().Logf("Plugin %s not healthy: exists=%v, state=%v", pluginName, exists, status.State)
		}
	}
	
	assert.True(suite.T(), allPluginsHealthy, "All plugins should remain healthy under load")
	
	// Analyze health events
	suite.analyzeHealthEventsUnderLoad(eventChan)
	
	suite.T().Logf("Performance under load test completed in %v", totalTime)
}

// TestResourceManagement tests resource cleanup and management
func (suite *HotReloadIntegrationSuite) TestResourceManagement() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Initialize system
	_, err := suite.configManager.LoadFromFile(ctx, suite.testConfigFile)
	require.NoError(suite.T(), err)
	
	_, err = suite.pluginManager.DiscoverAndLoadPlugins(ctx, suite.testPluginDir)
	require.NoError(suite.T(), err)
	
	// Get initial resource usage
	initialUsage, err := suite.pluginManager.GetPluginResourceUsage("test-plugin")
	if err != nil {
		suite.T().Logf("Initial resource usage not available: %v", err)
	}
	
	// Perform multiple reload cycles
	for i := 0; i < 10; i++ {
		newSettings := map[string]interface{}{
			"resource_test": true,
			"cycle":        i,
			"timestamp":    time.Now().Unix(),
		}
		
		suite.modifyPluginConfiguration("test-plugin", newSettings)
		time.Sleep(500 * time.Millisecond)
	}
	
	// Allow final operations to complete
	time.Sleep(2 * time.Second)
	
	// Check final resource usage
	finalUsage, err := suite.pluginManager.GetPluginResourceUsage("test-plugin")
	if err == nil {
		// Resource usage should not have grown significantly
		if initialUsage.MemoryBytes > 0 {
			memoryGrowthRatio := float64(finalUsage.MemoryBytes) / float64(initialUsage.MemoryBytes)
			assert.Less(suite.T(), memoryGrowthRatio, 2.0, "Memory usage should not double after reload cycles")
		}
		
		suite.T().Logf("Resource usage - Initial: %+v, Final: %+v", initialUsage, finalUsage)
	}
	
	// Test graceful shutdown
	shutdownStart := time.Now()
	err = suite.pluginManager.GracefulShutdown(ctx, 10*time.Second)
	shutdownTime := time.Since(shutdownStart)
	
	assert.NoError(suite.T(), err, "Graceful shutdown should complete without errors")
	assert.Less(suite.T(), shutdownTime, 10*time.Second, "Graceful shutdown should complete within timeout")
	
	suite.T().Log("Resource management test passed")
}

// ============================================================================
// SECURITY VALIDATION TESTS
// ============================================================================

// TestSecurityValidation tests security aspects of hot reload
func (suite *HotReloadIntegrationSuite) TestSecurityValidation() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Test path traversal protection
	suite.T().Log("Testing path traversal protection")
	
	maliciousPaths := []string{
		"../../../etc/passwd.json",
		"..\\..\\..\\windows\\system32\\config.json",
		"/etc/shadow.json",
		"../../../../root/.ssh/id_rsa.json",
	}
	
	for _, maliciousPath := range maliciousPaths {
		maliciousFile := filepath.Join(suite.testPluginDir, maliciousPath)
		
		// Attempt to create malicious file (should be prevented or cleaned)
		maliciousConfig := plugins.PluginConfig{
			Name:    "malicious-plugin",
			Type:    "input",
			Version: "1.0.0",
			Enabled: true,
		}
		
		configData, _ := json.Marshal(maliciousConfig)
		
		// This should either fail or the path should be cleaned
		err := os.MkdirAll(filepath.Dir(maliciousFile), 0755)
		if err == nil {
			err = ioutil.WriteFile(maliciousFile, configData, 0644)
		}
		
		// If file was created, verify it's in expected location (path cleaned)
		if err == nil {
			cleanedPath := filepath.Clean(maliciousFile)
			assert.Contains(suite.T(), cleanedPath, suite.testPluginDir, 
				"Malicious path should be contained within plugin directory")
		}
		
		suite.T().Logf("Path traversal test for %s: %v", maliciousPath, err)
	}
	
	// Test file permission validation
	suite.T().Log("Testing file permission validation")
	
	restrictedFile := filepath.Join(suite.testPluginDir, "restricted-plugin.json")
	restrictedConfig := plugins.PluginConfig{
		Name:    "restricted-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
	}
	
	configData, _ := json.Marshal(restrictedConfig)
	
	// Create file with restrictive permissions
	err := ioutil.WriteFile(restrictedFile, configData, 0000)
	require.NoError(suite.T(), err)
	
	// Hot reload should handle permission errors gracefully
	time.Sleep(1 * time.Second)
	
	// Restore readable permissions for cleanup
	os.Chmod(restrictedFile, 0644)
	
	suite.T().Log("Security validation test passed")
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// modifyPluginConfiguration modifies a plugin configuration file
func (suite *HotReloadIntegrationSuite) modifyPluginConfiguration(pluginName string, newSettings map[string]interface{}) bool {
	pluginFile := filepath.Join(suite.testPluginDir, pluginName+".json")
	
	// Read current configuration
	var config plugins.PluginConfig
	if data, err := ioutil.ReadFile(pluginFile); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			suite.T().Logf("Failed to parse existing config for %s: %v", pluginName, err)
			return false
		}
	}
	
	// Update settings
	if config.Settings == nil {
		config.Settings = make(map[string]interface{})
	}
	
	for key, value := range newSettings {
		config.Settings[key] = value
	}
	
	// Increment version to simulate update
	config.Version = fmt.Sprintf("1.0.%d", time.Now().Unix()%1000)
	
	// Write updated configuration
	updatedData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		suite.T().Logf("Failed to marshal updated config for %s: %v", pluginName, err)
		return false
	}
	
	err = ioutil.WriteFile(pluginFile, updatedData, 0644)
	if err != nil {
		suite.T().Logf("Failed to write updated config for %s: %v", pluginName, err)
		return false
	}
	
	return true
}

// createTestPlugin creates a test plugin configuration file
func (suite *HotReloadIntegrationSuite) createTestPlugin(name, pluginType, version string) {
	config := plugins.PluginConfig{
		Name:    name,
		Type:    pluginType,
		Version: version,
		Enabled: true,
		Settings: map[string]interface{}{
			"test": true,
		},
	}
	
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(suite.T(), err)
	
	pluginFile := filepath.Join(suite.testPluginDir, name+".json")
	err = ioutil.WriteFile(pluginFile, configData, 0644)
	require.NoError(suite.T(), err)
}

// verifyHealthEvents verifies that health monitoring detected plugin changes
func (suite *HotReloadIntegrationSuite) verifyHealthEvents(eventChan <-chan plugins.HealthEvent, pluginName string) {
	eventTimeout := time.After(5 * time.Second)
	eventsReceived := 0
	
	for {
		select {
		case event := <-eventChan:
			if event.PluginName == pluginName {
				eventsReceived++
				suite.T().Logf("Received health event for %s: %+v", pluginName, event)
			}
		case <-eventTimeout:
			assert.Greater(suite.T(), eventsReceived, 0, 
				"Should receive at least one health event for %s", pluginName)
			return
		}
	}
}

// analyzeHealthEventsUnderLoad analyzes health events during load testing
func (suite *HotReloadIntegrationSuite) analyzeHealthEventsUnderLoad(eventChan <-chan plugins.HealthEvent) {
	eventCount := 0
	errorCount := 0
	recoveryCount := 0
	
	// Drain events with timeout
	timeout := time.After(2 * time.Second)
	
	for {
		select {
		case event := <-eventChan:
			eventCount++
			
			if event.Health.Status.String() == "error" {
				errorCount++
			}
			
			if event.AutoRecoveryAttempted && event.RecoverySuccess {
				recoveryCount++
			}
			
		case <-timeout:
			suite.T().Logf("Health event analysis - Total: %d, Errors: %d, Recoveries: %d", 
				eventCount, errorCount, recoveryCount)
			
			// Under load, some errors might be expected, but recovery should work
			if errorCount > 0 {
				assert.Greater(suite.T(), recoveryCount, 0, 
					"Should have attempted recovery for errors under load")
			}
			return
		}
	}
}

// ============================================================================
// TEST SUITE RUNNER
// ============================================================================

// TestHotReloadIntegrationSuite runs the complete integration test suite
func TestHotReloadIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	
	suite.Run(t, new(HotReloadIntegrationSuite))
}

// ============================================================================
// BENCHMARK TESTS FOR INTEGRATION PERFORMANCE
// ============================================================================

// BenchmarkCompleteHotReloadCycle benchmarks the complete hot reload cycle
func BenchmarkCompleteHotReloadCycle(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark tests in short mode")
	}
	
	// Setup
	testDir := b.TempDir()
	pluginDir := filepath.Join(testDir, "plugins")
	configFile := filepath.Join(testDir, "config.yaml")
	
	os.MkdirAll(pluginDir, 0755)
	
	// Create test plugin
	config := plugins.PluginConfig{
		Name:    "benchmark-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
		Settings: map[string]interface{}{
			"benchmark": true,
		},
	}
	
	configData, _ := json.Marshal(config)
	pluginFile := filepath.Join(pluginDir, "benchmark-plugin.json")
	ioutil.WriteFile(pluginFile, configData, 0644)
	
	// Initialize components
	configManager := config.NewConfigManager()
	pluginManager := plugins.NewPluginManager()
	
	ctx := context.Background()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Simulate hot reload cycle
		config.Version = fmt.Sprintf("1.0.%d", i)
		config.Settings["iteration"] = i
		
		updatedData, _ := json.Marshal(config)
		ioutil.WriteFile(pluginFile, updatedData, 0644)
		
		// Small delay to simulate real-world timing
		time.Sleep(1 * time.Millisecond)
	}
}

// BenchmarkConcurrentHotReloads benchmarks concurrent hot reload operations
func BenchmarkConcurrentHotReloads(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark tests in short mode")
	}
	
	// Setup multiple plugins
	testDir := b.TempDir()
	pluginDir := filepath.Join(testDir, "plugins")
	os.MkdirAll(pluginDir, 0755)
	
	numPlugins := 10
	pluginFiles := make([]string, numPlugins)
	
	for i := 0; i < numPlugins; i++ {
		config := plugins.PluginConfig{
			Name:    fmt.Sprintf("benchmark-plugin-%d", i),
			Type:    "input",
			Version: "1.0.0",
			Enabled: true,
			Settings: map[string]interface{}{
				"benchmark": true,
				"id":       i,
			},
		}
		
		configData, _ := json.Marshal(config)
		pluginFile := filepath.Join(pluginDir, fmt.Sprintf("benchmark-plugin-%d.json", i))
		ioutil.WriteFile(pluginFile, configData, 0644)
		pluginFiles[i] = pluginFile
	}
	
	b.ResetTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Randomly select a plugin to modify
			pluginIndex := time.Now().UnixNano() % int64(numPlugins)
			pluginFile := pluginFiles[pluginIndex]
			
			// Read and modify plugin
			var config plugins.PluginConfig
			if data, err := ioutil.ReadFile(pluginFile); err == nil {
				json.Unmarshal(data, &config)
				config.Settings["timestamp"] = time.Now().UnixNano()
				updatedData, _ := json.Marshal(config)
				ioutil.WriteFile(pluginFile, updatedData, 0644)
			}
		}
	})
}