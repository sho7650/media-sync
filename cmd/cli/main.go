package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/sho7650/media-sync/internal/core"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	switch command {
	case "validate":
		runValidate()
	case "version":
		runVersion()
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("media-sync-cli - Media Synchronization CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  media-sync-cli validate    - Validate core interfaces")
	fmt.Println("  media-sync-cli version     - Show version information")
	fmt.Println("  media-sync-cli help        - Show this help message")
}

func runValidate() {
	fmt.Println("🔍 Validating core interfaces...")

	// Test MediaItem validation
	media := core.MediaItem{
		ID:          "cli-test-001",
		URL:         "https://example.com/test.jpg",
		ContentType: "image/jpeg",
	}

	if err := media.Validate(); err != nil {
		log.Fatalf("❌ MediaItem validation failed: %v", err)
	}

	// Test ServiceConfig validation
	config := core.ServiceConfig{
		Name:    "test-service",
		Type:    "input",
		Plugin:  "tumblr",
		Enabled: true,
	}

	if err := config.Validate(); err != nil {
		log.Fatalf("❌ ServiceConfig validation failed: %v", err)
	}

	fmt.Println("✅ All core interface validations passed")

	// Display example structures
	fmt.Println("\n📋 Example MediaItem:")
	mediaJSON, _ := json.MarshalIndent(media, "", "  ")
	fmt.Println(string(mediaJSON))

	fmt.Println("\n⚙️  Example ServiceConfig:")
	configJSON, _ := json.MarshalIndent(config, "", "  ")
	fmt.Println(string(configJSON))
}

func runVersion() {
	fmt.Println("media-sync-cli version 1.0.0")
	fmt.Println("Phase 1: Foundation Layer - Core Interfaces Implemented")
	fmt.Println("Build: recovery/roadmap-realignment")
}