package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements StorageManager using SQLite
type SQLiteStorage struct {
	dbPath string
	db     *sql.DB
	ready  bool
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string) *SQLiteStorage {
	return &SQLiteStorage{
		dbPath: dbPath,
	}
}

// Initialize sets up the SQLite database and creates tables
func (s *SQLiteStorage) Initialize(ctx context.Context) error {
	db, err := sql.Open("sqlite3", s.dbPath+"?cache=shared&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=ON")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return fmt.Errorf("failed to ping database: %w (close error: %v)", err, closeErr)
		}
		return fmt.Errorf("failed to ping database: %w", err)
	}

	s.db = db

	// Create tables
	if err := s.createTables(ctx); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	s.ready = true
	return nil
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		s.ready = false
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
}

// IsReady returns whether the storage is ready for operations
func (s *SQLiteStorage) IsReady() bool {
	return s.ready && s.db != nil
}

// StoreMedia stores a media item in the database with transaction support
func (s *SQLiteStorage) StoreMedia(ctx context.Context, item *MediaItem) error {
	if !s.IsReady() {
		return fmt.Errorf("storage not ready")
	}

	// Validate required fields
	if item.ID == "" {
		return fmt.Errorf("media item ID cannot be empty")
	}
	if item.ServiceID == "" {
		return fmt.Errorf("media item ServiceID cannot be empty")
	}
	if item.ExternalID == "" {
		return fmt.Errorf("media item ExternalID cannot be empty")
	}

	metadataJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Use transaction for consistency
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			// Log rollback error but preserve original error
			_ = rollbackErr
		}
	}()

	query := `
		INSERT INTO media_items (
			id, service_id, external_id, type, url, local_path,
			metadata, checksum, size_bytes, created_at, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.ExecContext(ctx, query,
		item.ID, item.ServiceID, item.ExternalID, item.Type, item.URL,
		item.LocalPath, string(metadataJSON), item.Checksum, item.SizeBytes,
		item.CreatedAt, item.SyncedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to store media item: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetMedia retrieves a media item by ID
func (s *SQLiteStorage) GetMedia(ctx context.Context, id string) (*MediaItem, error) {
	if !s.IsReady() {
		return nil, fmt.Errorf("storage not ready")
	}

	if id == "" {
		return nil, fmt.Errorf("media item ID cannot be empty")
	}

	query := `
		SELECT id, service_id, external_id, type, url, local_path,
		       metadata, checksum, size_bytes, created_at, synced_at
		FROM media_items WHERE id = ?`

	row := s.db.QueryRowContext(ctx, query, id)

	item := &MediaItem{}
	var metadataJSON string

	err := row.Scan(
		&item.ID, &item.ServiceID, &item.ExternalID, &item.Type,
		&item.URL, &item.LocalPath, &metadataJSON, &item.Checksum,
		&item.SizeBytes, &item.CreatedAt, &item.SyncedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get media item: %w", err)
	}

	if metadataJSON == "" {
		item.Metadata = make(map[string]interface{})
	} else {
		if err := json.Unmarshal([]byte(metadataJSON), &item.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return item, nil
}

// IsDuplicate checks if a media item with the given checksum exists for the service
func (s *SQLiteStorage) IsDuplicate(ctx context.Context, serviceID, checksum string) (bool, error) {
	if !s.IsReady() {
		return false, fmt.Errorf("storage not ready")
	}

	query := `SELECT COUNT(*) FROM media_items WHERE service_id = ? AND checksum = ?`

	var count int
	err := s.db.QueryRowContext(ctx, query, serviceID, checksum).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check duplicate: %w", err)
	}

	return count > 0, nil
}

// QueryMedia searches for media items based on the given criteria
func (s *SQLiteStorage) QueryMedia(ctx context.Context, query MediaQuery) ([]*MediaItem, error) {
	if !s.IsReady() {
		return nil, fmt.Errorf("storage not ready")
	}

	var conditions []string
	var args []interface{}

	if query.ServiceID != "" {
		conditions = append(conditions, "service_id = ?")
		args = append(args, query.ServiceID)
	}

	if query.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, query.Type)
	}

	if query.StartTime != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, *query.StartTime)
	}

	if query.EndTime != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, *query.EndTime)
	}

	sqlQuery := "SELECT id, service_id, external_id, type, url, local_path, metadata, checksum, size_bytes, created_at, synced_at FROM media_items"

	if len(conditions) > 0 {
		sqlQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	sqlQuery += " ORDER BY created_at DESC"

	if query.Limit > 0 {
		sqlQuery += " LIMIT ?"
		args = append(args, query.Limit)
	}

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query media items: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			// Log close error but preserve original error
			_ = closeErr
		}
	}()

	var results []*MediaItem
	for rows.Next() {
		item := &MediaItem{}
		var metadataJSON string

		err := rows.Scan(
			&item.ID, &item.ServiceID, &item.ExternalID, &item.Type,
			&item.URL, &item.LocalPath, &metadataJSON, &item.Checksum,
			&item.SizeBytes, &item.CreatedAt, &item.SyncedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan media item: %w", err)
		}

		if err := json.Unmarshal([]byte(metadataJSON), &item.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return results, nil
}

// SaveSyncState saves synchronization state for a service
func (s *SQLiteStorage) SaveSyncState(ctx context.Context, state *SyncState) error {
	if !s.IsReady() {
		return fmt.Errorf("storage not ready")
	}

	query := `
		INSERT OR REPLACE INTO sync_states (
			service_id, last_sync_time, last_sync_cursor,
			items_processed, items_success, items_failed
		) VALUES (?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		state.ServiceID, state.LastSyncTime, state.LastSyncCursor,
		state.ItemsProcessed, state.ItemsSuccess, state.ItemsFailed,
	)

	if err != nil {
		return fmt.Errorf("failed to save sync state: %w", err)
	}

	return nil
}

