package oidc

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return key
}

func TestEncryptDecryptStateCookie(t *testing.T) {
	key := makeTestKey(t)
	sc := &StateCookie{State: "s1", Nonce: "n1", Verifier: "v1"}

	encrypted, err := EncryptStateCookie(key, sc)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	decrypted, err := DecryptStateCookie(key, encrypted)
	require.NoError(t, err)
	assert.Equal(t, "s1", decrypted.State)
	assert.Equal(t, "n1", decrypted.Nonce)
	assert.Equal(t, "v1", decrypted.Verifier)
}

func TestDecryptStateCookie_WrongKey(t *testing.T) {
	keyA := makeTestKey(t)
	keyB := makeTestKey(t)
	sc := &StateCookie{State: "s1", Nonce: "n1", Verifier: "v1"}

	encrypted, err := EncryptStateCookie(keyA, sc)
	require.NoError(t, err)

	_, err = DecryptStateCookie(keyB, encrypted)
	assert.Error(t, err)
}

func TestDecryptStateCookie_Truncated(t *testing.T) {
	key := makeTestKey(t)
	sc := &StateCookie{State: "s1", Nonce: "n1", Verifier: "v1"}

	encrypted, err := EncryptStateCookie(key, sc)
	require.NoError(t, err)

	// Truncate to 5 chars
	_, err = DecryptStateCookie(key, encrypted[:5])
	assert.Error(t, err)
}

func TestDecryptStateCookie_Empty(t *testing.T) {
	key := makeTestKey(t)
	_, err := DecryptStateCookie(key, "")
	assert.Error(t, err)
}

func TestMakeStateCookieHTTP_Attributes(t *testing.T) {
	c := MakeStateCookieHTTP("testvalue", false)
	assert.Equal(t, StateCookieName, c.Name)
	assert.Equal(t, "testvalue", c.Value)
	assert.Equal(t, "/", c.Path)
	assert.Equal(t, StateCookieMaxAge, c.MaxAge)
	assert.True(t, c.HttpOnly)
	assert.False(t, c.Secure)
}

func TestClearStateCookieHTTP_MaxAge(t *testing.T) {
	c := ClearStateCookieHTTP(false)
	assert.Equal(t, StateCookieName, c.Name)
	assert.Equal(t, -1, c.MaxAge)
	assert.True(t, c.HttpOnly)
}

func TestGenerateState_Length(t *testing.T) {
	s := GenerateState()
	assert.Len(t, s, 64)
}

func TestGenerateNonce_Length(t *testing.T) {
	n := GenerateNonce()
	assert.Len(t, n, 64)
}

func TestGenerateState_Uniqueness(t *testing.T) {
	s1 := GenerateState()
	s2 := GenerateState()
	assert.NotEqual(t, s1, s2)
}
