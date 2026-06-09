package config

import (
	"os"
)

type Config struct {
	UploadFolder    string
	ProcessedFolder string
	MaxContentLength int64
	Port            string
}

func LoadConfig() *Config {
	return &Config{
		UploadFolder:    getEnv("UPLOAD_FOLDER", "static/uploads"),
		ProcessedFolder: getEnv("PROCESSED_FOLDER", "static/processed"),
		MaxContentLength: 16 * 1024 * 1024, // 16MB
		Port:            getEnv("PORT", "5000"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
