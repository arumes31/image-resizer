package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

// GenerateSignedURL creates a time-limited signed download URL.
func GenerateSignedURL(filename, signingKey string, expiryHours int) string {
	expiry := time.Now().Add(time.Duration(expiryHours) * time.Hour).Unix()
	message := filename + "|" + strconv.FormatInt(expiry, 10)
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))
	return "/private/download/" + filename + "?expires=" + strconv.FormatInt(expiry, 10) + "&sig=" + signature
}

// ValidateSignedURL checks if a download URL signature is valid and not expired.
func ValidateSignedURL(filename, expires, signature, signingKey string) bool {
	// Check expiry
	expiryInt, err := strconv.ParseInt(expires, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expiryInt {
		return false
	}

	// Verify HMAC using constant-time comparison
	message := filename + "|" + expires
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(message))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}
