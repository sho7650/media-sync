package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteStorage_Initialize(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	storage := NewSQLiteStorage(dbPath)

	err := storage.Initialize(ctx)
	require.NoError(t, err, "Initialize should not return error")

	assert.True(t, storage.IsReady(), "Storage should be ready after initialization")

	// Verify database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "Database file should exist")

	// Cleanup
	err = storage.Close()
	require.NoError(t, err, "Close should not return error")
}

func TestSQLiteStorage_MediaOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("Store and retrieve media item", func(t *testing.T) {
		storage := setupTestDB(t)
		defer func() {
			if err := storage.Close(); err != nil {
				t.Logf("Failed to close storage: %v", err)
			}
		}()
		item := &MediaItem{
			ID:         "test-123",
			ServiceID:  "tumblr",
			ExternalID: "post-456",
			Type:       "photo",
			URL:        "https://example.com/image.jpg",
			LocalPath:  "/tmp/image.jpg",
			Metadata:   map[string]interface{}{"tags": []string{"test", "photo"}},
			Checksum:   "sha256:abc123def456",
			SizeBytes:  2048,
			CreatedAt:  time.Now().UTC(),
			SyncedAt:   time.Now().UTC(),
		}

		// Store item
		err := storage.StoreMedia(ctx, item)
		require.NoError(t, err, "StoreMedia should not return error")

		// Retrieve item
		retrieved, err := storage.GetMedia(ctx, "test-123")
		require.NoError(t, err, "GetMedia should not return error")
		require.NotNil(t, retrieved, "Retrieved item should not be nil")

		assert.Equal(t, item.ID, retrieved.ID)
		assert.Equal(t, item.ServiceID, retrieved.ServiceID)
		assert.Equal(t, item.URL, retrieved.URL)
		assert.Equal(t, item.Checksum, retrieved.Checksum)
	})

	t.Run("Duplicate detection by checksum", func(t *testing.T) {
		storage := setupTestDB(t)
		defer func() {
			if err := storage.Close(); err != nil {
				t.Logf("Failed to close storage: %v", err)
			}
		}()
		item := &MediaItem{
			ID:         "dup-test-1",
			ServiceID:  "tumblr",
			ExternalID: "post-789",
			Type:       "photo",
			Checksum:   "sha256:duplicate123",
			CreatedAt:  time.Now().UTC(),
		}

		// Store first item
		err := storage.StoreMedia(ctx, item)
		require.NoError(t, err)

		// Check for duplicate
		exists, err := storage.IsDuplicate(ctx, "tumblr", "sha256:duplicate123")
		require.NoError(t, err, "IsDuplicate should not return error")
		assert.True(t, exists, "Should detect duplicate by checksum")

		// Check for non-duplicate
		exists, err = storage.IsDuplicate(ctx, "tumblr", "sha256:different456")
		require.NoError(t, err)
		assert.False(t, exists, "Should not detect non-duplicate")

		// Check different service (same checksum)
		exists, err = storage.IsDuplicate(ctx, "instagram", "sha256:duplicate123")
		require.NoError(t, err)
		assert.False(t, exists, "Should not detect duplicate across different services")
	})

	t.Run("Query media items", func(t *testing.T) {
		storage := setupTestDB(t)
		defer func() {
			if err := storage.Close(); err != nil {
				t.Logf("Failed to close storage: %v", err)
			}
		}()
		// Setup test data
		items := []*MediaItem{
			{ID: "q1", ServiceID: "tumblr", ExternalID: "ext-q1", Type: "photo", CreatedAt: time.Now().UTC()},
			{ID: "q2", ServiceID: "tumblr", ExternalID: "ext-q2", Type: "video", CreatedAt: time.Now().UTC()},
			{ID: "q3", ServiceID: "instagram", ExternalID: "ext-q3", Type: "photo", CreatedAt: time.Now().UTC()},
			{ID: "q4", ServiceID: "tumblr", ExternalID: "ext-q4", Type: "photo", CreatedAt: time.Now().Add(-time.Hour).UTC()},
		}

		for _, item := range items {
			err := storage.StoreMedia(ctx, item)
			require.NoError(t, err)
		}

		// Query by service
		results, err := storage.QueryMedia(ctx, MediaQuery{ServiceID: "tumblr"})
		require.NoError(t, err)
		assert.Len(t, results, 3, "Should find 3 tumblr items")

		// Query by type
		results, err = storage.QueryMedia(ctx, MediaQuery{Type: "photo"})
		require.NoError(t, err)
		assert.Len(t, results, 3, "Should find 3 photo items")

		// Query with limit
		results, err = storage.QueryMedia(ctx, MediaQuery{ServiceID: "tumblr", Limit: 1})
		require.NoError(t, err)
		assert.Len(t, results, 1, "Should respect limit")

		// Query with time range
		now := time.Now().UTC()
		startTime := now.Add(-30 * time.Minute)
		results, err = storage.QueryMedia(ctx, MediaQuery{
			StartTime: &startTime,
			EndTime:   &now,
		})
		require.NoError(t, err)
		assert.Len(t, results, 3, "Should find items in time range")
	})
}

