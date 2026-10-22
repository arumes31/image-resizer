package config

import (
	"strings"
	"testing"
)

func TestValidateAPIKey(t *testing.T) {
	cases := []struct {
		key     string
		wantErr bool
	}{
		{"", false},
		{legacyDefaultAPIKey, true},
		{"short-key", true},
		{strings.Repeat("x", MinAPIKeyLength-1), true},
		{strings.Repeat("x", MinAPIKeyLength), false},
	}
	for _, tc := range cases {
		if err := ValidateAPIKey(tc.key); (err != nil) != tc.wantErr {
			t.Errorf("ValidateAPIKey(%q) error = %v, wantErr %v", tc.key, err, tc.wantErr)
		}
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("API_KEY", "")
	cfg := LoadConfig()
	if cfg.APIKey != "" {
		t.Errorf("expected no default API key, got %q", cfg.APIKey)
	}
	if cfg.MaxContentLength != 32*1024*1024 {
		t.Errorf("unexpected MaxContentLength %d", cfg.MaxContentLength)
	}
	if cfg.MaxFilesPerRequest != 20 || cfg.RateLimitPerMinute != 30 {
		t.Errorf("unexpected limits: files=%d rate=%d", cfg.MaxFilesPerRequest, cfg.RateLimitPerMinute)
	}
}
