package storage

import (
	"context"
	"time"
)

// StorageManager defines the contract for data persistence
type StorageManager interface {
	// Lifecycle
	Initialize(ctx context.Context) error
	Close() error
	IsReady() bool

	// Media operations
	StoreMedia(ctx context.Context, item *MediaItem) error
	GetMedia(ctx context.Context, id string) (*MediaItem, error)
	QueryMedia(ctx context.Context, query MediaQuery) ([]*MediaItem, error)
	IsDuplicate(ctx context.Context, serviceID, checksum string) (bool, error)

	// Sync state operations
	SaveSyncState(ctx context.Context, state *SyncState) error
	GetSyncState(ctx context.Context, serviceID string) (*SyncState, error)
}

// MediaItem represents a media item stored in the system
type MediaItem struct {
	ID         string                 `json:"id" db:"id"`
	ServiceID  string                 `json:"service_id" db:"service_id"`
	ExternalID string                 `json:"external_id" db:"external_id"`
	Type       string                 `json:"type" db:"type"`
	URL        string                 `json:"url" db:"url"`
	LocalPath  string                 `json:"local_path,omitempty" db:"local_path"`
	Metadata   map[string]interface{} `json:"metadata" db:"metadata"`
	Checksum   string                 `json:"checksum" db:"checksum"`
	SizeBytes  int64                  `json:"size_bytes" db:"size_bytes"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`
	SyncedAt   time.Time              `json:"synced_at" db:"synced_at"`
}

// SyncState tracks synchronization progress for each service
type SyncState struct {
	ServiceID      string    `json:"service_id" db:"service_id"`
	LastSyncTime   time.Time `json:"last_sync_time" db:"last_sync_time"`
	LastSyncCursor string    `json:"last_sync_cursor" db:"last_sync_cursor"`
	ItemsProcessed int       `json:"items_processed" db:"items_processed"`
	ItemsSuccess   int       `json:"items_success" db:"items_success"`
	ItemsFailed    int       `json:"items_failed" db:"items_failed"`
}

// MediaQuery defines search parameters for media items
type MediaQuery struct {
	ServiceID string
	Type      string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
}
