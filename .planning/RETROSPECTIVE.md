# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.1 — Search, Auth & Admin

**Shipped:** 2026-04-06
**Phases:** 7 | **Plans:** 16

### What Was Built
- Pluggable session store (memory + Redis) with configurable TTL and sliding expiry
- Enhanced search: batch GROUP_CONCAT queries, multi-tag AND filtering, description search, unified dropdown UI
- Dedicated admin panel: shelf settings, auth config, OIDC setup, data export/import, session management
- OIDC authentication with PKCE/state/nonce, AES-GCM encrypted state cookies, local auth fallback
- JSON data export/import with two-step validate/confirm flow
- Optional Redis session persistence with active session listing and revocation
- Comprehensive README.md, Dockerfile, docker-compose.yml, .env.example

### What Worked
- Session Store interface design (Phase 11) paid off across OIDC (Phase 14), Redis (Phase 16), and admin sessions (Phase 16) — clean plug-in without auth rewrites
- Phasing OIDC into 4 plans (models → package → flow → admin UI) kept each plan focused and testable
- Admin panel shell (Phase 13) as early foundation let subsequent phases (14, 15, 16) just add sections
- Cross-phase integration was clean — 18/18 wiring checks passed on first audit

### What Was Inefficient
- Phase 16 ROADMAP.md progress got out of sync (showed "0/2 Planning complete" when actually complete) — manual state tracking drifts
- Screenshot placeholder handling required two passes (worktree agent claimed replacement but didn't merge properly)
- Nyquist VALIDATION.md files created for all phases but none signed off as compliant — overhead without payoff for a documentation-heavy milestone
- Several SUMMARY.md files lack `requirements_completed` frontmatter — makes audit cross-referencing harder

### Patterns Established
- Docker Compose profiles for optional services (Redis) — clean pattern for future optional deps
- AES-GCM encrypted cookies for OIDC state — reusable for any auth flow needing tamper-proof client state
- Admin panel section pattern: handler populates AdminData struct, template renders section, JS handles form submission — consistent across all admin features
- Two-step validate/confirm for destructive operations (import) — prevents accidental data loss

### Key Lessons
1. Worktree merges need spot-checking — agent may claim changes were made but worktree isolation can cause them to not land on main
2. Session management should be extracted as an interface from day 1 in any auth-using app — retrofitting is tractable but front-loading is cheaper
3. For documentation phases, the "human review" checkpoint is essential — automated verification catches structure but not accuracy of content like OIDC walkthroughs

### Cost Observations
- Model mix: ~80% opus (execution, planning), ~20% sonnet (verification, checking)
- v1.1 completed in 2 days (2026-04-05 to 2026-04-06)
- Most complex phase: OIDC (4 plans, 4 waves) — justified by security surface

---

## Cross-Milestone Trends

| Metric | v1.0 | v1.1 |
|--------|------|------|
| Phases | 10 | 7 |
| Plans | 14 | 16 |
| Duration | 2 days | 2 days |
| LOC (cumulative) | ~4,000 | 8,772 |
| Integration issues | 0 | 0 |
| Requirements covered | 14/14 | 18/18 |

**Observations:**
- v1.1 had more plans per phase (2.3 avg vs 1.4) — features are more complex as the app matures
- Zero integration issues across both milestones — phase dependency planning is working
- LOC doubled but complexity grew more than linearly (auth, OIDC, Redis)
