---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: Search, Auth & Admin
status: ready_to_plan
last_updated: "2026-04-05"
progress:
  total_phases: 17
  completed_phases: 10
  total_plans: 14
  completed_plans: 14
  percent: 59
---

# Project State: Screws Box

## Project Reference

**Core Value:** Szybkie znalezienie pozycji pojemnika (np. "3B") po wpisaniu nazwy lub tagu elementu.
**Current Milestone:** v1.1 — Search, Auth & Admin
**Current Focus:** Phase 11 - Session Store Interface

## Current Position

Phase: 11 of 17 (Session Store Interface)
Plan: 0 of 0 in current phase (not yet planned)
Status: Ready to plan
Last activity: 2026-04-05 — v1.1 roadmap created (7 phases, 18 requirements mapped)

Progress: [##########..............] 59%

## Accumulated Context

### Key Decisions (Locked)

| Decision | Rationale |
|----------|-----------|
| Go + chi v5 + modernc.org/sqlite | Single binary, CGo-free, no C toolchain required |
| html/template + vanilla JS | No build step, no framework overhead |
| Custom design system (Pico removed) | Specificity wars — custom app.css with design tokens |
| bcrypt for password hashing | SHA-256 too fast for passwords, bcrypt is industry standard |
| CSRF double-submit cookie pattern | Separate CSRF token from session, validated server-side |
| coreos/go-oidc/v3 for OIDC | De facto Go OIDC client, 1800+ importers |
| redis/go-redis/v9 for Redis | Official Redis Go client, 17000+ importers |
| Session store interface first | Load-bearing refactor: OIDC, Redis, admin sessions all depend on it |

### Open Questions

- OIDC state/nonce storage strategy (cookie vs server-side) — decide in Phase 14
- Admin panel layout (tabs vs sections) — decide in Phase 13

### Blockers

None.

---
*State initialized: 2026-04-05*
*Roadmap created: 2026-04-05*
