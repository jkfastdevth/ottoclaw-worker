package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes for AES-256
	plaintext := []byte("Hello, OttoClaw!")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Error("Ciphertext should not be equal to plaintext")
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted content mismatch, got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptInvalidKey(t *testing.T) {
	key1 := []byte("0123456789abcdef0123456789abcdef")
	key2 := []byte("abcdef0123456789abcdef0123456789")
	plaintext := []byte("Sensitive data")

	ciphertext, _ := Encrypt(plaintext, key1)

	_, err := Decrypt(ciphertext, key2)
	if err == nil {
		t.Error("Decrypt should fail with invalid key")
	}
}
