package oidc

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AES-256 requires a 32-byte key. Passing any other length triggers
// aes.NewCipher's error branch in both Encrypt and Decrypt.

func TestEncryptStateCookieWrongKeySize(t *testing.T) {
	short := make([]byte, 16) // valid AES-128 key, but our code expects 32 bytes for AES-256
	// AES accepts 16/24/32 byte keys, so this would actually succeed. Use truly bad size.
	bad := []byte{1, 2, 3}
	_, err := EncryptStateCookie(bad, &StateCookie{State: "s"})
	assert.Error(t, err)
	_ = short
}

func TestDecryptStateCookieWrongKeySize(t *testing.T) {
	// First produce a valid ciphertext with a 32-byte key.
	good := makeTestKey(t)
	enc, err := EncryptStateCookie(good, &StateCookie{State: "s"})
	require.NoError(t, err)

	// Now decrypt with a malformed key.
	bad := []byte{1, 2, 3}
	_, err = DecryptStateCookie(bad, enc)
	assert.Error(t, err)
}

// Base64 decode error: pass a string that is non-empty but not valid base64.
// "***" contains characters that aren't in the URL-safe base64 alphabet.
func TestDecryptStateCookieBadBase64(t *testing.T) {
	key := makeTestKey(t)
	_, err := DecryptStateCookie(key, "***not-base64!!!")
	assert.Error(t, err)
}

// Ciphertext that base64-decodes to fewer bytes than the GCM nonce size
// (12 for AES-256-GCM) hits the "ciphertext too short" branch (line 54-56)
// distinct from the empty-string and bad-base64 paths.
func TestDecryptStateCookieTooShortAfterBase64(t *testing.T) {
	key := makeTestKey(t)
	short := base64.URLEncoding.EncodeToString(make([]byte, 5))
	_, err := DecryptStateCookie(key, short)
	assert.Error(t, err)
}

// Ciphertext that is valid base64 and longer than the GCM nonce size,
// but whose payload was not produced by AES-256-GCM with this key, fails
// at gcm.Open. Using random bytes does this.
func TestDecryptStateCookieGarbagePayload(t *testing.T) {
	key := makeTestKey(t)
	garbage := make([]byte, 32) // longer than nonceSize=12
	for i := range garbage {
		garbage[i] = byte(i)
	}
	enc := base64.URLEncoding.EncodeToString(garbage)
	_, err := DecryptStateCookie(key, enc)
	assert.Error(t, err)
}

// Decrypted plaintext that successfully passes GCM auth but is not valid
// JSON triggers the json.Unmarshal error branch (cookie.go:63-65).
// We hand-roll a ciphertext using the same key: encrypt non-JSON bytes,
// then call DecryptStateCookie which will get past Open() and fail on
// Unmarshal.
func TestDecryptStateCookieNotJSON(t *testing.T) {
	key := makeTestKey(t)

	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := make([]byte, gcm.NonceSize())
	// Deterministic nonce; doesn't matter for this test since we're not
	// trying to verify uniqueness.
	for i := range nonce {
		nonce[i] = 0xAB
	}
	sealed := gcm.Seal(nonce, nonce, []byte("not-json-at-all"), nil)
	enc := base64.URLEncoding.EncodeToString(sealed)

	_, err = DecryptStateCookie(key, enc)
	assert.Error(t, err)
}