// GetSyncState retrieves synchronization state for a service
func (s *SQLiteStorage) GetSyncState(ctx context.Context, serviceID string) (*SyncState, error) {
	if !s.IsReady() {
		return nil, fmt.Errorf("storage not ready")
	}

	query := `
		SELECT service_id, last_sync_time, last_sync_cursor,
		       items_processed, items_success, items_failed
		FROM sync_states WHERE service_id = ?`

	row := s.db.QueryRowContext(ctx, query, serviceID)

	state := &SyncState{}
	err := row.Scan(
		&state.ServiceID, &state.LastSyncTime, &state.LastSyncCursor,
		&state.ItemsProcessed, &state.ItemsSuccess, &state.ItemsFailed,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get sync state: %w", err)
	}

	return state, nil
}

// createTables creates the necessary database tables
func (s *SQLiteStorage) createTables(ctx context.Context) error {
	mediaItemsTable := `
		CREATE TABLE IF NOT EXISTS media_items (
			id TEXT PRIMARY KEY,
			service_id TEXT NOT NULL,
			external_id TEXT NOT NULL,
			type TEXT NOT NULL,
			url TEXT NOT NULL,
			local_path TEXT,
			metadata TEXT NOT NULL,
			checksum TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			synced_at DATETIME NOT NULL,
			UNIQUE(service_id, external_id)
		)`

	syncStatesTable := `
		CREATE TABLE IF NOT EXISTS sync_states (
			service_id TEXT PRIMARY KEY,
			last_sync_time DATETIME NOT NULL,
			last_sync_cursor TEXT NOT NULL,
			items_processed INTEGER NOT NULL DEFAULT 0,
			items_success INTEGER NOT NULL DEFAULT 0,
			items_failed INTEGER NOT NULL DEFAULT 0
		)`

	if _, err := s.db.ExecContext(ctx, mediaItemsTable); err != nil {
		return fmt.Errorf("failed to create media_items table: %w", err)
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_service_checksum ON media_items(service_id, checksum)",
		"CREATE INDEX IF NOT EXISTS idx_service_type ON media_items(service_id, type)",
		"CREATE INDEX IF NOT EXISTS idx_created_at ON media_items(created_at)",
	}

	for _, index := range indexes {
		if _, err := s.db.ExecContext(ctx, index); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	if _, err := s.db.ExecContext(ctx, syncStatesTable); err != nil {
		return fmt.Errorf("failed to create sync_states table: %w", err)
	}

	return nil
}
