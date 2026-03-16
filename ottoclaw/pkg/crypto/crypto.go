package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// Encrypt encrypts data using AES-256-GCM with the provided passphrase.
// A random nonce is generated and prefixed to the ciphertext.
func Encrypt(data []byte, passphrase []byte) ([]byte, error) {
	block, err := aes.NewCipher(deriveKey(passphrase))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// Decrypt decrypts data encrypted with Encrypt.
func Decrypt(ciphertext []byte, passphrase []byte) ([]byte, error) {
	block, err := aes.NewCipher(deriveKey(passphrase))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// deriveKey simplifies key derivation for the demo.
// In a full production env, use Argon2 or Scrypt.
func deriveKey(passphrase []byte) []byte {
	key := make([]byte, 32)
	copy(key, passphrase)
	return key
}
