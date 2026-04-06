---
phase: 14-oidc-authentication
verified: 2026-04-06T00:00:00Z
status: human_needed
score: 4/5 must-haves verified (1 requires human with live OIDC provider)
re_verification: false
human_verification:
  - test: "OIDC end-to-end login flow with a real provider"
    expected: "User clicks SSO button on /login, is redirected to the OIDC provider, completes authentication, and lands on / with display name shown in header"
    why_human: "Requires a running OIDC provider (Authelia/Google). The callback handler calls NewProviderFromConfig (network call) and ExchangeAndVerify — cannot test without a real JWKS endpoint and token exchange."
  - test: "Admin OIDC config save with live discovery validation"
    expected: "Saving a valid issuer URL in the admin panel calls ValidateDiscovery, returns success, and the config is persisted. Saving an invalid URL returns an error message."
    why_human: "ValidateDiscovery makes an HTTP GET to the issuer's .well-known/openid-configuration. Requires a real or mock OIDC server to validate the success path."
  - test: "Disable OIDC revokes active OIDC sessions"
    expected: "After disabling OIDC in the admin panel and saving, any browser tab logged in via SSO is invalidated. The admin sees 'OIDC disabled. Active SSO sessions have been revoked.' feedback."
    why_human: "Requires active OIDC sessions from a previous login via a real provider."
---

# Phase 14: OIDC Authentication Verification Report

**Phase Goal:** Users can log in via an external OIDC provider (Authelia, Google) and the admin can configure the provider from the admin panel
**Verified:** 2026-04-06
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth                                                                 | Status      | Evidence                                                                                                      |
|----|-----------------------------------------------------------------------|-------------|---------------------------------------------------------------------------------------------------------------|
| 1  | User can click SSO button and complete OIDC provider redirect flow    | ? HUMAN     | Button exists in login.html (`{{if .OIDCEnabled}}<button class="sso-btn">`), routes wired, end-to-end needs live provider |
| 2  | OIDC flow uses PKCE, state, and nonce parameters                      | ✓ VERIFIED  | `provider.AuthURL(state, nonce, verifier)` calls `oauth2.S256ChallengeOption(verifier)` + `gooidc.Nonce(nonce)` in provider.go; nonce verified in `ExchangeAndVerify` with explicit `idToken.Nonce != expectedNonce` check |
| 3  | Local username/password login remains functional when OIDC is enabled  | ✓ VERIFIED  | login.html always renders local auth form; OIDC button conditional via `{{if .OIDCEnabled}}`; local login path in `handleLoginPost` is independent of OIDC config |
| 4  | Admin can configure OIDC provider from auth settings section           | ✓ VERIFIED  | admin.html has `id="oidc-form"` with all fields; GET/PUT `/api/oidc/config` handlers in handler.go; routes registered in protected `/api` group in routes.go |
| 5  | When no OIDC configured, login page shows only local auth              | ✓ VERIFIED  | `handleLoginPage` only sets `OIDCEnabled=true` when `oidcCfg.Enabled && oidcCfg.IssuerURL != ""`; SSO button absent by default |

**Score:** 4/5 truths verified (1 requires human with live OIDC provider)

### Required Artifacts

