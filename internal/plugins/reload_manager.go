package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sho7650/media-sync/pkg/core/interfaces"
)

// ReloadPhase represents the phase of a reload operation
type ReloadPhase string

const (
	PhaseValidation  ReloadPhase = "validation"
	PhaseSnapshot    ReloadPhase = "snapshot"
	PhaseStopping    ReloadPhase = "stopping"
	PhaseLoading     ReloadPhase = "loading"
	PhaseStarting    ReloadPhase = "starting"
	PhaseHealthCheck ReloadPhase = "health_check"
	PhaseComplete    ReloadPhase = "complete"
	PhaseRollback    ReloadPhase = "rollback"
	PhaseFailed      ReloadPhase = "failed"
)

// ConfigValidator validates plugin configurations
type ConfigValidator interface {
	ValidateConfig(config *PluginConfig) error
	ValidatePluginTransition(oldConfig, newConfig *PluginConfig) error
	ValidateSystemConstraints(config *PluginConfig) error
}

// ConfigValidatorImpl provides basic configuration validation
type ConfigValidatorImpl struct{}

func (cv *ConfigValidatorImpl) ValidateConfig(config *PluginConfig) error {
	if config.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	
	if config.Type != "input" && config.Type != "output" {
		return fmt.Errorf("plugin type must be 'input' or 'output'")
	}
	
	// Validate settings based on plugin type
	if timeout, exists := config.Settings["timeout"]; exists {
		if timeoutInt, ok := timeout.(int); ok && timeoutInt < 0 {
			return fmt.Errorf("timeout cannot be negative")
		}
	}
	
	return nil
}

func (cv *ConfigValidatorImpl) ValidatePluginTransition(oldConfig, newConfig *PluginConfig) error {
	if oldConfig.Name != newConfig.Name {
		return fmt.Errorf("plugin name cannot be changed during reload")
	}
	
	if oldConfig.Type != newConfig.Type {
		return fmt.Errorf("plugin type cannot be changed during reload")
	}
	
	return nil
}

func (cv *ConfigValidatorImpl) ValidateSystemConstraints(config *PluginConfig) error {
	// Add system-level validation (resource limits, dependencies, etc.)
	return nil
}

// ConfigSnapshot captures plugin state for rollback operations
type ConfigSnapshot struct {
	PluginName    string         `json:"plugin_name"`
	Config        *PluginConfig  `json:"config"`
	State         PluginState    `json:"state"`
	Timestamp     time.Time      `json:"timestamp"`
	ResourceUsage *ResourceUsage `json:"resource_usage,omitempty"`
	HealthStatus  string         `json:"health_status"`
}

// ReloadTransaction manages atomic plugin reload operations
type ReloadTransaction struct {
	ID          string         `json:"id"`
	PluginName  string         `json:"plugin_name"`
	OldSnapshot *ConfigSnapshot `json:"old_snapshot"`
	NewConfig   *PluginConfig   `json:"new_config"`
	Phase       ReloadPhase     `json:"phase"`
	StartTime   time.Time       `json:"start_time"`
	ErrorCount  int             `json:"error_count"`
	LastError   error           `json:"last_error,omitempty"`
}

// ReloadManager manages atomic plugin reload operations with rollback
type ReloadManager struct {
	pluginManager   *PluginManager
	validator       ConfigValidator
	versionManager  ConfigVersionManager
	snapshots       map[string]*ConfigSnapshot
	transactions    map[string]*ReloadTransaction
	activeReloads   map[string]string // plugin -> transaction ID
	healthTimeout   time.Duration
	phaseCallback   func(ReloadPhase)
	mu              sync.RWMutex
}

// NewReloadManager creates a new reload manager
func NewReloadManager(pm *PluginManager, validator ConfigValidator) *ReloadManager {
	return &ReloadManager{
		pluginManager: pm,
		validator:     validator,
		snapshots:     make(map[string]*ConfigSnapshot),
		transactions:  make(map[string]*ReloadTransaction),
		activeReloads: make(map[string]string),
		healthTimeout: 10 * time.Second,
	}
}

