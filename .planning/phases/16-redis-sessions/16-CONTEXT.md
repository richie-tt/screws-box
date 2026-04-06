# Phase 16: Redis Sessions - Context

**Gathered:** 2026-04-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Sessions can optionally persist in Redis for restart survival, and the admin can see and revoke active sessions. Setting REDIS_URL activates Redis; without it, in-memory sessions continue working. The admin panel's Sessions section becomes active with a table of active sessions and revocation controls.

</domain>

<decisions>
## Implementation Decisions

### Store Interface Extension
- **D-01:** Add `List(ctx context.Context) ([]*Session, error)` method to the Store interface. Returns all active (non-expired) sessions as full Session structs.
- **D-02:** Revocation uses the existing `Delete(ctx, id)` method. No separate Revoke method — Delete already removes a session.
- **D-03:** Add `Close() error` to the Store interface. MemoryStore already has Close(); RedisStore will close its redis.Client. Enables clean shutdown.
- **D-04:** Both MemoryStore and RedisStore implement List. Admin sessions UI works regardless of backend. Easier testing.

### Redis Connection & Fallback
- **D-05:** Redis activated via `REDIS_URL` env var. Without it, app uses MemoryStore with no error or degraded behavior.
- **D-06:** On startup with REDIS_URL set, PING Redis. If unreachable, app fails to start with clear error message. No silent degradation.
- **D-07:** If Redis becomes unavailable while running, session operations return errors. Login/auth fails until Redis recovers. No auto-fallback to memory — avoids data inconsistency.

### Redis Data Model
- **D-08:** Key pattern: `session:{id}`, value: JSON-serialized Session struct. Use Redis EXPIRE for TTL management.
- **D-09:** Use `redis/go-redis/v9` client (already decided in STATE.md key decisions).

### Admin Sessions UI
- **D-10:** Table layout with columns: User, Auth Method, Created, Last Active, Expires In, Actions. Consistent with admin card style.
- **D-11:** Each row shows: username, auth method (local/OIDC), created timestamp, last activity timestamp, relative expiry ("expires in 22h"), and a Revoke button.
- **D-12:** Admin's own session row has a "Your session" badge. Revoke button disabled/hidden on own session to prevent accidental lockout.
- **D-13:** Session count badge in nav sidebar: "Sessions (5)". Quick glance without clicking into section.
- **D-14:** Session list loads statically on page navigation. Manual "Refresh" button to re-fetch. No auto-polling.
- **D-15:** Empty state shows "No active sessions" message with note about which store backend is in use (Memory / Redis).

### Revocation UX
- **D-16:** Revoke requires confirmation dialog: "Revoke session for {username}?" with confirm/cancel. Consistent with resize confirmation pattern.
- **D-17:** "Revoke All Others" button clears all sessions except admin's current one. Useful for security incidents. Also requires confirmation.
- **D-18:** Admin cannot revoke their own current session — Revoke button is disabled/hidden on own session row.

### Claude's Discretion
- Redis connection pool settings (pool size, timeouts)
- RedisStore internal implementation details (SCAN vs SET for List)
- Session table sorting (by last activity, creation time)
- Exact confirm dialog styling
- Refresh button placement and icon

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Session store interface (Phase 11 foundation)
- `internal/session/store.go` -- Current Store interface (Create/Get/Delete/Touch/DeleteByAuthMethod)
- `internal/session/session.go` -- Session struct (ID, Username, AuthMethod, DisplayName, CSRFToken, CreatedAt, LastActivity)
- `internal/session/memory.go` -- MemoryStore implementation with background cleanup
- `internal/session/manager.go` -- Manager wrapping Store + cookie handling

### Admin panel (Phase 13 foundation)
- `internal/server/templates/admin.html` -- Admin template with disabled "Sessions coming soon" placeholder
- `internal/server/static/js/admin.js` -- Admin page JavaScript (section navigation, forms)
- `internal/server/static/css/admin.css` -- Admin page styles (if separate) or grid.css admin section

### Server wiring
- `internal/server/routes.go` -- Route structure, Server struct, middleware chain
- `main.go` -- Entry point, env var parsing (REDIS_URL to be added here)

### Requirements
- `.planning/REQUIREMENTS.md` -- SESS-02 (Redis via REDIS_URL), ADMN-06 (active session list with revoke)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `session.Store` interface -- extend with List and Close methods
- `session.MemoryStore` -- reference implementation, add List method iterating over map
- `session.Manager` -- already injected into Server, handles cookie lifecycle
- Admin page section pattern -- nav items, card sections, JS section switching

### Established Patterns
- Env var config: `SESSION_TTL` (Go duration), same pattern for `REDIS_URL`
- `context.Context` on all Store methods -- ready for Redis network calls
- Background cleanup goroutine in MemoryStore -- RedisStore uses Redis EXPIRE instead
- Admin JS IIFE pattern with section-specific initialization

### Integration Points
- `internal/session/store.go` -- Add List and Close to interface
- `internal/session/memory.go` -- Implement List (iterate map), update Close signature
- New `internal/session/redis.go` -- RedisStore implementing full Store interface
- `main.go` -- Parse REDIS_URL, create RedisStore or MemoryStore accordingly
- `internal/server/routes.go` -- New admin API endpoints: GET /api/sessions, DELETE /api/sessions/{id}, DELETE /api/sessions (bulk)
- `internal/server/handler.go` -- New handlers for session list, revoke, bulk revoke
- `internal/server/templates/admin.html` -- Replace disabled Sessions placeholder with active section

</code_context>

<specifics>
## Specific Ideas

No specific requirements -- open to standard approaches.

</specifics>

<deferred>
## Deferred Ideas

- **Docked container panel** -- Container detail panel fixed on left side under search bar instead of floating over grid. This is a UI layout change unrelated to sessions -- belongs in its own phase or backlog item.

</deferred>

---

*Phase: 16-redis-sessions*
*Context gathered: 2026-04-06*
