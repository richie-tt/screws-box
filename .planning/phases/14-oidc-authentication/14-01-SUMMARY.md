---
phase: 14-oidc-authentication
plan: 01
subsystem: session, model, store
tags: [oidc, session, database, encryption]
dependency_graph:
  requires: []
  provides: [session-authmethod, oidc-models, oidc-store-crud, encryption-key]
  affects: [internal/session, internal/model, internal/store]
tech_stack:
  added: []
  patterns: [upsert-on-conflict, secret-preservation, masked-api-response]
key_files:
  created: []
  modified:
    - internal/session/session.go
    - internal/session/store.go
    - internal/session/memory.go
    - internal/session/manager.go
    - internal/session/manager_test.go
    - internal/session/memory_test.go
    - internal/model/model.go
    - internal/store/store.go
    - internal/store/store_test.go
decisions:
  - "Manager.Create delegates to CreateWithMethod('local') for backward compatibility"
  - "GetOIDCConfigMasked wraps GetOIDCConfig and strips secret for API safety"
  - "SaveOIDCConfig preserves existing secret when ClientSecret is empty"
  - "Encryption key stored as hex-encoded string in shelf table"
metrics:
  duration: 4min
  completed: "2026-04-06T08:06:00Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 9
---

# Phase 14 Plan 01: OIDC Data Foundation Summary

Session AuthMethod tracking, OIDC model structs, DB migrations, and store CRUD with encryption key management.

## What Was Done

### Task 1: Extend session package with AuthMethod tracking
- Added `AuthMethod string` field to `Session` struct ("local" or "oidc")
- Added `DeleteByAuthMethod(ctx, method) (int, error)` to `Store` interface
- Implemented `DeleteByAuthMethod` on `MemoryStore` with proper locking
- Added `CreateWithMethod` to `Manager`; refactored `Create` to delegate with "local"
- Added 6 new tests covering AuthMethod on Create, CreateWithMethod, DeleteByAuthMethod

**Commit:** ad7b028

### Task 2: Add OIDC models and database migrations
- Added `OIDCConfig` and `OIDCUser` model structs with JSON tags
- Added 6 migration columns to shelf table (oidc_enabled, oidc_issuer, oidc_client_id, oidc_client_secret, oidc_display_name, encryption_key)
- Added `oidc_user` table DDL with UNIQUE(sub, issuer) constraint
- Implemented store methods: `GetOIDCConfig`, `GetOIDCConfigMasked`, `SaveOIDCConfig`, `UpsertOIDCUser`, `GetOIDCUserBySub`, `GetOrCreateEncryptionKey`
- SaveOIDCConfig preserves existing secret when form sends empty (per D-23)
- GetOIDCConfigMasked strips ClientSecret for API responses (per T-14-01)
- GetOrCreateEncryptionKey generates 32-byte key on first call, persists in shelf (per D-16)
- Added 9 new tests covering all OIDC config and user CRUD operations

**Commit:** e95edf0

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

- `go test ./internal/session/ -count=1` -- PASS
- `go test ./internal/store/ -count=1` -- PASS
- `go test ./internal/... -count=1` -- all 4 packages PASS
- `go vet ./...` -- no issues

## Threat Model Compliance

- T-14-01 (Information Disclosure): GetOIDCConfigMasked implemented and tested -- strips ClientSecret before return
- T-14-02 (encryption_key in shelf): Accepted per plan -- single-user home app
- T-14-03 (SaveOIDCConfig tampering): Store methods ready; handler-level auth/CSRF protection deferred to Plan 03

## Self-Check: PASSED

- All 9 modified files exist on disk
- Commit ad7b028: FOUND (Task 1)
- Commit e95edf0: FOUND (Task 2)
- `go test ./internal/... -count=1`: all 4 packages PASS
- `go vet ./...`: clean
