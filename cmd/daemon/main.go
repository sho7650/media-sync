package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sho7650/media-sync/internal/core"
	"github.com/sho7650/media-sync/internal/plugins"
	"github.com/sho7650/media-sync/pkg/core/interfaces"
)

func main() {
	log.Println("🚀 Starting media-sync daemon (Phase 2.2.1)...")

	// Initialize plugin manager with advanced features
	pluginManager := plugins.NewPluginManager()
	
	// Basic health check of core interfaces (preserved from recovery)
	media := core.MediaItem{
		ID:          "daemon-init-001",
		URL:         "https://example.com/init.jpg",
		ContentType: "image/jpeg",
	}

	if err := media.Validate(); err != nil {
		log.Fatalf("❌ Core validation failed: %v", err)
	}

	fmt.Println("✅ Core interface validation passed")

	// Start health monitoring with auto-recovery (Phase 2.2.1 feature)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start plugin health monitoring
	go func() {
		log.Println("🔍 Starting plugin health monitoring...")
		pluginManager.StartHealthMonitoringWithRecovery(30*time.Second, true)
	}()

	// Register lifecycle hooks (Phase 2.2.1 feature)
	err := pluginManager.RegisterLifecycleHook("plugin_start", func(ctx context.Context, pluginName string) error {
		log.Printf("🔌 Plugin '%s' started successfully", pluginName)
		return nil
	})
	if err != nil {
		log.Printf("⚠️  Failed to register lifecycle hook: %v", err)
	}

	err = pluginManager.RegisterLifecycleHook("plugin_error", func(ctx context.Context, pluginName string) error {
		log.Printf("🔥 Plugin '%s' encountered an error, attempting recovery", pluginName)
		return nil
	})
	if err != nil {
		log.Printf("⚠️  Failed to register error hook: %v", err)
	}

	fmt.Println("🎯 Phase 2.2.1 Features Initialized:")
	fmt.Println("  • Plugin lifecycle management")
	fmt.Println("  • Health monitoring & auto-recovery")
	fmt.Println("  • Resource usage tracking")
	fmt.Println("  • Multi-auth support (OAuth2, JWT, APIKey, Basic)")
	fmt.Println("  • Streaming data processing")
	fmt.Println("  • Advanced plugin state management")

	// Display capabilities
	fmt.Println("\n📋 Available Media Types:")
	for _, mediaType := range []interfaces.MediaType{
		interfaces.MediaTypePhoto,
		interfaces.MediaTypeVideo,
		interfaces.MediaTypeText,
		interfaces.MediaTypeLink,
		interfaces.MediaTypeAudio,
	} {
		fmt.Printf("  • %s\n", mediaType)
	}

	fmt.Println("\n🔄 Available Sync Modes:")
	for _, syncMode := range []interfaces.SyncMode{
		interfaces.SyncModeBatch,
		interfaces.SyncModeWebhook,
		interfaces.SyncModeStreaming,
		interfaces.SyncModeQueue,
	} {
		fmt.Printf("  • %s\n", syncMode)
	}

	fmt.Println("\n🔐 Supported Authentication Types:")
	for _, authType := range []interfaces.AuthType{
		interfaces.AuthTypeOAuth2,
		interfaces.AuthTypeJWT,
		interfaces.AuthTypeAPIKey,
		interfaces.AuthTypeBasic,
	} {
		fmt.Printf("  • %s\n", authType)
	}

	fmt.Println("\n✨ Media-sync daemon is ready with Phase 2.2.1 advanced features!")

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		log.Printf("📡 Received signal %v, initiating graceful shutdown...", sig)
	case <-ctx.Done():
		log.Println("📡 Context cancelled, shutting down...")
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	log.Println("🛑 Stopping plugin health monitoring...")
	pluginManager.StopHealthMonitoring()

	log.Println("🔌 Shutting down plugin manager...")
	if err := pluginManager.GracefulShutdown(shutdownCtx); err != nil {
		log.Printf("⚠️  Plugin manager shutdown warning: %v", err)
	}

	log.Println("✅ Media-sync daemon stopped gracefully")
}