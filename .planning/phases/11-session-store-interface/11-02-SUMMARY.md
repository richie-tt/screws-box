---
phase: 11-session-store-interface
plan: "02"
subsystem: server
tags: [session, refactor, dependency-injection]
dependency_graph:
  requires: ["11-01"]
  provides: ["Server struct with injected session.Manager", "SESSION_TTL env var support"]
  affects: ["internal/server/*", "cmd/screwsbox/main.go"]
tech_stack:
  added: []
  patterns: ["dependency injection via Server struct", "receiver methods replacing free functions"]
key_files:
  created: []
  modified:
    - internal/server/routes.go
    - internal/server/handler.go
    - internal/server/middleware.go
    - cmd/screwsbox/main.go
    - cmd/screwsbox/main_test.go
    - internal/server/handler_test.go
decisions:
  - "Server struct holds store + sessions as private fields"
  - "SESSION_TTL cleanup interval set to TTL/2"
  - "errRouter() test helper avoids leaking MemoryStore in error-path tests"
metrics:
  duration_seconds: 235
  completed: "2026-04-05T21:26:31Z"
  tasks_completed: 2
  tasks_total: 2
---

# Phase 11 Plan 02: Wire Session Package into Server Summary

Server handlers refactored from free functions with package-level session globals to Server receiver methods using injected session.Manager, with SESSION_TTL env var parsed in main.go.

## What Was Done

### Task 1: Create Server struct and wire session.Manager (5effc4c)

- Added `Server` struct to `routes.go` with `store StoreService` and `sessions *session.Manager` fields
- Added `NewServer(store, sessions)` constructor and `Router()` method
- Converted all 20+ handler functions from `func handleXxx(s StoreService) http.HandlerFunc` to `func (srv *Server) handleXxx() http.HandlerFunc`
- Removed all session globals from handler.go: `sessions sync.Map`, `cookieName`, `csrfCookieName`, `sessionData` struct
- Removed all free session functions: `generateToken`, `isSecure`, `createSession`, `destroySession`, `getSessionUser`, `getSessionCSRFToken`
- Replaced session calls: `createSession(w,r,username)` -> `srv.sessions.Create(w,r,username)` with error handling
- Converted `csrfProtect` middleware to Server method using `srv.sessions.GetCSRFToken(r)`
- Removed `sync` and `crypto/rand` and `encoding/hex` imports from handler.go (moved to session package)

### Task 2: Parse SESSION_TTL in main.go and update tests (e554b24)

- Added `parseSessionTTL()` function with 24h default, invalid value fallback, negative value rejection
- Created `MemoryStore` and `Manager` in `run()` with `defer memStore.Close()` for cleanup
- Replaced `server.NewRouter(&s)` with `server.NewServer(&s, sessionMgr)` + `appSrv.Router()`
- Updated `setupTestRouter` in handler_test.go to create session Manager
- Added `errRouter()` helper for error-path tests (14 tests using mock store)
- Added `testRouter(t, s)` helper in main_test.go
- All 40+ existing tests pass with zero regressions

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| Server struct with private fields | Encapsulates dependencies, prevents direct access from outside package |
| SESSION_TTL cleanup interval = TTL/2 | Reasonable sweep frequency without over-polling |
| errRouter() test helper | Avoids MemoryStore lifecycle leaks in error-path tests that don't use setupTestRouter |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed main_test.go references to server.NewRouter**
- **Found during:** Task 2
- **Issue:** main_test.go had 4 calls to `server.NewRouter(s)` which no longer exists
- **Fix:** Added `testRouter(t, s)` helper that creates session Manager, replaced all calls
- **Files modified:** cmd/screwsbox/main_test.go
- **Commit:** e554b24

**2. [Rule 3 - Blocking] Fixed handler_test.go error-path tests using NewRouter(errStore())**
- **Found during:** Task 2
- **Issue:** 14 error-path tests called `NewRouter(errStore())` directly
- **Fix:** Added `errRouter()` helper wrapping errStore with session Manager, replaced all calls
- **Files modified:** internal/server/handler_test.go
- **Commit:** e554b24

## Verification

```
go build ./...                     -- PASS
go test ./... -count=1 -race       -- PASS (all 5 packages)
grep "var sessions sync.Map"       -- not found (globals removed)
grep "func createSession"          -- not found (free functions removed)
grep "func getSessionUser"         -- not found
grep "func getSessionCSRFToken"    -- not found
grep "SESSION_TTL" main.go         -- found (env var handled)
```

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 5effc4c | Create Server struct, wire session.Manager into handlers and middleware |
| 2 | e554b24 | Parse SESSION_TTL in main.go, update all tests for Server struct |

## Self-Check: PASSED
