package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
)

func init() {
	if envOr("ENCRYPTION_KEY", "") == "" {
		log.Println("WARNING: ENCRYPTION_KEY is not set — API tokens will be encrypted with a dev-only key. " +
			"Generate a strong key with `openssl rand -hex 32` and set it via ENCRYPTION_KEY env var before deploying to production.")
	}
}

func encKey() []byte {
	// Dev fallback is intentionally not a recognizable branded string.
	// In production, always set ENCRYPTION_KEY to a random 32+ byte value.
	k := envOr("ENCRYPTION_KEY", "dev-only-key-set-ENCRYPTION_KEY-in-prod")
	h := sha256.Sum256([]byte(k))
	return h[:]
}

func encryptToken(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(encKey())
	if err != nil {
		return "", fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func decryptToken(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		// Not base64 → treat as plaintext (backwards compat for existing unencrypted tokens)
		return ciphertext, nil
	}
	block, err := aes.NewCipher(encKey())
	if err != nil {
		return "", fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	if len(data) < gcm.NonceSize() {
		// Too short to be a valid ciphertext → treat as plaintext
		return ciphertext, nil
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		// Decryption failed → treat as plaintext (unencrypted legacy value)
		return ciphertext, nil
	}
	return string(pt), nil
}
