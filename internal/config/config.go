package config

import (
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
)

// legacyDefaultAPIKey was the built-in key of earlier releases. It is public,
// so it is always rejected.
const legacyDefaultAPIKey = "pro_resizer_key_2026" // #nosec G101 -- known-bad value used only to reject it.

// MinAPIKeyLength is the minimum accepted length of API_KEY.
const MinAPIKeyLength = 32

type Config struct {
	UploadFolder       string
	ProcessedFolder    string
	MaxContentLength   int64
	MaxFilesPerRequest int
	RateLimitPerMinute int
	MaxConcurrent      int
	TrustedProxies     []string
	Port               string
	APIKey             string
	UIUsername         string
	UIPassword         string
	Env                string
}

func LoadConfig() *Config {
	env := getEnv("ENV", "development")
	apiKey := getEnv("API_KEY", "")

	if err := ValidateAPIKey(apiKey); err != nil {
		log.Fatalf("FATAL: %v", err)
	}
	if apiKey == "" {
		log.Println("WARNING: API_KEY is not set; the /api/v1/process endpoint is disabled.")
	}

	uiPassword := getEnv("UI_PASSWORD", "")
	if uiPassword == "" {
		log.Println("WARNING: UI_PASSWORD is not set; the web UI and downloads are reachable without authentication.")
	}

	var trustedProxies []string
	for _, p := range strings.Split(getEnv("TRUSTED_PROXIES", ""), ",") {
		if p = strings.TrimSpace(p); p != "" {
			trustedProxies = append(trustedProxies, p)
		}
	}

	return &Config{
		UploadFolder:       getEnv("UPLOAD_FOLDER", "static/uploads"),
		ProcessedFolder:    getEnv("PROCESSED_FOLDER", "static/processed"),
		MaxContentLength:   int64(getEnvInt("MAX_UPLOAD_MB", 32)) * 1024 * 1024,
		MaxFilesPerRequest: getEnvInt("MAX_FILES_PER_REQUEST", 20),
		RateLimitPerMinute: getEnvInt("RATE_LIMIT_PER_MINUTE", 30),
		MaxConcurrent:      getEnvInt("MAX_CONCURRENT_JOBS", 0),
		TrustedProxies:     trustedProxies,
		Port:               getEnv("PORT", "5000"),
		APIKey:             apiKey,
		UIUsername:         getEnv("UI_USERNAME", "admin"),
		UIPassword:         uiPassword,
		Env:                env,
	}
}

// ValidateAPIKey rejects the legacy public default and keys that are too short.
// An empty key is allowed and disables the API.
func ValidateAPIKey(apiKey string) error {
	if apiKey == "" {
		return nil
	}
	if apiKey == legacyDefaultAPIKey {
		return errors.New("API_KEY is set to the public legacy default; generate a random key (e.g. `openssl rand -hex 32`)")
	}
	if len(apiKey) < MinAPIKeyLength {
		return errors.New("API_KEY must be at least 32 characters long; generate a random key (e.g. `openssl rand -hex 32`)")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// getEnvInt returns a non-negative integer environment variable or the fallback.
func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		log.Printf("WARNING: invalid value %q for %s, using %d", value, key, fallback)
		return fallback
	}
	return n
}
