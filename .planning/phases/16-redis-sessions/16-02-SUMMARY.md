---
phase: 16-redis-sessions
plan: 02
subsystem: admin-ui
tags: [sessions, admin, revoke, session-table, csrf]

# Dependency graph
requires:
  - phase: 16-redis-sessions
    plan: 01
    provides: "Manager.ListSessions, DeleteSession, StoreType, TTL methods"
provides:
  - "Session API endpoints: GET/DELETE /api/sessions"
  - "Admin Sessions UI with table, revoke, badges, confirmation dialogs"
  - "SessionInfo response type (excludes CSRFToken)"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [session-api-handlers, safe-dom-construction, revoke-confirmation-modal]

key-files:
  created:
    - internal/server/handler_session.go
    - internal/server/handler_session_test.go
  modified:
    - internal/server/handler.go
    - internal/server/routes.go
    - internal/server/templates/admin.html
    - internal/server/static/css/admin.css
    - internal/server/static/js/admin.js
    - internal/server/handler_test.go

key-decisions:
  - "SessionInfo excludes CSRFToken to prevent information disclosure (T-16-06)"
  - "Session ID validated as 64 hex chars before store operations (T-16-07)"
  - "Reused resize-modal-overlay/dialog CSS pattern for revoke confirmation modal"

patterns-established:
  - "Session API handlers follow existing handler pattern with writeJSON responses"
  - "Safe DOM construction in admin.js (createElement/textContent, no innerHTML)"

requirements-completed: [ADMN-06]

# Metrics
duration: 5min
completed: 2026-04-06
---

# Phase 16 Plan 02: Admin Session Management UI Summary

**Session API endpoints (list/revoke/bulk-revoke) with full admin UI: session table, store indicator, confirmation dialogs, and sidebar badge**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-06T12:57:03Z
- **Completed:** 2026-04-06T13:01:40Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Created session API handlers: GET /api/sessions (list with is_current marking), DELETE /api/sessions/{id} (revoke single, 403 on own), DELETE /api/sessions (bulk revoke all except own)
- SessionInfo response type intentionally excludes CSRFToken (T-16-06 mitigation)
- Session ID format validation: 64 hex characters required (T-16-07 mitigation)
- Admin template updated: Sessions nav with count badge, session table with 6 columns, store indicator pill, refresh button, empty state
- "Your session" badge on current session row, revoke button disabled for own session (D-18 / T-16-05)
- Revoke confirmation modal with single and bulk modes, focus trap, Escape to close
- Row removal animation on successful revoke
- Responsive CSS: Expires column hidden on small screens, table scrollable on mobile
- AdminData extended with SessionStoreType, SessionCount, Sessions, CurrentSessionID, SessionTTL fields

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Failing tests for session API** - `e2e0c6f` (test)
2. **Task 1 GREEN: Session API handlers and routes** - `d254721` (feat)
3. **Task 2: Admin sessions UI (template, CSS, JS)** - `0528df4` (feat)

## Files Created/Modified
- `internal/server/handler_session.go` - SessionInfo type, handleListSessions, handleRevokeSession, handleRevokeAllOthers, formatExpiresIn, session ID validation
- `internal/server/handler_session_test.go` - TestHandleListSessions, TestHandleRevokeSession, TestHandleRevokeOwnSession, TestHandleRevokeAllOthers
- `internal/server/handler.go` - AdminData extended with 5 session fields, handleAdmin populates session data
- `internal/server/routes.go` - Added GET/DELETE /api/sessions and DELETE /api/sessions/{sessionID} routes
- `internal/server/templates/admin.html` - Sessions section with table, badges, modals; nav badge replaces "coming soon"
- `internal/server/static/css/admin.css` - Sessions table, badge, pill, empty state, responsive styles
- `internal/server/static/js/admin.js` - refreshSessions, renderSessionsTable, showRevokeModal, updateSessionBadge, event delegation
- `internal/server/handler_test.go` - Updated TestHandleAdminPage to expect sessions nav instead of "coming soon"

## Decisions Made
- SessionInfo response type excludes CSRFToken -- admin-only endpoint but defense in depth
- Session ID validated as exactly 64 hex chars before passing to store -- prevents injection
- Reused existing resize-modal-overlay/dialog CSS classes for revoke confirmation modal -- consistent UI
- All DOM construction in admin.js uses safe methods (createElement, textContent) -- no innerHTML

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated TestHandleAdminPage assertion**
- **Found during:** Task 2
- **Issue:** Existing test asserted "coming soon" text which was intentionally removed per plan
- **Fix:** Changed assertion to check for "Sessions" nav item and "nav-badge" class
- **Files modified:** internal/server/handler_test.go
- **Commit:** 0528df4

---

**Total deviations:** 1 auto-fixed (existing test needed update for removed "coming soon" text)
**Impact on plan:** Expected -- replacing disabled nav item with active sessions section.

## Threat Mitigations Applied
- T-16-04: Session endpoints behind existing authMiddleware + csrfProtect
- T-16-05: Server-side self-revocation check returns 403; UI hides revoke on own session
- T-16-06: SessionInfo excludes CSRFToken field
- T-16-07: sessionIDPattern validates 64 hex chars before store operations

## Self-Check: PASSED
All 7 key files verified on disk. All 3 task commits verified in git log.
