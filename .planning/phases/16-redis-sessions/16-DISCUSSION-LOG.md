# Phase 16: Redis Sessions - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md -- this log preserves the alternatives considered.

**Date:** 2026-04-06
**Phase:** 16-redis-sessions
**Areas discussed:** Store interface extension, Redis connection & fallback, Admin sessions UI, Revocation UX

---

## Store Interface Extension

| Option | Description | Selected |
|--------|-------------|----------|
| List + Delete | Add List(ctx) returning all active sessions. Revocation uses existing Delete(id). Minimal extension. | ✓ |
| List + Revoke + Count | Add List, Revoke (separate from Delete), and Count. More semantic but Delete already works. | |
| ListByUser + DeleteAll | Add ListByUser(ctx, username) and DeleteAll(ctx). Filtering by user. | |

**User's choice:** List + Delete (minimal extension)
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Both implement listing | MemoryStore and RedisStore both implement List. Works regardless of backend. | ✓ |
| RedisStore only | Listing only available when Redis is active. | |

**User's choice:** Both implement listing

| Option | Description | Selected |
|--------|-------------|----------|
| Full Session structs | List returns []*Session with all fields. Simple, reuses existing struct. | ✓ |
| Lightweight SessionInfo | New struct with only display-relevant fields (no CSRFToken). | |
| Map of ID→Session | Returns map[string]*Session for O(1) lookup. | |

**User's choice:** Full Session structs

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, add Close() to Store interface | MemoryStore already has Close(). Making it part of the interface ensures RedisStore cleanup too. | ✓ |
| Keep Close() off interface | MemoryStore has Close() but not on interface. RedisStore manages lifecycle separately. | |

**User's choice:** Add Close() to Store interface

---

## Redis Connection & Fallback

| Option | Description | Selected |
|--------|-------------|----------|
| Fail operations | Redis down = session ops return errors. No data inconsistency. | ✓ |
| Auto-fallback to memory | Detect failure, switch to MemoryStore. Complex dual-store logic. | |
| Retry with timeout | Each operation retries 2-3 times with backoff. | |

**User's choice:** Fail operations

| Option | Description | Selected |
|--------|-------------|----------|
| PING on startup, fail if unreachable | App won't start if REDIS_URL set but Redis unreachable. | ✓ |
| PING with warning, fall back to memory | Log warning, start with MemoryStore. | |
| No startup check | Lazy connection on first operation. | |

**User's choice:** PING on startup, fail if unreachable

| Option | Description | Selected |
|--------|-------------|----------|
| JSON value + Redis TTL | Key: 'session:{id}', JSON value, Redis EXPIRE. Simple, inspectable. | ✓ |
| Hash per session | Redis HASH with field per Session field. More Redis-native. | |
| JSON + secondary index | JSON value + SET 'sessions:all' tracking all IDs. | |

**User's choice:** JSON value + Redis TTL

---

## Admin Sessions UI

**Session row info (multiSelect):**
All 4 options selected:
- ✓ Username + auth method
- ✓ Created + last activity
- ✓ Session age / expires in
- ✓ Current session badge

| Option | Description | Selected |
|--------|-------------|----------|
| Static on page load | Load on navigation, manual refresh button. | ✓ |
| Auto-refresh every 30s | Poll periodically with htmx. | |
| Manual refresh only | Explicit refresh button, no auto-update. | |

**User's choice:** Static on page load

| Option | Description | Selected |
|--------|-------------|----------|
| Table rows | HTML table with columns. Consistent with admin card style. | ✓ |
| Card per session | Each session as a card block. | |
| Compact list | Dense list, expand on click. | |

**User's choice:** Table rows

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, show count badge | Sessions nav shows 'Sessions (5)'. | ✓ |
| No badge | Count visible inside section only. | |

**User's choice:** Show count badge

| Option | Description | Selected |
|--------|-------------|----------|
| Message + explanation | 'No active sessions' with store backend note. | ✓ |
| Just empty table | Table headers with no rows. | |

**User's choice:** Message + explanation

---

## Revocation UX

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, confirm dialog | Click Revoke → confirm/cancel dialog. Consistent with resize pattern. | ✓ |
| Instant revoke | Click → deleted immediately with toast. | |
| Undo-based | Revoke immediately, show Undo for 5 seconds. | |

**User's choice:** Confirm dialog

| Option | Description | Selected |
|--------|-------------|----------|
| Prevent self-revocation | Revoke button disabled on own session. Badge marks it. | ✓ |
| Allow with warning | Active button with extra warning. | |
| Allow without warning | No special treatment. | |

**User's choice:** Prevent self-revocation

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, revoke all except mine | 'Revoke All Others' button. Useful for security incidents. | ✓ |
| No bulk revoke | Only individual session revocation. | |

**User's choice:** Revoke all except mine

---

## Claude's Discretion

- Redis connection pool settings (pool size, timeouts)
- RedisStore internal implementation (SCAN vs SET for List)
- Session table sorting
- Confirm dialog styling
- Refresh button placement

## Deferred Ideas

- Docked container panel (left side under search bar instead of floating) -- separate phase/backlog
