package processor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// DeriveKey derives a 32-byte AES key from a password using SHA-256.
func DeriveKey(password string) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}

// EncryptData encrypts data using AES-GCM with the given password.
// The nonce is prepended to the ciphertext and the result is base64-encoded.
func EncryptData(data []byte, password string) (string, error) {
	if len(password) == 0 {
		return "", errors.New("encryption password is required")
	}
	key := DeriveKey(password)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	// Seal prepends the nonce to the ciphertext
	encrypted := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptData decrypts AES-GCM encrypted data with the given password.
// The input is a base64-encoded string where the nonce is prepended to the ciphertext.
func DecryptData(encodedData string, password string) ([]byte, error) {
	if len(password) == 0 {
		return nil, errors.New("decryption password is required")
	}
	data, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, err
	}
	key := DeriveKey(password)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("encrypted data too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
