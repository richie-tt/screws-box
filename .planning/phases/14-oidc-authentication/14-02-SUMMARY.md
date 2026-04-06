---
phase: 14-oidc-authentication
plan: 02
subsystem: oidc
tags: [oidc, pkce, aes-gcm, cookie, provider, security]
dependency_graph:
  requires: []
  provides: [internal/oidc/config.go, internal/oidc/cookie.go, internal/oidc/provider.go]
  affects: []
tech_stack:
  added: [github.com/coreos/go-oidc/v3 v3.17.0, golang.org/x/oauth2 v0.36.0, github.com/go-jose/go-jose/v4 v4.1.3]
  patterns: [AES-256-GCM authenticated encryption, PKCE S256 challenge, OIDC discovery]
key_files:
  created:
    - internal/oidc/config.go
    - internal/oidc/cookie.go
    - internal/oidc/provider.go
    - internal/oidc/cookie_test.go
    - internal/oidc/provider_test.go
  modified:
    - go.mod
    - go.sum
decisions:
  - "AES-256-GCM for state cookie encryption -- authenticated encryption prevents both tampering and reading"
  - "Nonce prepended to ciphertext in Seal output -- standard GCM pattern, no separate nonce storage needed"
  - "Manual nonce verification in ExchangeAndVerify -- go-oidc does NOT verify nonce automatically"
  - "10-second timeout for OIDC discovery -- prevents indefinite hangs on unreachable providers"
metrics:
  duration: 3min
  completed: "2026-04-06T08:04:00Z"
  tasks_completed: 2
  tasks_total: 2
---

# Phase 14 Plan 02: OIDC Package Summary

**One-liner:** PKCE-protected OIDC flow with AES-256-GCM encrypted state cookies and provider wrapper around go-oidc/oauth2.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | OIDC config types, PKCE helpers, and AES-GCM cookie encryption | 4e04bbe | config.go, cookie.go, cookie_test.go, go.mod, go.sum |
| 2 | OIDC provider wrapper for discovery, auth URL, and token verification | 4e89e7b | provider.go, provider_test.go, go.mod, go.sum |

## What Was Built

### internal/oidc/config.go
- `StateCookie` struct holding State, Nonce, Verifier fields (JSON-serializable)
- `GenerateState()` / `GenerateNonce()` -- crypto/rand 64-char hex strings
- `MakeStateCookieHTTP()` / `ClearStateCookieHTTP()` -- secure cookie helpers with HttpOnly, SameSite=Lax
- Constants: `StateCookieName = "screwsbox_oidc_state"`, `StateCookieMaxAge = 600`

### internal/oidc/cookie.go
- `EncryptStateCookie(key, sc)` -- AES-256-GCM encryption, nonce prepended, base64url output
- `DecryptStateCookie(key, encoded)` -- reverse: base64url decode, GCM Open, JSON unmarshal
- Errors on: empty input, truncated ciphertext, wrong key (GCM auth tag failure)

### internal/oidc/provider.go
- `Provider` struct wrapping oauth2.Config + IDTokenVerifier
- `NewProviderFromConfig(ctx, issuer, clientID, secret, callback)` -- OIDC discovery with 10s timeout
- `AuthURL(state, nonce, verifier)` -- builds auth URL with PKCE S256 + state + nonce
- `ExchangeAndVerify(ctx, code, verifier, nonce)` -- token exchange, ID token verification, explicit nonce check
- `ValidateDiscovery(ctx, issuerURL)` -- standalone issuer validation for admin config UI
- `IDTokenClaims` struct: Sub, Email, DisplayName, AvatarURL, Issuer, Nonce

## Test Coverage

14 tests total, all passing:
- 9 cookie/config tests: round-trip, wrong key, truncated, empty, cookie attributes, state/nonce length/uniqueness
- 5 provider tests: AuthURL PKCE params, AuthURL scopes, claims struct, invalid issuer for NewProviderFromConfig and ValidateDiscovery

## Deviations from Plan

None -- plan executed exactly as written.

## Known Stubs

None -- all functions are fully implemented with real crypto operations.

## Threat Mitigations Implemented

| Threat | Mitigation |
|--------|-----------|
| T-14-04 Authorization code injection | PKCE S256 challenge in AuthURL; VerifierOption in Exchange |
| T-14-05 State cookie tampering | AES-256-GCM authenticated encryption; tampered cookie fails gcm.Open |
| T-14-06 Nonce replay | Explicit `idToken.Nonce != expectedNonce` check in ExchangeAndVerify |
| T-14-07 CSRF on callback | State parameter encrypted in cookie, verified against callback query param |
| T-14-08 Provider impersonation | OIDC discovery over HTTPS; ID token signature verified by go-oidc verifier |

## Verification

```
go test ./internal/oidc/ -count=1 -v  -- 14/14 PASS
go build ./...                         -- OK
go vet ./...                           -- OK
```

## Self-Check: PASSED

- All 5 source files exist in internal/oidc/
- Commit 4e04bbe (Task 1) found in git log
- Commit 4e89e7b (Task 2) found in git log
