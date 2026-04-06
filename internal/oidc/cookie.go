package oidc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// EncryptStateCookie encrypts a StateCookie into a base64-encoded string using AES-256-GCM.
// key must be exactly 32 bytes (256 bits).
func EncryptStateCookie(key []byte, sc *StateCookie) (string, error) {
	plaintext, err := json.Marshal(sc)
	if err != nil {
		return "", fmt.Errorf("marshal state cookie: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.URLEncoding.EncodeToString(sealed), nil
}

// DecryptStateCookie decrypts a base64-encoded AES-256-GCM ciphertext back into a StateCookie.
func DecryptStateCookie(key []byte, encoded string) (*StateCookie, error) {
	if encoded == "" {
		return nil, errors.New("empty ciphertext")
	}
	ciphertext, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	var sc StateCookie
	if err := json.Unmarshal(plaintext, &sc); err != nil {
		return nil, fmt.Errorf("unmarshal state cookie: %w", err)
	}
	return &sc, nil
}
