package main

import (
	"context"
	"image-resizer/internal/config"
	"image-resizer/internal/server"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.LoadConfig()

	// Ensure directories exist
	if err := os.MkdirAll(cfg.UploadFolder, 0o750); err != nil {
		log.Fatalf("Failed to create upload folder: %v", err)
	}
	if err := os.MkdirAll(cfg.ProcessedFolder, 0o750); err != nil {
		log.Fatalf("Failed to create processed folder: %v", err)
	}

	s := server.NewServer(cfg)

	// IMP-04 FIX: Graceful shutdown support. Listen for OS interrupt signals
	// (SIGINT, SIGTERM) and gracefully shut down the server, allowing
	// in-progress requests to complete within a 5-second timeout.
	go func() {
		log.Printf("Server starting on port %s...", cfg.Port)
		if err := s.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	if err := s.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
		cancel()
		os.Exit(1)
	}
	cancel()
	log.Println("Server exited gracefully")
}
