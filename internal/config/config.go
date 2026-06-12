package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"strconv"
)

type Config struct {
	UploadFolder     string
	ProcessedFolder  string
	MaxContentLength int64
	Port             string
	APIKey           string
	Env              string
	SigningKey       string // HMAC signing key for private download links (env: SIGNING_KEY)
	LinkExpiry       int    // Private link expiry in hours (env: LINK_EXPIRY, default: 24)
}

func LoadConfig() *Config {
	env := getEnv("ENV", "development")
	apiKey := getEnv("API_KEY", "pro_resizer_key_2026")

	// #nosec G101
	if env == "production" && (apiKey == "pro_resizer_key_2026" || apiKey == "") {
		log.Fatalf("FATAL: A secure API_KEY environment variable is required in production. (Default or empty keys are not allowed)")
	}

	// Signing key for private download links — auto-generate a 32-byte hex key if not provided
	signingKey := getEnv("SIGNING_KEY", "")
	if signingKey == "" {
		keyBytes := make([]byte, 32)
		if _, err := rand.Read(keyBytes); err != nil {
			log.Fatalf("FATAL: Failed to generate random signing key: %v", err)
		}
		signingKey = hex.EncodeToString(keyBytes)
		log.Println("Generated random SIGNING_KEY (set SIGNING_KEY env var for persistent key)")
	}

	// Link expiry in hours, default 24
	linkExpiry := 24
	if expStr := getEnv("LINK_EXPIRY", ""); expStr != "" {
		if val, err := strconv.Atoi(expStr); err == nil && val > 0 {
			linkExpiry = val
		}
	}

	return &Config{
		UploadFolder:     getEnv("UPLOAD_FOLDER", "static/uploads"),
		ProcessedFolder:  getEnv("PROCESSED_FOLDER", "static/processed"),
		MaxContentLength: 16 * 1024 * 1024, // 16MB
		Port:             getEnv("PORT", "5000"),
		APIKey:           apiKey,
		Env:              env,
		SigningKey:       signingKey,
		LinkExpiry:       linkExpiry,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
