package main

import (
	"image-resizer/internal/config"
	"image-resizer/internal/server"
	"log"
	"os"
)

func main() {
	cfg := config.LoadConfig()

	// Ensure directories exist
	if err := os.MkdirAll(cfg.UploadFolder, 0755); err != nil {
		log.Fatalf("Failed to create upload folder: %v", err)
	}
	if err := os.MkdirAll(cfg.ProcessedFolder, 0755); err != nil {
		log.Fatalf("Failed to create processed folder: %v", err)
	}

	s := server.NewServer(cfg)
	log.Printf("Server starting on port %s...", cfg.Port)
	if err := s.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
