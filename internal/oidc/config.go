package oidc

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const (
	// StateCookieName is the cookie storing encrypted OIDC state during auth flow.
	StateCookieName = "screwsbox_oidc_state"
	// StateCookieMaxAge is the maximum lifetime of the state cookie (10 minutes).
	StateCookieMaxAge = 600
)

// StateCookie holds the OIDC flow parameters encrypted in a cookie.
type StateCookie struct {
	State    string `json:"s"` //nolint:tagliatelle // short tags intentional for cookie size
	Nonce    string `json:"n"` //nolint:tagliatelle // short tags intentional for cookie size
	Verifier string `json:"v"` //nolint:tagliatelle // short tags intentional for cookie size
}

// GenerateState returns a cryptographically random 64-char hex string for OIDC state.
func GenerateState() string {
	return randomHex(32)
}

// GenerateNonce returns a cryptographically random 64-char hex string for OIDC nonce.
func GenerateNonce() string {
	return randomHex(32)
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// MakeStateCookieHTTP creates an http.Cookie for storing the encrypted state.
// secure should be true when serving over HTTPS.
func MakeStateCookieHTTP(value string, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     StateCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   StateCookieMaxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearStateCookieHTTP creates an http.Cookie that clears the state cookie.
func ClearStateCookieHTTP(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     StateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}