// NewReloadManagerWithVersion creates a reload manager with version management
func NewReloadManagerWithVersion(pm *PluginManager, validator ConfigValidator, versionMgr ConfigVersionManager) *ReloadManager {
	rm := NewReloadManager(pm, validator)
	rm.versionManager = versionMgr
	return rm
}

// SetHealthTimeout sets the timeout for health checks
func (rm *ReloadManager) SetHealthTimeout(timeout time.Duration) {
	rm.healthTimeout = timeout
}

// SetPhaseCallback sets a callback for phase transitions
func (rm *ReloadManager) SetPhaseCallback(callback func(ReloadPhase)) {
	rm.phaseCallback = callback
}

// AtomicReload performs configuration reload with validation and rollback
func (rm *ReloadManager) AtomicReload(ctx context.Context, pluginName string, newConfig *PluginConfig) error {
	// Check for concurrent reload
	rm.mu.Lock()
	if existingTxnID, exists := rm.activeReloads[pluginName]; exists {
		rm.mu.Unlock()
		return fmt.Errorf("reload in progress for plugin %s (transaction %s)", 
			pluginName, existingTxnID)
	}
	
	txn := &ReloadTransaction{
		ID:         generateTxnID(),
		PluginName: pluginName,
		NewConfig:  newConfig,
		Phase:      PhaseValidation,
		StartTime:  time.Now(),
	}
	
	rm.transactions[txn.ID] = txn
	rm.activeReloads[pluginName] = txn.ID
	rm.mu.Unlock()
	
	defer rm.cleanupTransaction(txn.ID)
	
	// Notify phase changes
	rm.notifyPhase(PhaseValidation)
	
	// Phase 1: Validation
	if err := rm.validateReload(txn); err != nil {
		txn.Phase = PhaseFailed
		return fmt.Errorf("validation failed: %w", err)
	}
	
	// Phase 2: Create snapshot
	rm.notifyPhase(PhaseSnapshot)
	if err := rm.createSnapshot(txn); err != nil {
		txn.Phase = PhaseFailed
		return fmt.Errorf("snapshot creation failed: %w", err)
	}
	
	// Backup current config if version manager is available
	if rm.versionManager != nil && txn.OldSnapshot != nil && txn.OldSnapshot.Config != nil {
		if err := rm.versionManager.SaveVersion(pluginName, txn.OldSnapshot.Config, "pre_reload_backup"); err != nil {
			log.Printf("Failed to backup current config: %v", err)
		}
	}
	
	// Phase 3-6: Atomic reload with rollback on failure
	if err := rm.executeReload(ctx, txn); err != nil {
		// Rollback on any failure
		rm.notifyPhase(PhaseRollback)
		if rollbackErr := rm.rollback(ctx, txn); rollbackErr != nil {
			return fmt.Errorf("reload failed: %w, rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("reload failed and rolled back: %w", err)
	}
	
	// Success: save new version if version manager is available
	if rm.versionManager != nil {
		if err := rm.versionManager.SaveVersion(pluginName, newConfig, "hot_reload_success"); err != nil {
			log.Printf("Failed to save new version: %v", err)
		}
	}
	
	rm.notifyPhase(PhaseComplete)
	txn.Phase = PhaseComplete
	return nil
}

// validateReload performs comprehensive validation before reload
func (rm *ReloadManager) validateReload(txn *ReloadTransaction) error {
	// Basic config validation
	if err := rm.validator.ValidateConfig(txn.NewConfig); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}
	
	// Get current config for transition validation
	rm.mu.RLock()
	snapshot, exists := rm.snapshots[txn.PluginName]
	rm.mu.RUnlock()
	
	if exists && snapshot.Config != nil {
		if err := rm.validator.ValidatePluginTransition(snapshot.Config, txn.NewConfig); err != nil {
			return fmt.Errorf("transition validation: %w", err)
		}
	}
	
	// System constraints validation
	if err := rm.validator.ValidateSystemConstraints(txn.NewConfig); err != nil {
		return fmt.Errorf("system constraints: %w", err)
	}
	
	txn.Phase = PhaseSnapshot
	return nil
}

// createSnapshot captures current plugin state for rollback
func (rm *ReloadManager) createSnapshot(txn *ReloadTransaction) error {
	status, exists := rm.pluginManager.GetPluginStatus(txn.PluginName)
	if !exists {
		// Plugin doesn't exist yet, create minimal snapshot
		status = PluginStatus{
			State: PluginStateStopped,
		}
	}
	
	resourceUsage, _ := rm.pluginManager.GetPluginResourceUsage(txn.PluginName)
	
	snapshot := &ConfigSnapshot{
		PluginName:    txn.PluginName,
		Config:        status.Config,
		State:         status.State,
		Timestamp:     time.Now(),
		ResourceUsage: &resourceUsage,
		HealthStatus:  status.Health,
	}
	
	rm.mu.Lock()
	rm.snapshots[txn.PluginName] = snapshot
	rm.mu.Unlock()
	
	txn.OldSnapshot = snapshot
	txn.Phase = PhaseStopping
	return nil
}

// executeReload performs the actual reload operation
func (rm *ReloadManager) executeReload(ctx context.Context, txn *ReloadTransaction) error {
	// Phase 3: Stop and unload plugin gracefully
	rm.notifyPhase(PhaseStopping)
	txn.Phase = PhaseStopping
	
	// First stop the plugin if running
	if err := rm.pluginManager.StopPlugin(ctx, txn.PluginName); err != nil {
		// Allow stop to fail if plugin wasn't running
		if txn.OldSnapshot.State != PluginStateRunning && txn.OldSnapshot.State != PluginStateStopped {
			log.Printf("Plugin %s wasn't running, continuing reload", txn.PluginName)
		} else if txn.OldSnapshot.State != PluginStateStopped {
			return fmt.Errorf("failed to stop plugin: %w", err)
		}
	}
	
	// Then unload the plugin to free up the registration
	if err := rm.pluginManager.UnloadPlugin(ctx, txn.PluginName); err != nil {
		// Allow unload to fail if plugin wasn't loaded
		log.Printf("Warning: failed to unload plugin %s: %v", txn.PluginName, err)
	}
	
	// Phase 4: Load new configuration
	rm.notifyPhase(PhaseLoading)
	txn.Phase = PhaseLoading
	if err := rm.pluginManager.LoadPlugin(*txn.NewConfig); err != nil {
		return fmt.Errorf("failed to load plugin: %w", err)
	}
	
	// Phase 5: Start plugin with new config
	rm.notifyPhase(PhaseStarting)
	txn.Phase = PhaseStarting
	if err := rm.pluginManager.StartPlugin(ctx, txn.PluginName); err != nil {
		return fmt.Errorf("failed to start plugin: %w", err)
	}
	
	// Phase 6: Health validation
	rm.notifyPhase(PhaseHealthCheck)
	txn.Phase = PhaseHealthCheck
	if err := rm.validateHealth(ctx, txn.PluginName); err != nil {
		return fmt.Errorf("health validation failed: %w", err)
	}
	
	return nil
}

// validateHealth checks plugin health after reload
func (rm *ReloadManager) validateHealth(ctx context.Context, pluginName string) error {
	ctx, cancel := context.WithTimeout(ctx, rm.healthTimeout)
	defer cancel()
	
	retryCount := 0
	maxRetries := 10
	retryInterval := 500 * time.Millisecond
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()
	
	// Get the actual plugin instance to check health
	plugin, exists := rm.pluginManager.registry.GetPlugin(pluginName)
	if !exists {
		return fmt.Errorf("plugin %s not found for health check", pluginName)
	}
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health validation timeout after %v", rm.healthTimeout)
			
		case <-ticker.C:
			// Call plugin's Health method directly
			health := plugin.Health()
			
			switch health.Status {
			case interfaces.StatusHealthy:
				return nil
			case interfaces.StatusError, interfaces.StatusWarning:
				retryCount++
				if retryCount >= maxRetries {
					return fmt.Errorf("plugin unhealthy after %d retries: %s", maxRetries, health.Message)
				}
			case interfaces.StatusStopped:
				return fmt.Errorf("plugin is stopped")
			default:
				// Continue monitoring for unknown status
			}
		}
	}
}

