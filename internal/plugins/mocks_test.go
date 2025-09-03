package plugins

import (
	"context"

	"github.com/sho7650/media-sync/pkg/core/interfaces"
)

// mockPlugin is a shared mock implementation for testing
type mockPlugin struct {
	metadata PluginMetadata
	started  bool
	stopped  bool
	config   map[string]interface{}
}

func (p *mockPlugin) Start(ctx context.Context) error {
	p.started = true
	return nil
}

func (p *mockPlugin) Stop(ctx context.Context) error {
	p.stopped = true
	return nil
}

func (p *mockPlugin) Health() interfaces.ServiceHealth {
	status := interfaces.StatusHealthy
	if !p.started {
		status = interfaces.StatusStopped
	}
	return interfaces.ServiceHealth{
		Status: status,
	}
}

func (p *mockPlugin) Info() interfaces.ServiceInfo {
	return interfaces.ServiceInfo{
		Name:        p.metadata.Name,
		Version:     p.metadata.Version,
		Type:        p.metadata.Type,
		Description: p.metadata.Description,
	}
}

func (p *mockPlugin) Capabilities() []interfaces.Capability {
	return []interfaces.Capability{}
}

func (p *mockPlugin) GetMetadata() PluginMetadata {
	return p.metadata
}

func (p *mockPlugin) Configure(config map[string]interface{}) error {
	p.config = config
	return nil
}

// mockPluginFactory is a shared mock factory for testing
type mockPluginFactory struct {
	pluginType string
	createFunc func(PluginConfig) (Plugin, error)
}

func (f *mockPluginFactory) CreatePlugin(config PluginConfig) (Plugin, error) {
	if f.createFunc != nil {
		return f.createFunc(config)
	}
	return &mockPlugin{
		metadata: PluginMetadata{
			Name:        config.Name,
			Type:        f.pluginType,
			Version:     config.Version,
			Description: config.Description,
		},
	}, nil
}

func (f *mockPluginFactory) GetType() string {
	return f.pluginType
}