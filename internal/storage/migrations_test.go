package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationManager_Initialize(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_migrations.db")

	t.Run("Initialize migration tracking table", func(t *testing.T) {
		// This test will initially fail (RED) - no MigrationManager exists yet
		migrationManager := NewMigrationManager(dbPath)
		
		err := migrationManager.Initialize(ctx)
		require.NoError(t, err, "Initialize should not return error")
		
		// Verify migration tracking table exists
		exists, err := migrationManager.HasMigrationTable(ctx)
		require.NoError(t, err)
		assert.True(t, exists, "Migration tracking table should exist after initialize")
	})
}

func TestMigrationManager_GetCurrentVersion(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_version.db")

	t.Run("Get version from empty database", func(t *testing.T) {
		migrationManager := NewMigrationManager(dbPath)
		err := migrationManager.Initialize(ctx)
		require.NoError(t, err)
		
		version, err := migrationManager.GetCurrentVersion(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, version, "Empty database should have version 0")
	})
	
	t.Run("Get version after migration", func(t *testing.T) {
		migrationManager := NewMigrationManager(dbPath)
		err := migrationManager.Initialize(ctx)
		require.NoError(t, err)
		
		// Apply migration
		migration := Migration{
			Version: 1,
			Name:    "create_test_table",
			Up:      "CREATE TABLE test_table (id INTEGER PRIMARY KEY)",
			Down:    "DROP TABLE test_table",
		}
		
		err = migrationManager.ApplyMigration(ctx, migration)
		require.NoError(t, err)
		
		version, err := migrationManager.GetCurrentVersion(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, version, "Version should be 1 after applying migration")
	})
}

func TestMigrationManager_ApplyMigration(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_apply.db")

	t.Run("Apply single migration", func(t *testing.T) {
		migrationManager := NewMigrationManager(dbPath)
		err := migrationManager.Initialize(ctx)
		require.NoError(t, err)
		
		migration := Migration{
			Version: 1,
			Name:    "add_column",
			Up:      "CREATE TABLE test_migration (id INTEGER PRIMARY KEY, name TEXT)",
			Down:    "DROP TABLE test_migration",
		}
		
		err = migrationManager.ApplyMigration(ctx, migration)
		require.NoError(t, err, "ApplyMigration should not return error")
		
		// Verify migration was recorded
		applied, err := migrationManager.IsMigrationApplied(ctx, 1)
		require.NoError(t, err)
		assert.True(t, applied, "Migration should be marked as applied")
	})
	
	t.Run("Apply migration twice should fail", func(t *testing.T) {
		migrationManager := NewMigrationManager(dbPath)
		err := migrationManager.Initialize(ctx)
		require.NoError(t, err)
		
		migration := Migration{
			Version: 2,
			Name:    "duplicate_test",
			Up:      "CREATE TABLE duplicate_test (id INTEGER)",
			Down:    "DROP TABLE duplicate_test",
		}
		
		// First application should succeed
		err = migrationManager.ApplyMigration(ctx, migration)
		require.NoError(t, err)
		
		// Second application should fail
		err = migrationManager.ApplyMigration(ctx, migration)
		assert.Error(t, err, "Applying same migration twice should fail")
		assert.Contains(t, err.Error(), "already applied", "Error should mention migration already applied")
	})
}

func TestMigrationManager_RollbackMigration(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_rollback.db")

	t.Run("Rollback applied migration", func(t *testing.T) {
		migrationManager := NewMigrationManager(dbPath)
		err := migrationManager.Initialize(ctx)
		require.NoError(t, err)
		
		// Apply migration first
		migration := Migration{
			Version: 1,
			Name:    "rollback_test",
			Up:      "CREATE TABLE rollback_test (id INTEGER PRIMARY KEY)",
			Down:    "DROP TABLE rollback_test",
		}
		
		err = migrationManager.ApplyMigration(ctx, migration)
		require.NoError(t, err)
		
		// Verify it's applied
		applied, err := migrationManager.IsMigrationApplied(ctx, 1)
		require.NoError(t, err)
		assert.True(t, applied)
		
		// Rollback
		err = migrationManager.RollbackMigration(ctx, 1)
		require.NoError(t, err, "RollbackMigration should not return error")
		
		// Verify it's no longer applied
		applied, err = migrationManager.IsMigrationApplied(ctx, 1)
		require.NoError(t, err)
		assert.False(t, applied, "Migration should not be applied after rollback")
	})
	
	t.Run("Rollback non-applied migration should fail", func(t *testing.T) {
		migrationManager := NewMigrationManager(dbPath)
		err := migrationManager.Initialize(ctx)
		require.NoError(t, err)
		
		err = migrationManager.RollbackMigration(ctx, 999)
		assert.Error(t, err, "Rolling back non-applied migration should fail")
		assert.Contains(t, err.Error(), "not applied", "Error should mention migration not applied")
	})
}

