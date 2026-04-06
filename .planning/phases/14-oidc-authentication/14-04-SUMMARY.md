---
phase: 14-oidc-authentication
plan: 04
subsystem: oidc-admin-config
tags: [oidc, admin, config, api, ui]
dependency_graph:
  requires: [14-01, 14-02, 14-03]
  provides: [oidc-config-api, oidc-admin-ui, oidc-env-seeding]
  affects: [admin-panel, session-management, startup]
tech_stack:
  added: []
  patterns: [secret-masking, discovery-validation, env-var-seeding, session-revocation]
key_files:
  created: []
  modified:
    - internal/server/handler.go
    - internal/server/routes.go
    - internal/session/manager.go
    - cmd/screwsbox/main.go
    - internal/server/templates/admin.html
    - internal/server/static/js/admin.js
    - internal/server/static/css/admin.css
decisions:
  - "OIDC config routes placed inside /api route group (auth + CSRF protected)"
  - "Env var seeding runs once at startup, skips if DB already has config"
  - "Secret field uses password type with eye toggle, never returned in API"
metrics:
  duration: 3min
  completed: "2026-04-06"
  tasks_completed: 2
  tasks_total: 3
  files_modified: 7
status: checkpoint-pending
---

# Phase 14 Plan 04: OIDC Admin Configuration Summary

OIDC admin config API with discovery validation, secret masking, session revocation on disable, env var seeding, and full admin UI form.

## What Was Done

### Task 1: OIDC config API handlers, session revocation, env var seeding (4abc45e)

- Added `DeleteByAuthMethod` passthrough method on `session.Manager` for OIDC session revocation
- Implemented `handleGetOIDCConfig()` -- GET /api/oidc/config returns masked config (secret never exposed)
- Implemented `handleUpdateOIDCConfig()` -- PUT /api/oidc/config with:
  - Required field validation when enabling (issuer URL, client ID, client secret)
  - Discovery validation via `oidcpkg.ValidateDiscovery` before save (T-14-15 mitigation)
  - Secret preservation (empty secret = keep existing)
  - Session revocation when OIDC is disabled (D-22)
- Added OIDC config routes inside protected `/api` route group (auth + CSRF required, T-14-18)
- Added `seedOIDCFromEnv()` in main.go -- seeds OIDC config from `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_DISPLAY_NAME` env vars on first startup only

### Task 2: Admin OIDC config form UI and JavaScript (dfeae29)

- Added OIDC/SSO subsection in Authentication card with divider and subsection title
- Toggle checkbox to show/hide OIDC config fields
- Form fields: Display Name, Issuer URL, Client ID, Client Secret (password type)
- Secret eye toggle button to show/hide secret value
- Save button changes label to "Disable OIDC and revoke sessions" when disabling
- Form submits to PUT /api/oidc/config with CSRF token
- Success/error feedback with appropriate messages
- CSS: subsection title styling, secret field layout with flex

### Task 3: Verify OIDC flow end-to-end (CHECKPOINT -- PENDING)

Human verification required with a real OIDC provider.

## Deviations from Plan

None -- plan executed exactly as written.

## Threat Mitigations Applied

| Threat | Mitigation |
|--------|-----------|
| T-14-15 (Spoofing via malicious issuer) | ValidateDiscovery called before save |
| T-14-16 (Info disclosure of client secret) | GetOIDCConfigMasked used in GET handler |
| T-14-18 (Config tampering) | Routes behind authMiddleware + csrfProtect |

## Known Stubs

None -- all data flows are wired to real endpoints.

## Self-Check: PASSED

All 7 files exist. Both commits (4abc45e, dfeae29) verified. All acceptance criteria content markers confirmed present.
