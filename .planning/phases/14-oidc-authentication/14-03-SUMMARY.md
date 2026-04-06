---
phase: 14-oidc-authentication
plan: "03"
subsystem: auth
tags: [oidc, login, session, handlers, tests]
dependency_graph:
  requires: [14-01, 14-02]
  provides: [oidc-login-flow, oidc-callback, sso-button, header-user-display]
  affects: [internal/server, internal/session, internal/model, templates]
tech_stack:
  added: []
  patterns: [oidc-pkce-flow, encrypted-state-cookie, display-name-in-session]
key_files:
  created:
    - internal/server/static/css/login.css (SSO button + divider styles appended)
  modified:
    - internal/session/session.go
    - internal/session/manager.go
    - internal/session/manager_test.go
    - internal/model/model.go
    - internal/server/handler.go
    - internal/server/handler_test.go
    - internal/server/routes.go
    - internal/server/templates/login.html
    - internal/server/templates/grid.html
    - internal/server/templates/admin.html
    - internal/server/static/css/app.css
decisions:
  - "CreateWithMethod takes 5 params (username, authMethod, displayName) for OIDC display name support"
  - "OIDC routes registered as public (outside authMiddleware) per security design"
  - "State mismatch returns generic auth_failed to avoid information disclosure"
metrics:
  duration: "78 minutes"
  completed: "2026-04-06T08:38:00Z"
  tasks: 3
  files: 12
---

# Phase 14 Plan 03: OIDC Login Flow Handlers Summary

OIDC start/callback handlers with PKCE+state+nonce, login page SSO button, header user display name, and 7 handler tests.

## Completed Tasks

| Task | Name | Commit | Key Changes |
|------|------|--------|-------------|
| 1 | Extend session with DisplayName, add OIDC handlers, update routes | ad1feb4 | Session.DisplayName, GetSession, handleOIDCStart, handleOIDCCallback, StoreService OIDC methods, public routes |
| 2 | Update login template, header display, and CSS | 6a3e356 | SSO button in login.html, .sso-btn/.login-divider CSS, .header-user in grid/admin templates |
| 3 | Add OIDC handler tests | 00f42b2 | 7 tests: login page with/without OIDC, error display, callback validation, start disabled |

## Deviations from Plan

None - plan executed exactly as written.

## Key Implementation Details

### OIDC Start Handler (/auth/oidc)
- Loads OIDC config, creates provider via discovery
- Generates PKCE verifier (oauth2.GenerateVerifier), state, nonce
- Encrypts state cookie with AES-GCM via GetOrCreateEncryptionKey
- Redirects to provider AuthURL with all PKCE params
- Error: provider unreachable -> /login?error=sso_unavailable

### OIDC Callback Handler (/auth/callback)
- Validates code + state query params present
- Decrypts state cookie, verifies state matches (CSRF protection)
- Clears state cookie immediately after verification
- Creates provider, exchanges code with PKCE verifier
- Nonce verified by ExchangeAndVerify
- Upserts OIDC user record (non-fatal on error)
- Creates session with AuthMethod="oidc" and DisplayName
- Redirects to / on success

### Session Changes
- DisplayName field added to Session struct
- CreateWithMethod now accepts 5th param (displayName)
- GetSession method returns full Session for header display
- Create() passes empty displayName for backward compat

### Login Page
- SSO button shown only when OIDC enabled+configured
- "or" divider separates SSO from local auth form
- Error messages from OIDC flow displayed in error banner

### Header Display
- grid.html and admin.html show user DisplayName (falls back to Username)
- .header-user styled as muted 13px text

## Test Coverage

| Test | What It Verifies |
|------|-----------------|
| TestLoginPageWithOIDC | SSO button + provider name rendered |
| TestLoginPageWithoutOIDC | No SSO button, local form only |
| TestLoginPageWithOIDCError | SSO unreachable message displayed |
| TestOIDCCallbackMissingCode | Redirect on missing code param |
| TestOIDCCallbackMissingStateCookie | Redirect on missing cookie |
| TestOIDCCallbackStateMismatch | Redirect on state CSRF mismatch |
| TestOIDCStartDisabled | Redirect to /login when OIDC off |

## Verification

- `go build ./...` -- compiles successfully
- `go vet ./...` -- no issues
- `go test ./internal/... -count=1` -- all tests pass (5 packages)
- `go test ./internal/server/ -run "TestLoginPage|TestOIDCCallback|TestOIDCStart" -count=1 -v` -- 7 OIDC tests pass

## Self-Check: PASSED

All 12 modified files exist and all 3 commits verified:
- ad1feb4: feat(14-03): add OIDC handlers
- 6a3e356: feat(14-03): update login template
- 00f42b2: test(14-03): add OIDC handler tests