// rollback restores plugin to previous state
func (rm *ReloadManager) rollback(ctx context.Context, txn *ReloadTransaction) error {
	txn.Phase = PhaseRollback
	
	if txn.OldSnapshot == nil {
		return fmt.Errorf("no snapshot available for rollback")
	}
	
	// Stop current (failed) plugin
	if err := rm.pluginManager.StopPlugin(ctx, txn.PluginName); err != nil {
		log.Printf("Warning: failed to stop plugin during rollback: %v", err)
	}
	
	// Unload the failed plugin to free up registration
	if err := rm.pluginManager.UnloadPlugin(ctx, txn.PluginName); err != nil {
		log.Printf("Warning: failed to unload plugin during rollback: %v", err)
	}
	
	// Restore previous configuration
	if txn.OldSnapshot.Config != nil {
		if err := rm.pluginManager.LoadPlugin(*txn.OldSnapshot.Config); err != nil {
			return fmt.Errorf("failed to restore plugin config: %w", err)
		}
		
		// Start if it was running before
		if txn.OldSnapshot.State == PluginStateRunning {
			if err := rm.pluginManager.StartPlugin(ctx, txn.PluginName); err != nil {
				return fmt.Errorf("failed to start restored plugin: %w", err)
			}
		}
	}
	
	return nil
}

