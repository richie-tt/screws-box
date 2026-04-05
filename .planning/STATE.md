---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: Search, Auth & Admin
status: defining_requirements
last_updated: "2026-04-05"
progress:
  total_phases: 0
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State: Screws Box

## Project Reference

**Core Value:** Szybkie znalezienie pozycji pojemnika (np. "3B") po wpisaniu nazwy lub tagu elementu.
**Current Milestone:** v1.1 — Search, Auth & Admin
**Current Focus:** Defining requirements

## Current Position

Phase: Not started (defining requirements)
Plan: —
Status: Defining requirements
Last activity: 2026-04-05 — Milestone v1.1 started

## Accumulated Context

### Key Decisions (Locked)

| Decision | Rationale |
|----------|-----------|
| Go + chi v5 + modernc.org/sqlite | Single binary, CGo-free, no C toolchain required |
| html/template + vanilla JS | No build step, no framework overhead |
| Pico CSS removed, custom design system | Specificity wars — custom app.css with design tokens |
| bcrypt for password hashing | SHA-256 too fast for passwords, bcrypt is industry standard |
| CSRF double-submit cookie pattern | Separate CSRF token from session, validated server-side |
| Rate limiting per-IP (API 5/s, login 0.5/s) | Brute-force protection without external dependency |
| Cookie Secure flag dynamic (isSecure) | Works on HTTP dev and HTTPS prod behind proxy |
| Store.conn unexported | No raw DB access outside store package |

### Open Questions

- OIDC library choice for Go — coreos/go-oidc vs zitadel/oidc
- Redis client library — go-redis/redis vs mediocregopher/radix
- Admin panel routing — separate chi group or sub-router

### Blockers

None.

---
*State initialized: 2026-04-05*
