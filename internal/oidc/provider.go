package oidc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// IDTokenClaims holds the claims extracted from an OIDC ID token.
type IDTokenClaims struct {
	Sub         string
	Email       string
	DisplayName string
	AvatarURL   string
	Issuer      string
	Nonce       string
}

// Provider wraps go-oidc provider and oauth2 config for a single OIDC flow.
type Provider struct {
	oauth2Cfg    *oauth2.Config
	verifier     *gooidc.IDTokenVerifier
	oidcProvider *gooidc.Provider
	issuer       string
}

// NewProviderFromConfig creates a Provider by performing OIDC discovery on the issuer URL.
// callbackURL is the full URL for the OIDC callback endpoint (e.g., "https://app.example.com/auth/callback").
// Returns error if discovery fails.
func NewProviderFromConfig(ctx context.Context, issuerURL, clientID, clientSecret, callbackURL string) (*Provider, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	oidcProvider, err := gooidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed for %s: %w", issuerURL, err)
	}

	oauth2Cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     oidcProvider.Endpoint(),
		RedirectURL:  callbackURL,
		Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
	}

	verifier := oidcProvider.Verifier(&gooidc.Config{
		ClientID: clientID,
	})

	return &Provider{
		oauth2Cfg:    oauth2Cfg,
		verifier:     verifier,
		oidcProvider: oidcProvider,
		issuer:       issuerURL,
	}, nil
}

// AuthURL builds the authorization URL with PKCE state and nonce parameters.
// state, nonce: random tokens stored in encrypted cookie.
// verifier: PKCE code verifier (generated via oauth2.GenerateVerifier).
func (p *Provider) AuthURL(state, nonce, verifier string) string {
	return p.oauth2Cfg.AuthCodeURL(
		state,
		oauth2.S256ChallengeOption(verifier),
		gooidc.Nonce(nonce),
	)
}

// ExchangeAndVerify exchanges the authorization code for tokens, verifies the ID token,
// and extracts claims. Returns claims and error.
// pkceVerifier is the PKCE code verifier from the encrypted state cookie.
// expectedNonce is the nonce from the encrypted state cookie for verification.
func (p *Provider) ExchangeAndVerify(ctx context.Context, code, pkceVerifier, expectedNonce string) (*IDTokenClaims, error) {
	// Exchange code for token with PKCE verifier.
	token, err := p.oauth2Cfg.Exchange(ctx, code, oauth2.VerifierOption(pkceVerifier))
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Extract raw ID token.
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, fmt.Errorf("no id_token in token response")
	}

	// Verify ID token signature and claims.
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("ID token verification failed: %w", err)
	}

	// Manual nonce verification -- go-oidc does NOT do this automatically.
	if idToken.Nonce != expectedNonce {
		return nil, fmt.Errorf("nonce mismatch: expected %q, got %q", expectedNonce, idToken.Nonce)
	}

	// Extract claims from ID token first.
	var claims struct {
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		DisplayName       string `json:"display_name"`
		Picture           string `json:"picture"`
		Sub               string `json:"sub"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	// Authelia (and some other providers) only put profile/email claims in the
	// userinfo endpoint, not in the ID token. Fetch userinfo as fallback.
	if claims.Email == "" || (claims.Name == "" && claims.PreferredUsername == "") {
		userInfo, err := p.oidcProvider.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if err != nil {
			slog.Warn("OIDC userinfo fetch failed, using ID token claims only", "err", err)
		} else {
			var uiClaims struct {
				Email             string `json:"email"`
				Name              string `json:"name"`
				PreferredUsername string `json:"preferred_username"`
				DisplayName       string `json:"display_name"`
				Picture           string `json:"picture"`
			}
			if err := userInfo.Claims(&uiClaims); err == nil {
				if claims.Email == "" {
					claims.Email = uiClaims.Email
				}
				if claims.Name == "" {
					claims.Name = uiClaims.Name
				}
				if claims.PreferredUsername == "" {
					claims.PreferredUsername = uiClaims.PreferredUsername
				}
				if claims.DisplayName == "" {
					claims.DisplayName = uiClaims.DisplayName
				}
				if claims.Picture == "" {
					claims.Picture = uiClaims.Picture
				}
			}
		}
	}

	// Pick the best display name: name > display_name > preferred_username > email
	displayName := claims.Name
	if displayName == "" {
		displayName = claims.DisplayName
	}
	if displayName == "" {
		displayName = claims.PreferredUsername
	}
	if displayName == "" {
		displayName = claims.Email
	}

	return &IDTokenClaims{
		Sub:         claims.Sub,
		Email:       claims.Email,
		DisplayName: displayName,
		AvatarURL:   claims.Picture,
		Issuer:      p.issuer,
		Nonce:       idToken.Nonce,
	}, nil
}

// ValidateDiscovery attempts OIDC discovery to verify the issuer URL is reachable and valid.
// Returns nil on success or error describing the failure.
func ValidateDiscovery(ctx context.Context, issuerURL string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := gooidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return fmt.Errorf("OIDC discovery failed for %s: %w", issuerURL, err)
	}
	return nil
}
