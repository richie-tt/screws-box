# Milestones

## v1.1 Search, Auth & Admin (Shipped: 2026-04-06)

**Phases completed:** 7 phases, 16 plans, 23 tasks

**Key accomplishments:**

- 1. [Rule 2 - Enhancement] NewMemoryStore takes cleanupInterval parameter
- 1. [Rule 3 - Blocking] Fixed main_test.go references to server.NewRouter
- GROUP_CONCAT batch search replacing N+1 queries with matched_on metadata, description search, multi-tag AND filtering, and 50-item cap with total_count
- Unified search dropdown with dual-section tags/items, filter chip management, URL state sync, mark highlights, and description display implementing all 26 user decisions (D-01 through D-26)
- 1. [Rule 2 - Security] Used safe DOM methods instead of innerHTML for resize modal
- Grid page stripped of all settings UI — gear icon replaced with Admin text link, ~690 lines of settings JS/CSS removed, bidirectional navigation confirmed
- One-liner:
- 1. [Rule 3 - Blocking] Updated mockStore in handler_test.go
- RedisStore implementation with go-redis/v9, extended Store interface (List/Close), REDIS_URL env var wiring, and Manager admin methods
- Session API endpoints (list/revoke/bulk-revoke) with full admin UI: session table, store indicator, confirmation dialogs, and sidebar badge
- README.md with 7 sections (Quick Start through Troubleshooting), .env.example, docker-compose.yml with Redis profile, and Dockerfile CA cert fix for OIDC

---
