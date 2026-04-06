package oidc

import (
	"context"
	"strings"
	"testing"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestAuthURL_ContainsPKCE(t *testing.T) {
	// Construct a Provider manually with a known oauth2 config (no discovery needed).
	p := &Provider{
		oauth2Cfg: &oauth2.Config{
			ClientID:    "test-client",
			RedirectURL: "http://localhost/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://example.com/auth",
				TokenURL: "https://example.com/token",
			},
			Scopes: []string{gooidc.ScopeOpenID, "email", "profile"},
		},
		issuer: "https://example.com",
	}

	url := p.AuthURL("test-state", "test-nonce", "test-verifier")

	assert.Contains(t, url, "code_challenge=")
	assert.Contains(t, url, "code_challenge_method=S256")
	assert.Contains(t, url, "state=test-state")
	assert.Contains(t, url, "nonce=test-nonce")
}

func TestAuthURL_ContainsScopes(t *testing.T) {
	p := &Provider{
		oauth2Cfg: &oauth2.Config{
			ClientID:    "test-client",
			RedirectURL: "http://localhost/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://example.com/auth",
				TokenURL: "https://example.com/token",
			},
			Scopes: []string{gooidc.ScopeOpenID, "email", "profile"},
		},
		issuer: "https://example.com",
	}

	url := p.AuthURL("s", "n", "v")

	// URL-encoded scope should contain openid, email, profile
	assert.Contains(t, url, "scope=")
	// Scopes are space-separated and URL-encoded
	assert.True(t, strings.Contains(url, "openid"), "URL should contain openid scope")
	assert.True(t, strings.Contains(url, "email"), "URL should contain email scope")
	assert.True(t, strings.Contains(url, "profile"), "URL should contain profile scope")
}

func TestIDTokenClaims_Fields(t *testing.T) {
	claims := IDTokenClaims{
		Sub:         "user-123",
		Email:       "user@example.com",
		DisplayName: "Test User",
		AvatarURL:   "https://example.com/avatar.png",
		Issuer:      "https://example.com",
		Nonce:       "abc123",
	}

	assert.Equal(t, "user-123", claims.Sub)
	assert.Equal(t, "user@example.com", claims.Email)
	assert.Equal(t, "Test User", claims.DisplayName)
	assert.Equal(t, "https://example.com/avatar.png", claims.AvatarURL)
	assert.Equal(t, "https://example.com", claims.Issuer)
	assert.Equal(t, "abc123", claims.Nonce)
}

func TestNewProviderFromConfig_InvalidIssuer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := NewProviderFromConfig(ctx, "https://invalid.example.com", "client", "secret", "http://localhost/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discovery failed")
}

func TestValidateDiscovery_InvalidIssuer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := ValidateDiscovery(ctx, "https://invalid.example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discovery failed")
}