| Artifact                                        | Expected                                      | Status     | Details                                                                          |
|-------------------------------------------------|-----------------------------------------------|------------|----------------------------------------------------------------------------------|
| `internal/session/session.go`                   | Session struct with AuthMethod + DisplayName  | ✓ VERIFIED | Line 9: `AuthMethod string`; Line 10: `DisplayName string`                       |
| `internal/session/store.go`                     | Store interface with DeleteByAuthMethod       | ✓ VERIFIED | Interface includes `DeleteByAuthMethod(ctx context.Context, method string) (int, error)` |
| `internal/session/memory.go`                    | MemoryStore DeleteByAuthMethod implementation | ✓ VERIFIED | Line 74: `func (m *MemoryStore) DeleteByAuthMethod`                              |
| `internal/session/manager.go`                   | CreateWithMethod, DeleteByAuthMethod on Mgr   | ✓ VERIFIED | Line 54: `func (m *Manager) CreateWithMethod`; Line 138: `func (m *Manager) DeleteByAuthMethod` |
| `internal/model/model.go`                       | OIDCConfig and OIDCUser structs               | ✓ VERIFIED | Line 69: `type OIDCConfig struct`; Line 79: `type OIDCUser struct`               |
| `internal/store/store.go`                       | OIDC config CRUD, oidc_user table, enc key    | ✓ VERIFIED | Lines 1237-1360: GetOIDCConfig, GetOIDCConfigMasked, SaveOIDCConfig, UpsertOIDCUser, GetOIDCUserBySub, GetOrCreateEncryptionKey |
| `internal/store/store_test.go`                  | Tests for OIDC config and user CRUD           | ✓ VERIFIED | TestOIDCConfigSaveAndGet, TestOIDCConfigGetDefault, TestOIDCConfigMaskedHidesSecret, TestOIDCConfigSavePreservesSecret, TestUpsertOIDCUser, TestUpsertOIDCUserUpdates, TestGetOIDCUserBySub_NotFound, TestGetOrCreateEncryptionKey, TestGetOrCreateEncryptionKey_Length |
| `internal/oidc/config.go`                       | PKCE helpers and StateCookie struct           | ✓ VERIFIED | StateCookie struct, GenerateState(), GenerateNonce(), MakeStateCookieHTTP, ClearStateCookieHTTP |
| `internal/oidc/cookie.go`                       | AES-GCM encrypt/decrypt for state cookie      | ✓ VERIFIED | EncryptStateCookie and DecryptStateCookie with `cipher.NewGCM`                   |
| `internal/oidc/provider.go`                     | OIDC provider wrapper                         | ✓ VERIFIED | NewProviderFromConfig, AuthURL, ExchangeAndVerify (with nonce check), ValidateDiscovery |
| `internal/oidc/cookie_test.go`                  | Round-trip encryption tests                   | ✓ VERIFIED | TestEncryptDecryptStateCookie present; 9 cookie/config tests total               |
| `internal/server/handler.go`                    | OIDC start/callback handlers, admin API       | ✓ VERIFIED | handleOIDCStart (L632), handleOIDCCallback (L686), handleGetOIDCConfig (L802), handleUpdateOIDCConfig (L822) |
| `internal/server/handler_test.go`               | OIDC handler tests with mock StoreService     | ✓ VERIFIED | 7 tests: TestLoginPageWithOIDC/WithoutOIDC/WithOIDCError, TestOIDCCallbackMissingCode/MissingStateCookie/StateMismatch, TestOIDCStartDisabled |
| `internal/server/routes.go`                     | Public OIDC routes + protected API routes     | ✓ VERIFIED | Lines 45-46: `/auth/oidc` and `/auth/callback`; Lines 77-78: `/oidc/config` GET+PUT inside `/api` group |
| `internal/server/templates/login.html`          | SSO button + or divider + local auth form     | ✓ VERIFIED | `{{if .OIDCEnabled}}<button class="sso-btn">` with divider; local form always present |
| `internal/server/static/css/login.css`          | SSO button and divider styles                 | ✓ VERIFIED | `.sso-btn` at L126, `.login-divider` at L147                                     |
| `internal/server/templates/admin.html`          | OIDC config form in Authentication card       | ✓ VERIFIED | `id="oidc-form"`, admin-subsection-title, oidc-enabled, oidc-issuer, oidc-client-id, oidc-client-secret, secret-toggle |
| `internal/server/static/js/admin.js`            | OIDC form handling JS                         | ✓ VERIFIED | fetch `/api/oidc/config`, toggle logic, "Disable OIDC and revoke sessions" label |
| `internal/server/static/css/admin.css`          | OIDC subsection styles                        | ✓ VERIFIED | `.admin-subsection-title` at L204, `.secret-field` at L214                       |
| `cmd/screwsbox/main.go`                         | OIDC env var seeding on startup               | ✓ VERIFIED | `seedOIDCFromEnv` called at L94, reads `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_DISPLAY_NAME` |

### Key Link Verification

