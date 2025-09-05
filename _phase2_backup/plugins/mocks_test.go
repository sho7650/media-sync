package plugins

import (
	"context"

	"github.com/sho7650/media-sync/internal/core"
)

// mockPlugin is a simplified mock for Phase 1 testing
type mockPlugin struct {
	metadata core.PluginMetadata
	initialized bool
	config   map[string]interface{}
}

func (p *mockPlugin) Initialize() error {
	p.initialized = true
	return nil
}

func (p *mockPlugin) Cleanup() error {
	p.initialized = false
	return nil
}

func (p *mockPlugin) GetMetadata() core.PluginMetadata {
	return p.metadata
}

func (p *mockPlugin) Configure(config map[string]interface{}) error {
	p.config = config
	return nil
}

// mockInputPlugin for input testing
type mockInputPlugin struct {
	mockPlugin
}

func (p *mockInputPlugin) FetchMedia(ctx context.Context, config map[string]interface{}) ([]core.MediaItem, error) {
	media := core.MediaItem{
		ID:          "mock-001",
		URL:         "https://example.com/mock.jpg", 
		ContentType: "image/jpeg",
	}
	return []core.MediaItem{media}, nil
}

// mockOutputPlugin for output testing  
type mockOutputPlugin struct {
	mockPlugin
}

func (p *mockOutputPlugin) SendMedia(ctx context.Context, media []core.MediaItem, config map[string]interface{}) error {
	return nil
}

// mockPluginFactory for testing plugin creation
type mockPluginFactory struct {
	pluginType string
}

func (f *mockPluginFactory) CreatePlugin(config PluginConfig) (Plugin, error) {
	metadata := core.PluginMetadata{
		Name:        config.Name,
		Type:        f.pluginType,
		Version:     config.Version,
		Description: config.Description,
	}

	if f.pluginType == "input" {
		return &mockInputPlugin{
			mockPlugin: mockPlugin{metadata: metadata},
		}, nil
	}
	
	return &mockOutputPlugin{
		mockPlugin: mockPlugin{metadata: metadata},
	}, nil
}

func (f *mockPluginFactory) GetType() string {
	return f.pluginType
}