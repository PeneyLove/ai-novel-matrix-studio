// Package secure_test provides tests for the secure encryption package.
package secure_test

import (
	"testing"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/secure"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	fp := "test-machine-fingerprint-abc123"
	plaintext := "sk-abc123def456ghi789"

	encrypted, err := secure.Encrypt(plaintext, fp)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted == plaintext || encrypted == "" {
		t.Fatal("encrypted value should differ from plaintext")
	}

	decrypted, err := secure.Decrypt(encrypted, fp)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("roundtrip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongFingerprint(t *testing.T) {
	plaintext := "sk-top-secret"
	encrypted, err := secure.Encrypt(plaintext, "machine-a")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = secure.Decrypt(encrypted, "machine-b")
	if err == nil {
		t.Fatal("expected error when decrypting with wrong fingerprint")
	}
}

func TestFingerprintIsStable(t *testing.T) {
	fp1 := secure.Fingerprint()
	fp2 := secure.Fingerprint()
	if fp1 != fp2 {
		t.Fatalf("fingerprint should be stable: %q vs %q", fp1, fp2)
	}
	if len(fp1) != 64 {
		t.Fatalf("fingerprint should be 64 hex chars (SHA256), got %d", len(fp1))
	}
}