| From                              | To                              | Via                                      | Status      | Details                                                              |
|-----------------------------------|---------------------------------|------------------------------------------|-------------|----------------------------------------------------------------------|
| `handler.go handleOIDCStart`      | `oidc/provider.go Provider.AuthURL` | Handler builds auth URL via wrapper  | ✓ WIRED     | L681: `authURL := provider.AuthURL(state, nonce, verifier)`          |
| `handler.go handleOIDCCallback`   | `session/manager.go CreateWithMethod` | Callback creates session with oidc | ✓ WIRED     | L776: `srv.sessions.CreateWithMethod(w, r, username, "oidc", claims.DisplayName)` |
| `routes.go`                       | `handler.go`                    | Public route registration                | ✓ WIRED     | L45-46: `r.Get("/auth/oidc", ...)` and `r.Get("/auth/callback", ...)` outside authMiddleware |
| `admin.js`                        | `handler.go handleGetOIDCConfig` | fetch GET /api/oidc/config on page load | ✓ WIRED     | L404: `fetch('/api/oidc/config', {method: 'PUT', ...})` (also GETs for populated form fields) |
| `handler.go handleUpdateOIDCConfig` | `oidc/provider.go ValidateDiscovery` | Discovery validation before save  | ✓ WIRED     | L854: `if err := oidcpkg.ValidateDiscovery(ctx, req.IssuerURL); err != nil` |
| `handler.go handleUpdateOIDCConfig` | `session/manager.go DeleteByAuthMethod` | Revoke OIDC sessions on disable  | ✓ WIRED     | L877: `count, err := srv.sessions.DeleteByAuthMethod(ctx, "oidc")` when `wasEnabled && !req.Enabled` |
| `internal/oidc/cookie.go`         | `crypto/aes + crypto/cipher`    | AES-GCM encrypt/decrypt                  | ✓ WIRED     | `cipher.NewGCM` at lines 24 and 49                                   |
| `internal/oidc/provider.go`       | `github.com/coreos/go-oidc/v3/oidc` | oidc.NewProvider for discovery       | ✓ WIRED     | go.mod includes `github.com/coreos/go-oidc/v3 v3.17.0`; `gooidc.NewProvider` in provider.go |

### Data-Flow Trace (Level 4)

| Artifact                      | Data Variable       | Source                                         | Produces Real Data | Status      |
|-------------------------------|---------------------|------------------------------------------------|--------------------|-------------|
| `login.html` SSO button       | `.OIDCEnabled`      | `handleLoginPage` → `store.GetOIDCConfig()`    | Yes — DB query     | ✓ FLOWING   |
| `admin.html` OIDC form        | `.OIDCEnabled` etc. | `handleAdmin` → `store.GetOIDCConfigMasked()`  | Yes — DB query     | ✓ FLOWING   |
| `admin.js` OIDC form fields   | fetch `/api/oidc/config` | `handleGetOIDCConfig` → `store.GetOIDCConfigMasked()` | Yes — DB query | ✓ FLOWING |
| `handleOIDCCallback` session  | `claims.DisplayName` | `provider.ExchangeAndVerify()` → ID token claims | Yes — live provider | ? LIVE (human) |

### Behavioral Spot-Checks

| Behavior                              | Command                                                                  | Result                    | Status  |
|---------------------------------------|--------------------------------------------------------------------------|---------------------------|---------|
| All internal tests pass               | `go test ./internal/... -count=1`                                         | 5 packages OK             | ✓ PASS  |
| Binary compiles cleanly               | `go build ./...`                                                          | Exit 0, no output         | ✓ PASS  |
| OIDC package tests specifically       | `go test ./internal/oidc/ -count=1`                                       | ok (14 tests)             | ✓ PASS  |
| Session tests with AuthMethod         | `go test ./internal/session/ -count=1`                                    | ok                        | ✓ PASS  |
| Store OIDC CRUD tests                 | `go test ./internal/store/ -count=1`                                      | ok (9 OIDC tests present) | ✓ PASS  |
| Server handler OIDC tests             | `go test ./internal/server/ -run "TestOIDCCallback\|TestLoginPage\|TestOIDCStart" -count=1` | 7 tests pass | ✓ PASS |
| Live OIDC redirect flow               | Requires browser + real provider                                           | Not testable offline      | ? SKIP  |

### Requirements Coverage

