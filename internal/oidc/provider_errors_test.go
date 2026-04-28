package oidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test OIDC server -----------------------------------------------------
//
// A minimal httptest-based OIDC provider that:
// - Serves /.well-known/openid-configuration
// - Serves a JWKS with one RS256 key
// - Serves /token with programmable response (success / failure / no_id_token)
// - Serves /userinfo with programmable response
//
// We hand-roll JWT signing so that no extra dependency is added.

type tokenScenario string

const (
	scenarioOK              tokenScenario = "ok"
	scenarioTokenError      tokenScenario = "token_error"
	scenarioNoIDToken       tokenScenario = "no_id_token"
	scenarioBadIDTokenSig   tokenScenario = "bad_id_token_sig" //nolint:gosec // G101: scenario label, not a credential
	scenarioWrongNonceToken tokenScenario = "wrong_nonce_token"
	scenarioOKEmptyEmail    tokenScenario = "ok_empty_email"
)

type oidcTestServer struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	scenario   tokenScenario
	nonce      string // nonce embedded in id_token when scenarioOK / scenarioWrongNonceToken
	userInfo   *http.HandlerFunc
}

func newOIDCTestServer(t *testing.T) *oidcTestServer {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	s := &oidcTestServer{privateKey: priv, scenario: scenarioOK}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		base := s.server.URL
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test fixture; encode failure is impossible for this static map
			"issuer":                                base,
			"authorization_endpoint":                base + "/auth",
			"token_endpoint":                        base + "/token",
			"jwks_uri":                              base + "/jwks",
			"userinfo_endpoint":                     base + "/userinfo",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		n := encodeBigInt(priv.N)
		e := encodeBigInt(big.NewInt(int64(priv.E)))
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test fixture; encode failure is impossible for this static map
			"keys": []any{
				map[string]any{
					"kty": "RSA",
					"alg": "RS256",
					"use": "sig",
					"kid": "k1",
					"n":   n,
					"e":   e,
				},
			},
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		switch s.scenario {
		case scenarioTokenError:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		case scenarioNoIDToken:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"a","token_type":"Bearer"}`))
		case scenarioBadIDTokenSig:
			// id_token whose signature is garbage.
			tok := makeUnsignedJWT(s.server.URL, "test-client", "n-good")
			tok += ".AAAA"
			writeTokenResponse(w, tok)
		case scenarioWrongNonceToken:
			tok, err := signJWT(priv, s.server.URL, "test-client", "n-wrong")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeTokenResponse(w, tok)
		case scenarioOKEmptyEmail:
			tok, err := signEmptyEmailJWT(priv, s.server.URL, "test-client", s.nonce)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeTokenResponse(w, tok)
		default: // scenarioOK
			tok, err := signJWT(priv, s.server.URL, "test-client", s.nonce)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeTokenResponse(w, tok)
		}
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if s.userInfo != nil {
			(*s.userInfo)(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{ //nolint:errchkjson // test fixture; encode failure is impossible for this static map
			"sub":   "user-1",
			"email": "u@example.com",
			"name":  "U",
		})
	})

	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

func writeTokenResponse(w http.ResponseWriter, idToken string) {
	w.Header().Set("Content-Type", "application/json")
	body := map[string]any{
		"access_token": "access-xyz",
		"token_type":   "Bearer",
		"id_token":     idToken,
		"expires_in":   3600,
	}
	_ = json.NewEncoder(w).Encode(body) //nolint:errchkjson // test fixture; encode failure is impossible for this static map
}

func encodeBigInt(n *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}

// makeUnsignedJWT builds a JWT header.payload pair (no signature appended).
// The caller is expected to append a signature segment.
func makeUnsignedJWT(issuer, audience, nonce string) string {
	header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": "k1"}
	now := time.Now().Unix()
	payload := map[string]any{
		"iss":   issuer,
		"sub":   "test-sub",
		"aud":   audience,
		"exp":   now + 3600,
		"iat":   now,
		"nonce": nonce,
		"email": "u@example.com",
		"name":  "U",
	}
	hb, _ := json.Marshal(header)  //nolint:errchkjson // marshal of map[string]string cannot fail
	pb, _ := json.Marshal(payload) //nolint:errchkjson // marshal of map[string]any with primitive values cannot fail
	return base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(pb)
}

func signJWT(priv *rsa.PrivateKey, issuer, audience, nonce string) (string, error) {
	signingInput := makeUnsignedJWT(issuer, audience, nonce)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// signEmptyEmailJWT mints a token with no email or name claims so that
// ExchangeAndVerify hits the userinfo-fallback branch.
func signEmptyEmailJWT(priv *rsa.PrivateKey, issuer, audience, nonce string) (string, error) {
	header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": "k1"}
	now := time.Now().Unix()
	payload := map[string]any{
		"iss":   issuer,
		"sub":   "test-sub",
		"aud":   audience,
		"exp":   now + 3600,
		"iat":   now,
		"nonce": nonce,
	}
	hb, _ := json.Marshal(header)  //nolint:errchkjson // marshal of map[string]string cannot fail
	pb, _ := json.Marshal(payload) //nolint:errchkjson // marshal of map[string]any with primitive values cannot fail
	signingInput := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(pb)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// --- Tests ----------------------------------------------------------------

// Happy-path discovery → covers NewProviderFromConfig success path (43-60).
func TestNewProviderFromConfigDiscoverySucceeds(t *testing.T) {
	srv := newOIDCTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := NewProviderFromConfig(ctx, srv.server.URL, "test-client", "secret", "http://localhost/cb")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

// Happy-path ValidateDiscovery → covers the success-return at line 180-181.
func TestValidateDiscoverySucceeds(t *testing.T) {
	srv := newOIDCTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ValidateDiscovery(ctx, srv.server.URL)
	require.NoError(t, err)
}

// Token-exchange failure (server returns 400) → covers line 81-83.
func TestExchangeAndVerifyTokenExchangeFails(t *testing.T) {
	srv := newOIDCTestServer(t)
	srv.scenario = scenarioTokenError

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := NewProviderFromConfig(ctx, srv.server.URL, "test-client", "secret", "http://localhost/cb")
	require.NoError(t, err)

	_, err = p.ExchangeAndVerify(ctx, "code-x", "verifier-x", "n-good")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token exchange failed")
}

// Token response missing id_token → covers line 87-89.
func TestExchangeAndVerifyMissingIDToken(t *testing.T) {
	srv := newOIDCTestServer(t)
	srv.scenario = scenarioNoIDToken

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := NewProviderFromConfig(ctx, srv.server.URL, "test-client", "secret", "http://localhost/cb")
	require.NoError(t, err)

	_, err = p.ExchangeAndVerify(ctx, "code-x", "verifier-x", "n-good")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no id_token")
}

// id_token signature mismatch → covers line 93-95.
func TestExchangeAndVerifyBadIDTokenSignature(t *testing.T) {
	srv := newOIDCTestServer(t)
	srv.scenario = scenarioBadIDTokenSig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := NewProviderFromConfig(ctx, srv.server.URL, "test-client", "secret", "http://localhost/cb")
	require.NoError(t, err)

	_, err = p.ExchangeAndVerify(ctx, "code-x", "verifier-x", "n-good")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ID token verification failed")
}

// id_token has wrong nonce vs expected → covers line 98-100.
func TestExchangeAndVerifyNonceMismatch(t *testing.T) {
	srv := newOIDCTestServer(t)
	srv.scenario = scenarioWrongNonceToken

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := NewProviderFromConfig(ctx, srv.server.URL, "test-client", "secret", "http://localhost/cb")
	require.NoError(t, err)

	_, err = p.ExchangeAndVerify(ctx, "code-x", "verifier-x", "n-good")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonce mismatch")
}

// Happy-path exchange/verify → covers the rest of ExchangeAndVerify
// including the userinfo fallback and claim-merging code paths.
func TestExchangeAndVerifyHappyPath(t *testing.T) {
	srv := newOIDCTestServer(t)
	srv.scenario = scenarioOK
	srv.nonce = "n-good"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := NewProviderFromConfig(ctx, srv.server.URL, "test-client", "secret", "http://localhost/cb")
	require.NoError(t, err)

	claims, err := p.ExchangeAndVerify(ctx, "code-x", "verifier-x", "n-good")
	require.NoError(t, err)
	assert.Equal(t, "test-sub", claims.Sub)
	assert.Equal(t, "u@example.com", claims.Email)
	assert.Equal(t, "U", claims.DisplayName)
	assert.Equal(t, srv.server.URL, claims.Issuer)
	assert.Equal(t, "n-good", claims.Nonce)
}

// id_token has no email/name claims → ExchangeAndVerify falls back to
// the /userinfo endpoint and merges its claims (lines 117-145).
func TestExchangeAndVerifyUserinfoFallbackSucceeds(t *testing.T) {
	srv := newOIDCTestServer(t)
	srv.scenario = scenarioOKEmptyEmail
	srv.nonce = "n-good"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := NewProviderFromConfig(ctx, srv.server.URL, "test-client", "secret", "http://localhost/cb")
	require.NoError(t, err)

	claims, err := p.ExchangeAndVerify(ctx, "code-x", "verifier-x", "n-good")
	require.NoError(t, err)
	// Email should come from /userinfo since the id_token has none.
	assert.Equal(t, "u@example.com", claims.Email)
	assert.Equal(t, "U", claims.DisplayName)
}

// id_token has no email + /userinfo returns 500 → covers the slog.Warn
// branch where userinfo fetch fails (lines 119-121). The function still
// succeeds because the empty-claim ID-token is acceptable.
func TestExchangeAndVerifyUserinfoFallbackFails(t *testing.T) {
	srv := newOIDCTestServer(t)
	srv.scenario = scenarioOKEmptyEmail
	srv.nonce = "n-good"
	failHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv.userInfo = &failHandler

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := NewProviderFromConfig(ctx, srv.server.URL, "test-client", "secret", "http://localhost/cb")
	require.NoError(t, err)

	claims, err := p.ExchangeAndVerify(ctx, "code-x", "verifier-x", "n-good")
	// Function does not propagate userinfo errors — it logs a warning and
	// returns whatever the id_token had (possibly empty fields).
	require.NoError(t, err)
	assert.Equal(t, "test-sub", claims.Sub)
	assert.Empty(t, claims.Email)
}
