package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sho7650/media-sync/internal/core"
)

func TestCoreIntegration_MediaItemLifecycle(t *testing.T) {
	t.Run("Complete media item lifecycle", func(t *testing.T) {
		// Create a media item
		media := core.MediaItem{
			ID:          "integration-test-001",
			URL:         "https://example.com/integration-test.jpg",
			ContentType: "image/jpeg",
			Metadata: map[string]interface{}{
				"source": "integration-test",
				"size":   1024,
			},
			CreatedAt: time.Now(),
		}

		// Validate media item
		err := media.Validate()
		require.NoError(t, err)

		// Verify metadata
		assert.Equal(t, "integration-test-001", media.ID)
		assert.Equal(t, "https://example.com/integration-test.jpg", media.URL)
		assert.Equal(t, "image/jpeg", media.ContentType)
		assert.Contains(t, media.Metadata, "source")
		assert.Equal(t, "integration-test", media.Metadata["source"])
	})

	t.Run("Media item validation edge cases", func(t *testing.T) {
		testCases := []struct {
			name    string
			media   core.MediaItem
			wantErr bool
		}{
			{
				name: "valid media item",
				media: core.MediaItem{
					ID:  "valid-001",
					URL: "https://example.com/valid.jpg",
				},
				wantErr: false,
			},
			{
				name: "empty ID should fail",
				media: core.MediaItem{
					ID:  "",
					URL: "https://example.com/valid.jpg",
				},
				wantErr: true,
			},
			{
				name: "empty URL should fail",
				media: core.MediaItem{
					ID:  "valid-001",
					URL: "",
				},
				wantErr: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.media.Validate()
				if tc.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestCoreIntegration_ServiceConfiguration(t *testing.T) {
	t.Run("Service configuration validation", func(t *testing.T) {
		config := core.ServiceConfig{
			Name:    "tumblr-input",
			Type:    "input",
			Plugin:  "tumblr",
			Enabled: true,
			Settings: map[string]interface{}{
				"username": "testuser",
				"api_key":  "test-api-key",
			},
		}

		err := config.Validate()
		require.NoError(t, err)

		// Verify configuration structure
		assert.Equal(t, "tumblr-input", config.Name)
		assert.Equal(t, "input", config.Type)
		assert.Equal(t, "tumblr", config.Plugin)
		assert.True(t, config.Enabled)
		assert.Contains(t, config.Settings, "username")
		assert.Equal(t, "testuser", config.Settings["username"])
	})
}

func TestCoreIntegration_PluginMetadata(t *testing.T) {
	t.Run("Plugin metadata structure", func(t *testing.T) {
		metadata := core.PluginMetadata{
			Name:        "tumblr-input-plugin",
			Version:     "1.0.0",
			Type:        "input",
			Description: "Tumblr input plugin for media synchronization",
		}

		// Verify metadata structure
		assert.Equal(t, "tumblr-input-plugin", metadata.Name)
		assert.Equal(t, "1.0.0", metadata.Version)
		assert.Equal(t, "input", metadata.Type)
		assert.Contains(t, metadata.Description, "Tumblr")
	})
}

// MockInputService for integration testing
type mockInputService struct {
	metadata core.PluginMetadata
}

func (m *mockInputService) FetchMedia(ctx context.Context, config map[string]interface{}) ([]core.MediaItem, error) {
	// Simulate media fetch
	media := core.MediaItem{
		ID:          "mock-001",
		URL:         "https://example.com/mock.jpg",
		ContentType: "image/jpeg",
		Metadata: map[string]interface{}{
			"source": "mock-service",
		},
		CreatedAt: time.Now(),
	}

	return []core.MediaItem{media}, nil
}

func (m *mockInputService) GetMetadata() core.PluginMetadata {
	return m.metadata
}

func TestCoreIntegration_InputServiceContract(t *testing.T) {
	t.Run("Input service implementation", func(t *testing.T) {
		// Create mock input service
		service := &mockInputService{
			metadata: core.PluginMetadata{
				Name:        "mock-input",
				Version:     "1.0.0",
				Type:        "input",
				Description: "Mock input service for testing",
			},
		}

		// Test interface compliance
		var _ core.InputService = service

		// Test metadata
		metadata := service.GetMetadata()
		assert.Equal(t, "mock-input", metadata.Name)
		assert.Equal(t, "input", metadata.Type)

		// Test media fetching
		ctx := context.Background()
		config := map[string]interface{}{
			"test": "config",
		}

		mediaItems, err := service.FetchMedia(ctx, config)
		require.NoError(t, err)
		require.Len(t, mediaItems, 1)

		media := mediaItems[0]
		assert.Equal(t, "mock-001", media.ID)
		assert.Equal(t, "https://example.com/mock.jpg", media.URL)
		assert.Equal(t, "image/jpeg", media.ContentType)

		// Validate fetched media
		err = media.Validate()
		require.NoError(t, err)
	})
}