func TestMigrationManager_ListMigrations(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_list.db")

	t.Run("List applied migrations", func(t *testing.T) {
		migrationManager := NewMigrationManager(dbPath)
		err := migrationManager.Initialize(ctx)
		require.NoError(t, err)
		
		// Apply multiple migrations
		migrations := []Migration{
			{
				Version: 1,
				Name:    "first_migration",
				Up:      "CREATE TABLE test1 (id INTEGER)",
				Down:    "DROP TABLE test1",
			},
			{
				Version: 2,
				Name:    "second_migration", 
				Up:      "CREATE TABLE test2 (id INTEGER)",
				Down:    "DROP TABLE test2",
			},
		}
		
		for _, migration := range migrations {
			err = migrationManager.ApplyMigration(ctx, migration)
			require.NoError(t, err)
		}
		
		// List migrations
		applied, err := migrationManager.ListAppliedMigrations(ctx)
		require.NoError(t, err)
		assert.Len(t, applied, 2, "Should list 2 applied migrations")
		
		// Verify migration details
		assert.Equal(t, "first_migration", applied[0].Name)
		assert.Equal(t, "second_migration", applied[1].Name)
		assert.Equal(t, 1, applied[0].Version)
		assert.Equal(t, 2, applied[1].Version)
	})
}

func TestMigrationManager_MigrateToVersion(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_migrate.db")

	t.Run("Migrate to specific version", func(t *testing.T) {
		migrationManager := NewMigrationManager(dbPath)
		err := migrationManager.Initialize(ctx)
		require.NoError(t, err)
		
		// Define migrations
		migrations := []Migration{
			{Version: 1, Name: "init", Up: "CREATE TABLE v1 (id INTEGER)", Down: "DROP TABLE v1"},
			{Version: 2, Name: "add_col", Up: "ALTER TABLE v1 ADD COLUMN name TEXT", Down: "ALTER TABLE v1 DROP COLUMN name"},
			{Version: 3, Name: "add_index", Up: "CREATE INDEX idx_v1_name ON v1(name)", Down: "DROP INDEX idx_v1_name"},
		}
		
		// Migrate to version 2
		err = migrationManager.MigrateToVersion(ctx, migrations, 2)
		require.NoError(t, err, "MigrateToVersion should not return error")
		
		// Verify current version is 2
		version, err := migrationManager.GetCurrentVersion(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, version, "Current version should be 2")
		
		// Verify migrations 1 and 2 are applied, but not 3
		applied1, err := migrationManager.IsMigrationApplied(ctx, 1)
		require.NoError(t, err)
		assert.True(t, applied1, "Migration 1 should be applied")
		
		applied2, err := migrationManager.IsMigrationApplied(ctx, 2)
		require.NoError(t, err)
		assert.True(t, applied2, "Migration 2 should be applied")
		
		applied3, err := migrationManager.IsMigrationApplied(ctx, 3)
		require.NoError(t, err)
		assert.False(t, applied3, "Migration 3 should not be applied")
	})
	
	t.Run("Rollback to lower version", func(t *testing.T) {
		migrationManager := NewMigrationManager(dbPath)
		err := migrationManager.Initialize(ctx)
		require.NoError(t, err)
		
		// Start with version 3 applied
		migrations := []Migration{
			{Version: 1, Name: "init", Up: "CREATE TABLE v1 (id INTEGER)", Down: "DROP TABLE v1"},
			{Version: 2, Name: "add_col", Up: "ALTER TABLE v1 ADD COLUMN name TEXT", Down: "ALTER TABLE v1 DROP COLUMN name"},
			{Version: 3, Name: "add_index", Up: "CREATE INDEX idx_v1_name ON v1(name)", Down: "DROP INDEX idx_v1_name"},
		}
		
		// Apply all migrations
		err = migrationManager.MigrateToVersion(ctx, migrations, 3)
		require.NoError(t, err)
		
		// Rollback to version 1
		err = migrationManager.MigrateToVersion(ctx, migrations, 1)
		require.NoError(t, err, "Rollback should not return error")
		
		// Verify current version is 1
		version, err := migrationManager.GetCurrentVersion(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, version, "Current version should be 1 after rollback")
	})
}