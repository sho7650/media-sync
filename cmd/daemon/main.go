package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sho7650/media-sync/internal/core"
)

func main() {
	log.Println("Starting media-sync daemon...")

	// Basic health check to verify core interfaces are working
	media := core.MediaItem{
		ID:          "daemon-test-001",
		URL:         "https://example.com/test.jpg",
		ContentType: "image/jpeg",
	}

	if err := media.Validate(); err != nil {
		log.Fatalf("Core validation failed: %v", err)
	}

	fmt.Println("✅ Core interfaces validated successfully")
	fmt.Println("🚀 Media-sync daemon is ready (Phase 1 Foundation)")

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down gracefully...", sig)
	case <-ctx.Done():
		log.Println("Context cancelled, shutting down...")
	}

	log.Println("Media-sync daemon stopped")
}