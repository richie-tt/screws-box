# Screws Box

## What This Is

A web application for managing hardware organizer boxes. Presents a shelf as a configurable grid of containers (e.g., 5x10), supports adding items with tags/categories, and searching by name or tag with visual highlighting of positions on the grid. Features OIDC authentication, admin panel, data export/import, and optional Redis session persistence. Accessible on the home network.

## Core Value

Quickly find which container position (e.g., "3B") holds a part by typing its name or tag.

## Requirements

### Validated

- ✓ Network access (0.0.0.0, not localhost) — v1.0
- ✓ SQLite data storage — v1.0
- ✓ Configurable shelf grid (e.g., 5x10) with safe resize — v1.0
- ✓ Visual chessboard grid with labels (1A, 1B... 5J) — v1.0
- ✓ Add items to containers by clicking grid cells — v1.0
- ✓ Items have name and multiple tags — v1.0
- ✓ Containers hold multiple items — v1.0
- ✓ Text search by name or tag with grid highlighting — v1.0
- ✓ Multi-tag AND filtering, description search, batch queries — v1.1
- ✓ Session store interface with memory + Redis implementations — v1.1
- ✓ OIDC authentication with PKCE, local auth fallback — v1.1
- ✓ Admin panel (shelf settings, auth, OIDC config, export/import, sessions) — v1.1
- ✓ JSON data export/import with validation — v1.1
- ✓ Optional Redis session persistence via REDIS_URL — v1.1
- ✓ Comprehensive README with setup, config, deployment guides — v1.1

### Active

(None — ready for next milestone)

### Out of Scope

- Quantity tracking / inventory management — not needed, organizer doesn't track stock
- Multiple shelves — one shelf is sufficient
- Mobile app — responsive web is enough
- OAuth2 without OIDC — requires OIDC discovery endpoint
- GitHub as OIDC provider — not standard OIDC, use Authelia as proxy
- Multiple simultaneous OIDC providers — overkill for single-user app
- RBAC / multi-user roles — single admin user, auth is on/off
- FTS5 full-text search — LIKE queries sufficient for home organizer scale
- Redis as required dependency — must remain optional, single-binary + SQLite is core value

## Context

**Current state:** v1.1 shipped (2026-04-06). 8,772 LOC Go. 17 phases across 2 milestones.

**Tech stack:** Go 1.26+, chi v5, modernc.org/sqlite (CGo-free), go-oidc v3, go-redis v9. Custom CSS design system (DM Sans/DM Mono). html/template + htmx + vanilla JS.

**Architecture:** Single binary via go:embed. SQLite WAL mode. Pluggable session store (memory/Redis). OIDC with AES-GCM encrypted state cookies. Docker multi-stage build (scratch image with CA certs).

**Known tech debt:**
- Dead interface methods: `SearchItems`/`SearchItemsByTags` in StoreService (replaced by `SearchItemsBatch`)
- Human checkpoints pending: live OIDC E2E test, browser export/import round-trip

## Constraints

- **Stack**: Go backend, plain HTML frontend, SQLite database
- **Deployment**: Home network (0.0.0.0, not 127.0.0.1)
- **Simplicity**: Single Go binary + SQLite file, minimal configuration

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go + SQLite + plain HTML | Simple stack, single binary, zero dependencies | ✓ Good |
| Single shelf in v1 | Simplicity — user needs one | ✓ Good |
| Tags instead of hierarchical categories | Flexibility — items can have multiple tags | ✓ Good |
| Addressing: number+letter (3B) | Intuitive, like a chessboard / spreadsheet | ✓ Good |
| frontend-design plugin for UI | Better frontend design quality | ✓ Good |
| Custom CSS over Pico CSS | Pico caused specificity wars with custom styles (removed Phase 6) | ✓ Good |
| modernc.org/sqlite over mattn/go-sqlite3 | CGo-free, single-binary cross-compilation | ✓ Good |
| Session Store interface | Enabled Redis, admin session listing without auth rewrite | ✓ Good |
| OIDC with PKCE + AES-GCM state | Secure by default, no plaintext state in cookies | ✓ Good |
| Docker Compose profiles for Redis | Optional Redis without bloating default compose | ✓ Good |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-06 after v1.1 milestone complete*