func TestSQLiteStorage_SyncState(t *testing.T) {
	ctx := context.Background()

	t.Run("Save and retrieve sync state", func(t *testing.T) {
		storage := setupTestDB(t)
		defer func() {
			if err := storage.Close(); err != nil {
				t.Logf("Failed to close storage: %v", err)
			}
		}()
		state := &SyncState{
			ServiceID:      "tumblr",
			LastSyncTime:   time.Now().UTC(),
			LastSyncCursor: "cursor-abc123",
			ItemsProcessed: 250,
			ItemsSuccess:   240,
			ItemsFailed:    10,
		}

		// Save state
		err := storage.SaveSyncState(ctx, state)
		require.NoError(t, err, "SaveSyncState should not return error")

		// Retrieve state
		retrieved, err := storage.GetSyncState(ctx, "tumblr")
		require.NoError(t, err, "GetSyncState should not return error")
		require.NotNil(t, retrieved, "Retrieved state should not be nil")

		assert.Equal(t, state.ServiceID, retrieved.ServiceID)
		assert.Equal(t, state.LastSyncCursor, retrieved.LastSyncCursor)
		assert.Equal(t, state.ItemsProcessed, retrieved.ItemsProcessed)
		assert.Equal(t, state.ItemsSuccess, retrieved.ItemsSuccess)
		assert.Equal(t, state.ItemsFailed, retrieved.ItemsFailed)
	})

	t.Run("Update existing sync state", func(t *testing.T) {
		storage := setupTestDB(t)
		defer func() {
			if err := storage.Close(); err != nil {
				t.Logf("Failed to close storage: %v", err)
			}
		}()
		// Initial state
		state1 := &SyncState{
			ServiceID:      "instagram",
			LastSyncTime:   time.Now().UTC(),
			LastSyncCursor: "cursor-initial",
			ItemsProcessed: 100,
			ItemsSuccess:   95,
			ItemsFailed:    5,
		}

		err := storage.SaveSyncState(ctx, state1)
		require.NoError(t, err)

		// Updated state
		state2 := &SyncState{
			ServiceID:      "instagram",
			LastSyncTime:   time.Now().UTC(),
			LastSyncCursor: "cursor-updated",
			ItemsProcessed: 200,
			ItemsSuccess:   190,
			ItemsFailed:    10,
		}

		err = storage.SaveSyncState(ctx, state2)
		require.NoError(t, err)

		// Verify update
		retrieved, err := storage.GetSyncState(ctx, "instagram")
		require.NoError(t, err)

		assert.Equal(t, state2.LastSyncCursor, retrieved.LastSyncCursor)
		assert.Equal(t, state2.ItemsProcessed, retrieved.ItemsProcessed)
	})
}

func TestSQLiteStorage_Concurrency(t *testing.T) {
	ctx := context.Background()

	t.Run("Concurrent media storage", func(t *testing.T) {
		storage := setupTestDB(t)
		defer func() {
			if err := storage.Close(); err != nil {
				t.Logf("Failed to close storage: %v", err)
			}
		}()
		const numGoroutines = 10
		const itemsPerGoroutine = 5

		done := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(workerID int) {
				defer func() { done <- nil }()

				for j := 0; j < itemsPerGoroutine; j++ {
					item := &MediaItem{
						ID:         fmt.Sprintf("worker-%d-item-%d", workerID, j),
						ServiceID:  "tumblr",
						ExternalID: fmt.Sprintf("ext-worker-%d-item-%d", workerID, j),
						Type:       "photo",
						Checksum:   fmt.Sprintf("checksum-%d-%d", workerID, j),
						CreatedAt:  time.Now().UTC(),
					}

					err := storage.StoreMedia(ctx, item)
					if err != nil {
						done <- err
						return
					}
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			err := <-done
			require.NoError(t, err, "Concurrent storage should not fail")
		}

		// Verify all items were stored
		results, err := storage.QueryMedia(ctx, MediaQuery{ServiceID: "tumblr"})
		require.NoError(t, err)
		assert.Len(t, results, numGoroutines*itemsPerGoroutine, "All items should be stored")
	})
}

func TestSQLiteStorage_Transactions(t *testing.T) {
	ctx := context.Background()

	t.Run("Transaction rollback on error", func(t *testing.T) {
		storage := setupTestDB(t)
		defer func() {
			if err := storage.Close(); err != nil {
				t.Logf("Failed to close storage: %v", err)
			}
		}()
		// This test verifies that failed operations don't corrupt the database
		item1 := &MediaItem{
			ID:         "tx-test-1",
			ServiceID:  "tumblr",
			ExternalID: "ext-tx-test-1",
			Type:       "photo",
			CreatedAt:  time.Now().UTC(),
		}

		err := storage.StoreMedia(ctx, item1)
		require.NoError(t, err)

		// Try to store item with same ID (should fail)
		item2 := &MediaItem{
			ID:         "tx-test-1", // Duplicate ID
			ServiceID:  "instagram",
			ExternalID: "ext-tx-test-2",
			Type:       "video",
			CreatedAt:  time.Now().UTC(),
		}

		err = storage.StoreMedia(ctx, item2)
		assert.Error(t, err, "Should fail on duplicate ID")

		// Verify original item is still there and unchanged
		retrieved, err := storage.GetMedia(ctx, "tx-test-1")
		require.NoError(t, err)
		assert.Equal(t, "tumblr", retrieved.ServiceID, "Original item should be unchanged")
	})
}

// Helper function to setup test database
func setupTestDB(t *testing.T) StorageManager {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	storage := NewSQLiteStorage(dbPath)
	err := storage.Initialize(context.Background())
	require.NoError(t, err, "Test database initialization should succeed")

	return storage
}
