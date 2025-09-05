package core

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestInputService_Contract(t *testing.T) {
    // Test that InputService interface can be implemented
    var _ InputService = (*mockInputService)(nil)
}

func TestMediaItem_Validation(t *testing.T) {
    media := MediaItem{
        ID:          "test-123",
        URL:         "https://example.com/image.jpg",
        ContentType: "image/jpeg",
        CreatedAt:   time.Now(),
    }
    
    err := media.Validate()
    require.NoError(t, err)
}

func TestMediaItem_ValidationFailure(t *testing.T) {
    media := MediaItem{
        ID:  "", // Invalid: empty ID
        URL: "https://example.com/image.jpg",
    }
    
    err := media.Validate()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "ID cannot be empty")
}

func TestServiceConfig_Validation(t *testing.T) {
    config := ServiceConfig{
        Name:    "test-service",
        Type:    "input",
        Plugin:  "tumblr",
        Enabled: true,
        Settings: map[string]interface{}{
            "username": "testuser",
        },
    }
    
    err := config.Validate()
    require.NoError(t, err)
}

func TestServiceConfig_ValidationErrors(t *testing.T) {
    tests := []struct {
        name    string
        config  ServiceConfig
        wantErr string
    }{
        {
            name: "empty name",
            config: ServiceConfig{
                Type:   "input",
                Plugin: "tumblr",
            },
            wantErr: "name cannot be empty",
        },
        {
            name: "invalid type",
            config: ServiceConfig{
                Name:   "test",
                Type:   "invalid",
                Plugin: "tumblr",
            },
            wantErr: "type must be 'input' or 'output'",
        },
        {
            name: "empty plugin",
            config: ServiceConfig{
                Name: "test",
                Type: "input",
            },
            wantErr: "plugin cannot be empty",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.config.Validate()
            assert.Error(t, err)
            assert.Contains(t, err.Error(), tt.wantErr)
        })
    }
}

func TestPluginMetadata_Validation(t *testing.T) {
    t.Run("Valid plugin metadata", func(t *testing.T) {
        metadata := PluginMetadata{
            Name:        "tumblr-input",
            Version:     "1.0.0",
            Type:        "input",
            Description: "Tumblr input plugin",
        }
        
        err := metadata.Validate()
        assert.NoError(t, err)
    })
    
    t.Run("Plugin metadata validation errors", func(t *testing.T) {
        tests := []struct {
            name     string
            metadata PluginMetadata
            wantErr  string
        }{
            {
                name: "empty name",
                metadata: PluginMetadata{
                    Version: "1.0.0",
                    Type:    "input",
                },
                wantErr: "name cannot be empty",
            },
            {
                name: "invalid type",
                metadata: PluginMetadata{
                    Name:    "test",
                    Version: "1.0.0",
                    Type:    "invalid",
                },
                wantErr: "type must be 'input' or 'output'",
            },
            {
                name: "empty version",
                metadata: PluginMetadata{
                    Name: "test",
                    Type: "input",
                },
                wantErr: "version cannot be empty",
            },
        }
        
        for _, tt := range tests {
            t.Run(tt.name, func(t *testing.T) {
                err := tt.metadata.Validate()
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.wantErr)
            })
        }
    })
}

type mockInputService struct{}

func (m *mockInputService) FetchMedia(ctx context.Context, config map[string]interface{}) ([]MediaItem, error) {
    return []MediaItem{}, nil
}

func (m *mockInputService) GetMetadata() PluginMetadata {
    return PluginMetadata{Name: "mock", Version: "1.0.0", Type: "input"}
}