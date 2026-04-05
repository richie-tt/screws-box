---
phase: 11-session-store-interface
plan: 01
subsystem: session
tags: [session, store, interface, memory, manager, cookies]
dependency_graph:
  requires: []
  provides: [session-store-interface, session-manager, memory-store]
  affects: [internal/server/handler.go]
tech_stack:
  added: []
  patterns: [Store interface with context.Context, RWMutex concurrency, background cleanup goroutine, sliding window expiry]
key_files:
  created:
    - internal/session/session.go
    - internal/session/store.go
    - internal/session/memory.go
    - internal/session/manager.go
    - internal/session/memory_test.go
    - internal/session/manager_test.go
  modified: []
decisions:
  - "NewMemoryStore takes both ttl and cleanupInterval params for testability"
  - "Manager.Create detects Secure via TLS or X-Forwarded-Proto header"
  - "Destroy clears 4 cookies (session+csrf x secure+insecure) to handle stale Secure cookies"
metrics:
  duration: 2min
  completed: "2026-04-05T21:18:46Z"
  tasks_completed: 2
  tasks_total: 2
  files_created: 6
  files_modified: 0
---

# Phase 11 Plan 01: Session Store Interface Summary

Pluggable session infrastructure with Store interface, MemoryStore, and Manager cookie wrapper -- 17 tests passing with race detector.

## Task Results

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Store interface + Session struct + MemoryStore | b5b6ff7 | session.go, store.go, memory.go, memory_test.go |
| 2 | Manager wrapping Store with cookie logic | cbc4449 | manager.go, manager_test.go |

## What Was Built

### Store Interface (store.go)
Minimal 4-method interface: Create, Get, Delete, Touch -- all taking context.Context as first parameter for future Redis compatibility.

### Session Struct (session.go)
Exported struct with ID, Username, CSRFToken, CreatedAt, LastActivity fields.

### MemoryStore (memory.go)
In-memory implementation using sync.RWMutex with:
- Lazy expiry on Get (returns nil for expired sessions and deletes them)
- Background cleanup goroutine with configurable interval
- Sliding window via Touch updating LastActivity
- Close() to stop background goroutine cleanly

### Manager (manager.go)
Cookie lifecycle wrapper around Store:
- Create: generates 256-bit session+CSRF tokens, sets HttpOnly session cookie and JS-readable CSRF cookie
- Destroy: deletes from store, clears cookies with both Secure=true and Secure=false variants
- GetUser: reads cookie, fetches session, calls Touch for sliding window, returns username
- GetCSRFToken: reads cookie, fetches session, returns server-side CSRF token

## Deviations from Plan

### Minor Adjustments

**1. [Rule 2 - Enhancement] NewMemoryStore takes cleanupInterval parameter**
- **Issue:** Plan showed cleanup hardcoded to 5 minutes, but tests need short intervals
- **Fix:** Added cleanupInterval as second parameter to NewMemoryStore for testability
- **Impact:** None -- callers pass their desired interval

## Test Coverage

- 9 MemoryStore tests: Create, Get_NotFound, Delete, Touch, Expiry, SlidingWindow, BackgroundCleanup, Close, ConcurrentAccess
- 8 Manager tests: Create, Destroy, GetUser, GetUser_NoSession, GetUser_ExpiredSession, GetCSRFToken, GetCSRFToken_NoSession, GetUser_TouchesSession
- All 17 tests pass with `-race` flag

## Threat Surface Review

No new threat flags. All threat mitigations from the plan are implemented:
- T-11-01: Session IDs use crypto/rand (32 bytes / 256 bits), verified by test checking 64 hex chars
- T-11-02: Unknown IDs return nil with no information leakage; HttpOnly + SameSiteLax on cookies
- T-11-03: Background cleanup + lazy expiry prevent unbounded memory growth
- T-11-04: CSRF token stored server-side, separate from session ID
- T-11-05: Accepted -- in-memory store loses sessions on restart

## Known Stubs

None -- all functionality is fully wired.

## Self-Check: PASSED

All 6 files confirmed on disk. Both commit hashes (b5b6ff7, cbc4449) verified in git log.
