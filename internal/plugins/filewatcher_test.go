package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Helper function to write plugin configuration to file
func writePluginConfig(t *testing.T, path string, config PluginConfig) {
	data, err := yaml.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))
}

func TestFileWatcher_DetectConfigChanges(t *testing.T) {
	tmpDir := t.TempDir()
	eventChan := make(chan PluginFileEvent, 10)
	
	watcher := NewFileWatcher()
	watcher.SetEventHandler(func(event PluginFileEvent) error {
		eventChan <- event
		return nil
	})
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	require.NoError(t, watcher.AddWatchPath(tmpDir))
	require.NoError(t, watcher.Start(ctx))
	defer watcher.Stop()
	
	// Create plugin config file
	configPath := filepath.Join(tmpDir, "test-plugin.yaml")
	config := PluginConfig{
		Name:    "test-plugin",
		Type:    "input",
		Version: "1.0.0",
		Enabled: true,
		Settings: map[string]interface{}{
			"timeout": 5000,
		},
	}
	
	writePluginConfig(t, configPath, config)
	
	select {
	case event := <-eventChan:
		assert.Equal(t, configPath, event.Path)
		assert.Equal(t, fsnotify.Create, event.Operation)
		assert.NotNil(t, event.ConfigDelta)
		assert.Equal(t, "test-plugin", event.ConfigDelta.PluginName)
		assert.Equal(t, DeltaTypeCreate, event.ConfigDelta.ChangeType)
	case <-time.After(2 * time.Second):
		t.Fatal("Expected file create event within 2 seconds")
	}
}

func TestFileWatcher_DetectConfigModification(t *testing.T) {
	tmpDir := t.TempDir()
	eventChan := make(chan PluginFileEvent, 10)
	
	watcher := NewFileWatcher()
	watcher.SetEventHandler(func(event PluginFileEvent) error {
		eventChan <- event
		return nil
	})
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	require.NoError(t, watcher.AddWatchPath(tmpDir))
	require.NoError(t, watcher.Start(ctx))
	defer watcher.Stop()
	
	// Create initial config
	configPath := filepath.Join(tmpDir, "test-plugin.yaml")
	initialConfig := PluginConfig{
		Name:    "test-plugin",
		Version: "1.0.0",
		Settings: map[string]interface{}{
			"timeout": 5000,
			"retries": 3,
		},
	}
	writePluginConfig(t, configPath, initialConfig)
	
	// Wait for create event
	<-eventChan
	
	// Modify config
	modifiedConfig := PluginConfig{
		Name:    "test-plugin",
		Version: "1.1.0", // Changed
		Settings: map[string]interface{}{
			"timeout": 10000, // Changed
			"retries": 3,     // Unchanged
			"debug":   true,  // Added
		},
	}
	writePluginConfig(t, configPath, modifiedConfig)
	
	select {
	case event := <-eventChan:
		// macOS may emit Chmod instead of Write for file modifications
		assert.True(t, event.Operation == fsnotify.Write || event.Operation == fsnotify.Chmod)
		assert.Equal(t, DeltaTypeUpdate, event.ConfigDelta.ChangeType)
		
		changes := event.ConfigDelta.Changes
		assert.Equal(t, 10000, changes["timeout"])
		assert.Equal(t, true, changes["debug"])
		assert.NotContains(t, changes, "retries") // Unchanged field not in delta
		
	case <-time.After(2 * time.Second):
		t.Fatal("Expected file modification event")
	}
}

func TestFileWatcher_IgnoreNonPluginFiles(t *testing.T) {
	tmpDir := t.TempDir()
	eventChan := make(chan PluginFileEvent, 10)
	
	watcher := NewFileWatcher()
	watcher.SetEventHandler(func(event PluginFileEvent) error {
		eventChan <- event
		return nil
	})
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	require.NoError(t, watcher.AddWatchPath(tmpDir))
	require.NoError(t, watcher.Start(ctx))
	defer watcher.Stop()
	
	// Create non-plugin files
	textFile := filepath.Join(tmpDir, "readme.txt")
	require.NoError(t, os.WriteFile(textFile, []byte("This is not a plugin"), 0644))
	
	jsonFile := filepath.Join(tmpDir, "config.json")
	require.NoError(t, os.WriteFile(jsonFile, []byte(`{"not": "plugin"}`), 0644))
	
	// Create plugin file
	configPath := filepath.Join(tmpDir, "real-plugin.yaml")
	config := PluginConfig{Name: "real-plugin", Version: "1.0.0"}
	writePluginConfig(t, configPath, config)
	
	// Should only receive plugin file event
	select {
	case event := <-eventChan:
		assert.Equal(t, configPath, event.Path)
		assert.Equal(t, "real-plugin", event.ConfigDelta.PluginName)
	case <-time.After(2 * time.Second):
		t.Fatal("Expected plugin config event")
	}
	
	// Verify no additional events for non-plugin files
	select {
	case event := <-eventChan:
		t.Fatalf("Unexpected event for non-plugin file: %s", event.Path)
	case <-time.After(100 * time.Millisecond):
		// Expected - no events for non-plugin files
	}
}

func TestFileWatcher_EventDebouncing(t *testing.T) {
	tmpDir := t.TempDir()
	eventChan := make(chan PluginFileEvent, 10)
	
	watcher := NewFileWatcher()
	watcher.SetEventHandler(func(event PluginFileEvent) error {
		eventChan <- event
		return nil
	})
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	require.NoError(t, watcher.AddWatchPath(tmpDir))
	require.NoError(t, watcher.Start(ctx))
	defer watcher.Stop()
	
	configPath := filepath.Join(tmpDir, "debounce-plugin.yaml")
	config := PluginConfig{Name: "debounce-plugin", Version: "1.0.0", Enabled: true}
	
	// Rapid successive writes (simulating editor behavior)
	for i := 0; i < 5; i++ {
		config.Settings = map[string]interface{}{
			"iteration": i,
		}
		writePluginConfig(t, configPath, config)
		time.Sleep(30 * time.Millisecond) // Shorter than debounce delay
	}
	
	// Wait for debounced event (should only get the last one after debounce delay)
	time.Sleep(200 * time.Millisecond) // Wait for debounce to complete
	
	eventCount := 0
	var lastEvent PluginFileEvent
	
	// Collect all events within a short window
	timeout := time.After(300 * time.Millisecond)
	
	for {
		select {
		case event := <-eventChan:
			eventCount++
			lastEvent = event
		case <-timeout:
			goto done
		}
	}
	
done:
	// With debouncing, we should get significantly fewer events than writes
	// Expect at most 2 events (create + one debounced update)
	assert.LessOrEqual(t, eventCount, 2, "Event debouncing should reduce event count")
	
	// The last event should have the final iteration value
	if eventCount > 0 && lastEvent.ConfigDelta != nil && lastEvent.ConfigDelta.NewConfig != nil {
		if lastEvent.ConfigDelta.NewConfig.Settings != nil {
			if iteration, ok := lastEvent.ConfigDelta.NewConfig.Settings["iteration"]; ok {
				assert.Equal(t, 4, iteration, "Should have final iteration value")
			}
		}
	}
}