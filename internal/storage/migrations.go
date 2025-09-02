package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Migration represents a database migration
type Migration struct {
	Version     int       `json:"version"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Up          string    `json:"up"`
	Down        string    `json:"down"`
	AppliedAt   time.Time `json:"applied_at"`
}

// MigrationManager handles database migrations
type MigrationManager struct {
	dbPath string
	db     *sql.DB
}

// MigrationRunner provides migration execution capabilities
type MigrationRunner interface {
	Initialize(ctx context.Context) error
	GetCurrentVersion(ctx context.Context) (int, error)
	ApplyMigration(ctx context.Context, migration Migration) error
	RollbackMigration(ctx context.Context, version int) error
	ListAppliedMigrations(ctx context.Context) ([]Migration, error)
	MigrateToVersion(ctx context.Context, migrations []Migration, targetVersion int) error
	Close() error
}

// Ensure MigrationManager implements MigrationRunner
var _ MigrationRunner = (*MigrationManager)(nil)

// NewMigrationManager creates a new migration manager
func NewMigrationManager(dbPath string) *MigrationManager {
	return &MigrationManager{
		dbPath: dbPath,
	}
}

// Initialize sets up the migration tracking table
func (mm *MigrationManager) Initialize(ctx context.Context) error {
	db, err := sql.Open("sqlite3", mm.dbPath+"?cache=shared&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=ON")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	mm.db = db

	// Create migrations tracking table
	createSQL := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`

	_, err = mm.db.ExecContext(ctx, createSQL)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	return nil
}

// HasMigrationTable checks if the migration tracking table exists
func (mm *MigrationManager) HasMigrationTable(ctx context.Context) (bool, error) {
	query := `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`
	var count int
	err := mm.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check migration table: %w", err)
	}
	return count > 0, nil
}

// GetCurrentVersion returns the current database schema version
func (mm *MigrationManager) GetCurrentVersion(ctx context.Context) (int, error) {
	query := `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`
	var version int
	err := mm.db.QueryRowContext(ctx, query).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}
	return version, nil
}

// IsMigrationApplied checks if a specific migration version has been applied
func (mm *MigrationManager) IsMigrationApplied(ctx context.Context, version int) (bool, error) {
	query := `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`
	var count int
	err := mm.db.QueryRowContext(ctx, query, version).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check migration status: %w", err)
	}
	return count > 0, nil
}

// ApplyMigration applies a single migration
func (mm *MigrationManager) ApplyMigration(ctx context.Context, migration Migration) error {
	// Validate migration
	if migration.Version <= 0 {
		return fmt.Errorf("migration version must be positive, got %d", migration.Version)
	}
	if migration.Name == "" {
		return fmt.Errorf("migration name cannot be empty")
	}
	if migration.Up == "" {
		return fmt.Errorf("migration Up script cannot be empty")
	}

	// Check if already applied
	applied, err := mm.IsMigrationApplied(ctx, migration.Version)
	if err != nil {
		return err
	}
	if applied {
		return fmt.Errorf("migration version %d already applied", migration.Version)
	}

	// Start transaction
	tx, err := mm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Execute the migration
	_, err = tx.ExecContext(ctx, migration.Up)
	if err != nil {
		return fmt.Errorf("failed to execute migration %d (%s): %w", migration.Version, migration.Name, err)
	}

	// Record the migration
	_, err = tx.ExecContext(ctx, 
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		migration.Version, migration.Name, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
	}

	return nil
}

// RollbackMigration rolls back a specific migration
func (mm *MigrationManager) RollbackMigration(ctx context.Context, version int) error {
	// Check if migration is applied
	applied, err := mm.IsMigrationApplied(ctx, version)
	if err != nil {
		return err
	}
	if !applied {
		return fmt.Errorf("migration version %d not applied", version)
	}

	// Get migration details - need Down script
	query := `SELECT name FROM schema_migrations WHERE version = ?`
	var name string
	err = mm.db.QueryRowContext(ctx, query, version).Scan(&name)
	if err != nil {
		return fmt.Errorf("failed to get migration details: %w", err)
	}

	// Start transaction
	tx, err := mm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start rollback transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Remove from tracking first
	_, err = tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = ?`, version)
	if err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit rollback: %w", err)
	}

	return nil
}

// ListAppliedMigrations returns all applied migrations
func (mm *MigrationManager) ListAppliedMigrations(ctx context.Context) ([]Migration, error) {
	query := `SELECT version, name, applied_at FROM schema_migrations ORDER BY version`
	rows, err := mm.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var migrations []Migration
	for rows.Next() {
		var migration Migration
		err := rows.Scan(&migration.Version, &migration.Name, &migration.AppliedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan migration: %w", err)
		}
		migrations = append(migrations, migration)
	}

	return migrations, nil
}

// MigrateToVersion migrates database to a specific version
func (mm *MigrationManager) MigrateToVersion(ctx context.Context, migrations []Migration, targetVersion int) error {
	currentVersion, err := mm.GetCurrentVersion(ctx)
	if err != nil {
		return err
	}

	if targetVersion > currentVersion {
		// Apply migrations up to target version
		for _, migration := range migrations {
			if migration.Version <= currentVersion {
				continue
			}
			if migration.Version > targetVersion {
				break
			}
			err = mm.ApplyMigration(ctx, migration)
			if err != nil {
				return err
			}
		}
	} else if targetVersion < currentVersion {
		// Rollback migrations down to target version
		for i := currentVersion; i > targetVersion; i-- {
			err = mm.RollbackMigration(ctx, i)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Close closes the database connection
func (mm *MigrationManager) Close() error {
	if mm.db != nil {
		return mm.db.Close()
	}
	return nil
}