// cleanupTransaction cleans up transaction state
func (rm *ReloadManager) cleanupTransaction(txnID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	if txn, exists := rm.transactions[txnID]; exists {
		delete(rm.activeReloads, txn.PluginName)
		delete(rm.transactions, txnID)
	}
}

// notifyPhase calls the phase callback if set
func (rm *ReloadManager) notifyPhase(phase ReloadPhase) {
	if rm.phaseCallback != nil {
		rm.phaseCallback(phase)
	}
}

// generateTxnID generates a unique transaction ID
func generateTxnID() string {
	return fmt.Sprintf("reload-%d-%s", time.Now().UnixNano(), uuid.New().String()[:8])
}

// ConfigVersionManager manages configuration version history
type ConfigVersionManager interface {
	SaveVersion(pluginName string, config *PluginConfig, reason string) error
	GetVersion(pluginName string, versionID string) (*PluginConfig, error)
	ListVersions(pluginName string, limit int) ([]ConfigVersion, error)
	RollbackToVersion(pluginName string, versionID string) error
	PruneOldVersions(keepCount int) error
}

// ConfigVersion represents a configuration snapshot with metadata
type ConfigVersion struct {
	ID         string                 `json:"id"`
	PluginName string                 `json:"plugin_name"`
	Config     *PluginConfig          `json:"config"`
	Timestamp  time.Time              `json:"timestamp"`
	Reason     string                 `json:"reason"`
	Hash       string                 `json:"hash"`
	ParentID   string                 `json:"parent_id"`
	Status     VersionStatus          `json:"status"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// VersionStatus represents the status of a configuration version
type VersionStatus string

const (
	VersionStatusActive   VersionStatus = "active"
	VersionStatusStable   VersionStatus = "stable"
	VersionStatusFailed   VersionStatus = "failed"
	VersionStatusArchived VersionStatus = "archived"
)

// FileSystemVersionManager implements version management using filesystem
type FileSystemVersionManager struct {
	baseDir       string
	maxVersions   int
	retentionDays int
	mu            sync.RWMutex
}

// NewFileSystemVersionManager creates a new filesystem-based version manager
func NewFileSystemVersionManager(baseDir string) *FileSystemVersionManager {
	return &FileSystemVersionManager{
		baseDir:       baseDir,
		maxVersions:   10,
		retentionDays: 30,
	}
}

// SaveVersion saves a configuration version
func (vm *FileSystemVersionManager) SaveVersion(pluginName string, config *PluginConfig, reason string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	// Create version ID with timestamp
	versionID := fmt.Sprintf("%s-%s",
		time.Now().Format("20060102-150405"),
		generateShortID())
	
	// Calculate config hash for deduplication
	configHash := vm.calculateHash(config)
	
	// Create plugin version directory if needed
	pluginDir := filepath.Join(vm.baseDir, pluginName)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create version directory: %w", err)
	}
	
	// Save config file
	versionFile := filepath.Join(pluginDir, versionID+".yaml")
	if err := vm.writeConfigFile(versionFile, config); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}
	
	// Update version index
	version := ConfigVersion{
		ID:         versionID,
		PluginName: pluginName,
		Config:     config,
		Timestamp:  time.Now(),
		Reason:     reason,
		Hash:       configHash,
		Status:     VersionStatusActive,
		Metadata: map[string]interface{}{
			"user": os.Getenv("USER"),
		},
	}
	
	if err := vm.updateVersionIndex(pluginName, version); err != nil {
		return fmt.Errorf("failed to update version index: %w", err)
	}
	
	// Update current symlink
	currentLink := filepath.Join(pluginDir, "current")
	os.Remove(currentLink) // Remove old link if exists
	if err := os.Symlink(versionFile, currentLink); err != nil {
		log.Printf("Failed to create current symlink: %v", err)
	}
	
	// Prune old versions
	if err := vm.pruneOldVersionsForPlugin(pluginName); err != nil {
		log.Printf("Failed to prune old versions: %v", err)
	}
	
	return nil
}

// GetVersion retrieves a specific version
func (vm *FileSystemVersionManager) GetVersion(pluginName string, versionID string) (*PluginConfig, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	
	versionFile := filepath.Join(vm.baseDir, pluginName, versionID+".yaml")
	return vm.loadConfigFromFile(versionFile)
}

// ListVersions lists configuration versions
func (vm *FileSystemVersionManager) ListVersions(pluginName string, limit int) ([]ConfigVersion, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	
	indexFile := filepath.Join(vm.baseDir, pluginName, "versions.json")
	data, err := os.ReadFile(indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []ConfigVersion{}, nil
		}
		return nil, err
	}
	
	var versions []ConfigVersion
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, err
	}
	
	// Sort by timestamp descending
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Timestamp.After(versions[j].Timestamp)
	})
	
	if limit > 0 && limit < len(versions) {
		return versions[:limit], nil
	}
	
	return versions, nil
}

// RollbackToVersion rolls back to a specific version
func (vm *FileSystemVersionManager) RollbackToVersion(pluginName string, versionID string) error {
	config, err := vm.GetVersion(pluginName, versionID)
	if err != nil {
		return fmt.Errorf("failed to load version %s: %w", versionID, err)
	}
	
	// Save as new version with rollback reason
	return vm.SaveVersion(pluginName, config, fmt.Sprintf("rollback_to_%s", versionID))
}

// PruneOldVersions removes old versions across all plugins
func (vm *FileSystemVersionManager) PruneOldVersions(keepCount int) error {
	// Not implemented for brevity
	return nil
}

// Helper methods

func (vm *FileSystemVersionManager) calculateHash(config *PluginConfig) string {
	data, _ := json.Marshal(config)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (vm *FileSystemVersionManager) writeConfigFile(path string, config *PluginConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (vm *FileSystemVersionManager) loadConfigFromFile(path string) (*PluginConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var config PluginConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

func (vm *FileSystemVersionManager) updateVersionIndex(pluginName string, version ConfigVersion) error {
	indexFile := filepath.Join(vm.baseDir, pluginName, "versions.json")
	
	// Read existing versions
	var versions []ConfigVersion
	if data, err := os.ReadFile(indexFile); err == nil {
		json.Unmarshal(data, &versions)
	}
	
	// Add new version
	versions = append(versions, version)
	
	// Write updated index
	data, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(indexFile, data, 0644)
}

func (vm *FileSystemVersionManager) pruneOldVersionsForPlugin(pluginName string) error {
	versions, err := vm.ListVersions(pluginName, 0)
	if err != nil {
		return err
	}
	
	if len(versions) <= vm.maxVersions {
		return nil
	}
	
	// Remove oldest versions
	for i := vm.maxVersions; i < len(versions); i++ {
		versionFile := filepath.Join(vm.baseDir, pluginName, versions[i].ID+".yaml")
		if err := os.Remove(versionFile); err != nil {
			log.Printf("Failed to remove old version %s: %v", versions[i].ID, err)
		}
	}
	
	// Update index
	versions = versions[:vm.maxVersions]
	data, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return err
	}
	
	indexFile := filepath.Join(vm.baseDir, pluginName, "versions.json")
	return os.WriteFile(indexFile, data, 0644)
}

func generateShortID() string {
	return uuid.New().String()[:8]
}