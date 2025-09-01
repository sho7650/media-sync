package examples

// TDD Examples for Media-Sync Project
// This file contains concrete examples of how to implement TDD cycles
// for different components of the media-sync system.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// EXAMPLE 1: Core Interface Testing (Red-Green-Refactor)
// =============================================================================

// RED: Write failing test first
func TestInputService_FetchMedia_ReturnsMediaItems(t *testing.T) {
	// This test will fail initially because InputService doesn't exist
	mockService := &MockInputService{}
	mockService.On("FetchMedia", mock.Anything, mock.Anything).Return([]MediaItem{
		{
			ID:          "test-123",
			URL:         "https://example.com/image.jpg",
			ContentType: "image/jpeg",
		},
	}, nil)

	config := map[string]interface{}{
		"username": "testuser",
	}

	media, err := mockService.FetchMedia(context.Background(), config)

	require.NoError(t, err)
	assert.Len(t, media, 1)
	assert.Equal(t, "test-123", media[0].ID)
	mockService.AssertExpectations(t)
}

// GREEN: Minimal implementation to make test pass
type InputService interface {
	FetchMedia(ctx context.Context, config map[string]interface{}) ([]MediaItem, error)
	GetMetadata() PluginMetadata
}

type MediaItem struct {
	ID          string                 `json:"id"`
	URL         string                 `json:"url"`
	ContentType string                 `json:"content_type"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
}

type PluginMetadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// REFACTOR: Add validation and improve structure
type ValidatedMediaItem struct {
	MediaItem
	validated bool
}

func (m *ValidatedMediaItem) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("media item ID cannot be empty")
	}
	if m.URL == "" {
		return fmt.Errorf("media item URL cannot be empty")
	}
	m.validated = true
	return nil
}

// =============================================================================
// EXAMPLE 2: Configuration Hot Reload Testing
// =============================================================================

func TestConfigLoader_HotReload_DetectsFileChanges(t *testing.T) {
	// Create temporary config file
	tempFile := createTempConfigFile(t, initialConfig)
	defer os.Remove(tempFile)

	loader := NewConfigLoader()
	changeChan := make(chan ConfigChange, 1)

	// Start watching for changes
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go loader.WatchForChanges(ctx, tempFile, changeChan)

	// Wait a moment for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	err := os.WriteFile(tempFile, []byte(modifiedConfig), 0644)
	require.NoError(t, err)

	// Wait for change notification
	select {
	case change := <-changeChan:
		assert.Equal(t, ConfigModified, change.Type)
		assert.Equal(t, tempFile, change.FilePath)
		assert.NotEqual(t, initialConfig, change.NewConfig.Raw)
	case <-time.After(2 * time.Second):
		t.Fatal("Config change not detected within timeout")
	}
}

// Supporting types and functions for config testing
type ConfigLoader struct {
	watchers map[string]*fsWatcher
}

type ConfigChange struct {
	Type      ConfigChangeType
	FilePath  string
	NewConfig *Config
	Error     error
}

type ConfigChangeType int

const (
	ConfigModified ConfigChangeType = iota
	ConfigDeleted
	ConfigError
)

type Config struct {
	Services []ServiceConfig `yaml:"services"`
	Raw      []byte
}

type ServiceConfig struct {
	Name     string                 `yaml:"name"`
	Type     string                 `yaml:"type"`
	Plugin   string                 `yaml:"plugin"`
	Settings map[string]interface{} `yaml:"settings"`
}

const initialConfig = `
services:
  - name: tumblr-input
    type: input
    plugin: tumblr
    settings:
      username: testuser
`

const modifiedConfig = `
services:
  - name: tumblr-input
    type: input
    plugin: tumblr
    settings:
      username: testuser2
      api_key: new-key
`

// =============================================================================
// EXAMPLE 3: Database Layer Testing with Transactions
// =============================================================================

func TestSQLiteStore_CreateMedia_WithTransaction(t *testing.T) {
	db := setupTestDB(t)
	store := &SQLiteStore{db: db}

	media := MediaItem{
		ID:          "transaction-test-123",
		URL:         "https://example.com/test.jpg",
		ContentType: "image/jpeg",
		CreatedAt:   time.Now(),
	}

	// Test successful transaction
	ctx := context.Background()
	err := store.CreateMedia(ctx, media)
	require.NoError(t, err)

	// Verify media was stored
	retrieved, err := store.GetMedia(ctx, media.ID)
	require.NoError(t, err)
	assert.Equal(t, media.ID, retrieved.ID)
	assert.Equal(t, media.URL, retrieved.URL)

	// Test transaction rollback on error
	invalidMedia := MediaItem{
		ID:  "", // This should cause a constraint error
		URL: "https://example.com/invalid.jpg",
	}

	err = store.CreateMedia(ctx, invalidMedia)
	assert.Error(t, err, "Expected constraint violation")

	// Verify original media is still there (transaction didn't affect it)
	retrieved, err = store.GetMedia(ctx, media.ID)
	require.NoError(t, err)
	assert.Equal(t, media.ID, retrieved.ID)
}

// Database implementation with transaction support
type SQLiteStore struct {
	db *sql.DB
}

func (s *SQLiteStore) CreateMedia(ctx context.Context, media MediaItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Safe to call even if committed

	query := `
		INSERT INTO media (id, url, content_type, metadata, created_at)
		VALUES (?, ?, ?, ?, ?)
	`

	metadataJSON, err := json.Marshal(media.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = tx.ExecContext(ctx, query,
		media.ID, media.URL, media.ContentType, metadataJSON, media.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert media: %w", err)
	}

	return tx.Commit()
}

func (s *SQLiteStore) GetMedia(ctx context.Context, id string) (*MediaItem, error) {
	query := `
		SELECT id, url, content_type, metadata, created_at
		FROM media WHERE id = ?
	`

	var media MediaItem
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&media.ID, &media.URL, &media.ContentType, &metadataJSON, &media.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get media: %w", err)
	}

	if len(metadataJSON) > 0 {
		err = json.Unmarshal(metadataJSON, &media.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &media, nil
}

// =============================================================================
// EXAMPLE 4: Plugin System Testing with Hot Reload
// =============================================================================

func TestPluginManager_LoadAndReload_Plugin(t *testing.T) {
	// Setup test plugin directory
	pluginDir := t.TempDir()
	configFile := filepath.Join(pluginDir, "tumblr.yaml")

	// Create initial plugin configuration
	initialPluginConfig := `
name: tumblr
version: 1.0.0
type: input
description: Tumblr media fetcher
settings:
  api_url: https://api.tumblr.com/v2
`

	err := os.WriteFile(configFile, []byte(initialPluginConfig), 0644)
	require.NoError(t, err)

	manager := NewPluginManager()
	eventChan := make(chan PluginEvent, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start plugin manager
	go manager.WatchPlugins(ctx, pluginDir, eventChan)

	// Wait for initial load
	select {
	case event := <-eventChan:
		assert.Equal(t, PluginLoaded, event.Type)
		assert.Equal(t, "tumblr", event.Plugin.Name)
		assert.Equal(t, "1.0.0", event.Plugin.Version)
	case <-time.After(2 * time.Second):
		t.Fatal("Plugin load event not received")
	}

	// Modify plugin config (simulate hot reload)
	updatedPluginConfig := `
name: tumblr
version: 1.1.0
type: input
description: Tumblr media fetcher with improvements
settings:
  api_url: https://api.tumblr.com/v2
  timeout: 30s
`

	time.Sleep(100 * time.Millisecond) // Ensure different modification time
	err = os.WriteFile(configFile, []byte(updatedPluginConfig), 0644)
	require.NoError(t, err)

	// Wait for reload event
	select {
	case event := <-eventChan:
		assert.Equal(t, PluginReloaded, event.Type)
		assert.Equal(t, "tumblr", event.Plugin.Name)
		assert.Equal(t, "1.1.0", event.Plugin.Version)
		assert.Contains(t, event.Plugin.Description, "improvements")
	case <-time.After(2 * time.Second):
		t.Fatal("Plugin reload event not received")
	}

	// Verify manager has updated plugin
	plugin, exists := manager.GetPlugin("tumblr")
	assert.True(t, exists)
	assert.Equal(t, "1.1.0", plugin.Version)
}

// Plugin system types
type PluginManager struct {
	plugins  map[string]*Plugin
	watchers map[string]*pluginWatcher
}

type Plugin struct {
	Name        string
	Version     string
	Type        string
	Description string
	Settings    map[string]interface{}
	ConfigPath  string
	Service     interface{} // Will be InputService or OutputService
}

type PluginEvent struct {
	Type   PluginEventType
	Plugin *Plugin
	Error  error
}

type PluginEventType int

const (
	PluginLoaded PluginEventType = iota
	PluginReloaded
	PluginUnloaded
	PluginError
)

// =============================================================================
// EXAMPLE 5: External API Testing with Mock Server
// =============================================================================

func TestTumblrInputPlugin_FetchMedia_WithMockServer(t *testing.T) {
	// Setup mock Tumblr API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/blog/testuser.tumblr.com/posts":
			w.Header().Set("Content-Type", "application/json")
			response := TumblrAPIResponse{
				Response: TumblrResponse{
					Posts: []TumblrPost{
						{
							ID:   "12345",
							Type: "photo",
							Photos: []TumblrPhoto{
								{
									OriginalSize: TumblrPhotoSize{
										URL:    "https://example.com/image1.jpg",
										Width:  1920,
										Height: 1080,
									},
								},
							},
							Timestamp: time.Now().Unix(),
						},
						{
							ID:   "12346",
							Type: "video",
							VideoURL: "https://example.com/video1.mp4",
							Timestamp: time.Now().Unix(),
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)

		case "/oauth/access_token":
			w.Header().Set("Content-Type", "application/json")
			response := map[string]string{
				"access_token": "test-access-token",
				"token_type":   "bearer",
			}
			json.NewEncoder(w).Encode(response)

		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	// Create plugin with mock server URL
	plugin := &TumblrInputPlugin{
		apiBaseURL: mockServer.URL,
		client:     &http.Client{Timeout: 5 * time.Second},
	}

	config := map[string]interface{}{
		"username":     "testuser",
		"api_key":      "test-api-key",
		"access_token": "test-access-token",
	}

	// Test media fetching
	ctx := context.Background()
	media, err := plugin.FetchMedia(ctx, config)

	require.NoError(t, err)
	assert.Len(t, media, 2) // photo + video

	// Verify photo media item
	photoItem := media[0]
	assert.Equal(t, "12345", photoItem.ID)
	assert.Equal(t, "https://example.com/image1.jpg", photoItem.URL)
	assert.Equal(t, "image/jpeg", photoItem.ContentType)

	// Verify video media item
	videoItem := media[1]
	assert.Equal(t, "12346", videoItem.ID)
	assert.Equal(t, "https://example.com/video1.mp4", videoItem.URL)
	assert.Equal(t, "video/mp4", videoItem.ContentType)
}

// Tumblr API response structures
type TumblrAPIResponse struct {
	Response TumblrResponse `json:"response"`
}

type TumblrResponse struct {
	Posts []TumblrPost `json:"posts"`
}

type TumblrPost struct {
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Photos    []TumblrPhoto `json:"photos,omitempty"`
	VideoURL  string        `json:"video_url,omitempty"`
	Timestamp int64         `json:"timestamp"`
}

type TumblrPhoto struct {
	OriginalSize TumblrPhotoSize `json:"original_size"`
}

type TumblrPhotoSize struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Tumblr plugin implementation
type TumblrInputPlugin struct {
	apiBaseURL string
	client     *http.Client
}

func (p *TumblrInputPlugin) FetchMedia(ctx context.Context, config map[string]interface{}) ([]MediaItem, error) {
	username, ok := config["username"].(string)
	if !ok {
		return nil, fmt.Errorf("username is required")
	}

	apiKey, ok := config["api_key"].(string)
	if !ok {
		return nil, fmt.Errorf("api_key is required")
	}

	url := fmt.Sprintf("%s/v2/blog/%s.tumblr.com/posts", p.apiBaseURL, username)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("api_key", apiKey)
	req.URL.RawQuery = q.Encode()

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	var apiResp TumblrAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var mediaItems []MediaItem
	for _, post := range apiResp.Response.Posts {
		switch post.Type {
		case "photo":
			for _, photo := range post.Photos {
				mediaItems = append(mediaItems, MediaItem{
					ID:          post.ID,
					URL:         photo.OriginalSize.URL,
					ContentType: "image/jpeg",
					CreatedAt:   time.Unix(post.Timestamp, 0),
					Metadata: map[string]interface{}{
						"type":   "photo",
						"width":  photo.OriginalSize.Width,
						"height": photo.OriginalSize.Height,
					},
				})
			}
		case "video":
			if post.VideoURL != "" {
				mediaItems = append(mediaItems, MediaItem{
					ID:          post.ID,
					URL:         post.VideoURL,
					ContentType: "video/mp4",
					CreatedAt:   time.Unix(post.Timestamp, 0),
					Metadata: map[string]interface{}{
						"type": "video",
					},
				})
			}
		}
	}

	return mediaItems, nil
}

func (p *TumblrInputPlugin) GetMetadata() PluginMetadata {
	return PluginMetadata{
		Name:        "tumblr",
		Version:     "1.0.0",
		Type:        "input",
		Description: "Fetches media from Tumblr blogs",
	}
}

// =============================================================================
// HELPER FUNCTIONS AND MOCKS
// =============================================================================

// MockInputService for testing
type MockInputService struct {
	mock.Mock
}

func (m *MockInputService) FetchMedia(ctx context.Context, config map[string]interface{}) ([]MediaItem, error) {
	args := m.Called(ctx, config)
	return args.Get(0).([]MediaItem), args.Error(1)
}

func (m *MockInputService) GetMetadata() PluginMetadata {
	args := m.Called()
	return args.Get(0).(PluginMetadata)
}

// Test database setup
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create tables
	schema := `
	CREATE TABLE media (
		id TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		content_type TEXT NOT NULL,
		metadata TEXT,
		created_at DATETIME
	);
	`

	_, err = db.Exec(schema)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// Create temporary config file for testing
func createTempConfigFile(t *testing.T, content string) string {
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)

	_, err = tmpfile.Write([]byte(content))
	require.NoError(t, err)
	
	err = tmpfile.Close()
	require.NoError(t, err)

	return tmpfile.Name()
}

// Placeholder implementations for interfaces referenced in tests
func NewConfigLoader() *ConfigLoader           { return &ConfigLoader{} }
func NewPluginManager() *PluginManager         { return &PluginManager{} }
func (c *ConfigLoader) WatchForChanges(ctx context.Context, file string, ch chan<- ConfigChange) error { return nil }
func (p *PluginManager) WatchPlugins(ctx context.Context, dir string, ch chan<- PluginEvent) error { return nil }
func (p *PluginManager) GetPlugin(name string) (*Plugin, bool) { return nil, false }

type fsWatcher struct{}
type pluginWatcher struct{}