| Requirement | Source Plans | Description                                                      | Status        | Evidence                                                                                    |
|-------------|--------------|------------------------------------------------------------------|---------------|---------------------------------------------------------------------------------------------|
| OIDC-01     | 14-01, 14-03 | User can login via OIDC provider                                 | ? HUMAN       | Full flow implemented; end-to-end with real provider pending human checkpoint in 14-04 Task 3 |
| OIDC-02     | 14-02, 14-03 | OIDC flow uses PKCE + state + nonce per RFC 9700                 | ✓ SATISFIED   | AuthURL uses S256ChallengeOption + GenerateState + GenerateNonce; ExchangeAndVerify checks nonce |
| OIDC-03     | 14-03        | Local auth remains available as fallback when OIDC is enabled    | ✓ SATISFIED   | login.html local form always rendered; `handleLoginPost` independent of OIDC config        |
| OIDC-04     | 14-01, 14-04 | Admin can configure OIDC provider                                | ✓ SATISFIED   | GET/PUT /api/oidc/config handlers; full admin form with all required fields                 |
| ADMN-02     | 14-04        | Auth settings section (local auth + OIDC config)                 | ✓ SATISFIED   | admin.html has existing local auth form + new OIDC/SSO subsection under Authentication card |

### Anti-Patterns Found

| File                          | Line | Pattern                                      | Severity     | Impact                                                    |
|-------------------------------|------|----------------------------------------------|--------------|-----------------------------------------------------------|
| `14-04-SUMMARY.md` (meta)     | 33   | `status: checkpoint-pending`                 | ℹ Info       | Plan 04 Task 3 (human verify OIDC flow) was not completed — expected per design |
| `handler.go`                  | 640  | `if err != nil \|\| !cfg.Enabled` — nil cfg silently fails | ⚠ Warning | `GetOIDCConfig` returns nil when OIDC not configured; the nil+disabled check lumps both cases together, which is intentional but implicit |

No blockers found. No placeholder or stub implementations detected. All data flows connect to real DB queries or real external calls.

### Human Verification Required

#### 1. Complete OIDC Login Flow

**Test:** Start the app with a configured OIDC provider (e.g., Authelia). Navigate to `/login`. Verify the "Sign in with [Provider]" SSO button appears. Click it, complete authentication at the provider, and confirm redirect back to `/` with the user's display name in the header.

**Expected:** After clicking the SSO button, the browser redirects to the OIDC provider's authorization endpoint with `code_challenge`, `code_challenge_method=S256`, `state`, and `nonce` query parameters visible in the URL. After authentication, the provider redirects to `/auth/callback`, the app creates a session with `AuthMethod=oidc`, and redirects to `/`. The header shows the OIDC user's display name.

**Why human:** Requires a running OIDC provider with a registered client. `NewProviderFromConfig` performs OIDC discovery (HTTP GET to `/.well-known/openid-configuration`) and `ExchangeAndVerify` exchanges the authorization code for tokens and verifies ID token JWKs — both require a live provider.

#### 2. Admin OIDC Config Save and Discovery Validation

**Test:** In the admin panel Authentication card, scroll to "OIDC / SSO". Toggle "Enable OIDC", fill in a valid issuer URL, client ID, client secret, and display name, then save.

**Expected:** On save, the app calls `ValidateDiscovery` against the issuer URL. With a reachable provider the save succeeds and shows "OIDC settings saved." With an invalid URL it shows "Could not reach OIDC provider at ... Check the Issuer URL." The client secret field clears after save and shows "Secret is configured. Enter a new value to replace it."

**Why human:** `ValidateDiscovery` makes a real HTTP request. Success path requires a reachable OIDC server.

#### 3. Disable OIDC Revokes Active OIDC Sessions

**Test:** With OIDC configured and at least one user logged in via SSO, disable OIDC in the admin panel and save.

**Expected:** Admin sees "OIDC disabled. Active SSO sessions have been revoked." The previously logged-in SSO user is logged out on next request.

**Why human:** Requires active OIDC sessions from a prior live login flow.

### Gaps Summary

No automated gaps found. All 20 required artifacts exist and are substantive. All 8 key links are wired. Both AES-GCM crypto links and OIDC discovery links are wired to real implementations. All 5 internal test suites pass (60+ tests).

The single outstanding item is Plan 04 Task 3: the blocking human checkpoint for end-to-end OIDC verification with a real provider. This is by design — the phase SUMMARY notes `status: checkpoint-pending` for this task. Three human verification items are documented above for the person completing the checkpoint.

---

_Verified: 2026-04-06_
_Verifier: Claude (gsd-verifier)_
