// Package secure provides AES-256-GCM encryption for protecting sensitive
// configuration values (API keys) at rest in .novelAgent/config.yaml.
//
// The encryption key is derived from a machine fingerprint to prevent
// plaintext config files from being copied to another machine and reused.
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
)

// Fingerprint returns a machine-specific hash used as key derivation input.
// It combines hostname, OS, and architecture to create a stable per-machine secret.
func Fingerprint() string {
	host, _ := os.Hostname()
	raw := fmt.Sprintf("%s|%s|%s", host, runtime.GOOS, runtime.GOARCH)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// deriveKey creates a 32-byte AES-256 key from the fingerprint and an optional salt.
func deriveKey(fingerprint string) []byte {
	h := sha256.Sum256([]byte("ai-novel-agent:" + fingerprint))
	return h[:]
}

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// Returns the hex-encoded ciphertext (nonce || ciphertext).
func Encrypt(plaintext, fingerprint string) (string, error) {
	key := deriveKey(fingerprint)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("secure: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("secure: create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("secure: generate nonce: %w", err)
	}

	// ciphertext = nonce || encrypted
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a hex-encoded ciphertext produced by Encrypt.
func Decrypt(hexCiphertext, fingerprint string) (string, error) {
	data, err := hex.DecodeString(hexCiphertext)
	if err != nil {
		return "", fmt.Errorf("secure: decode hex: %w", err)
	}

	key := deriveKey(fingerprint)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("secure: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("secure: create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("secure: ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("secure: decrypt failed (wrong machine?): %w", err)
	}

	return string(plaintext), nil
}
