package config

import (
	"log"
	"os"
)

type Config struct {
	UploadFolder    string
	ProcessedFolder string
	MaxContentLength int64
	Port            string
	APIKey          string
	Env             string
}

func LoadConfig() *Config {
	env := getEnv("ENV", "development")
	apiKey := getEnv("API_KEY", "pro_resizer_key_2026")

	// #nosec G101
	if env == "production" && apiKey == "pro_resizer_key_2026" {
		log.Fatalf("FATAL: Default API_KEY is not allowed in production. Please set a secure API_KEY environment variable.")
	}

	return &Config{
		UploadFolder:    getEnv("UPLOAD_FOLDER", "static/uploads"),
		ProcessedFolder: getEnv("PROCESSED_FOLDER", "static/processed"),
		MaxContentLength: 16 * 1024 * 1024, // 16MB
		Port:            getEnv("PORT", "5000"),
		APIKey:          apiKey,
		Env:             env,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
