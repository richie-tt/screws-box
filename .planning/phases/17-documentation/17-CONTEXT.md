# Phase 17: Documentation - Context

**Gathered:** 2026-04-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Create a comprehensive README.md and supporting deployment files (Dockerfile, docker-compose.yml, .env.example) so a new user can set up, configure, and use Screws Box without reading source code. Also include a brief development section for contributors.

</domain>

<decisions>
## Implementation Decisions

### Structure & Depth
- **D-01:** Single README.md file with all documentation (no separate docs/ folder)
- **D-02:** Linked table of contents at top for navigation
- **D-03:** Setup-first section ordering: Overview -> Quick Start -> Configuration -> Usage Guide -> Admin -> Development -> Troubleshooting/FAQ
- **D-04:** Thorough usage guide with step-by-step walkthroughs for each feature (grid, search, admin panel, OIDC setup, export/import)
- **D-05:** Brief FAQ/Troubleshooting section covering common pitfalls (port conflicts, SQLite permissions, OIDC callback URL, Redis connection)
- **D-06:** Screenshots + ASCII diagrams for grid UI (screenshots of actual UI, ASCII for coordinate system explanation)

### Audience & Tone
- **D-07:** English only (consistent with UI text convention)
- **D-08:** Primary audience: self-hosting user setting up on home network. Light dev section at end.
- **D-09:** Friendly practical tone — warm but concise, no corporate jargon
- **D-10:** Neutral/professional framing — present as a general-purpose tool, no mention of personal project

### Environment Variables
- **D-11:** Markdown table (name, required?, default, description) PLUS a committed .env.example file
- **D-12:** Env vars grouped by feature: Core (PORT, DB_PATH), Sessions (SESSION_TTL, REDIS_URL), OIDC (OIDC_ISSUER, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, OIDC_DISPLAY_NAME)

### Deployment
- **D-13:** Document two deployment methods: binary + systemd AND Docker + compose
- **D-14:** Commit Dockerfile (multi-stage) and docker-compose.yml to repo root
- **D-15:** docker-compose.yml uses Docker Compose profiles — Redis as optional profile (`docker compose --profile redis up`)
- **D-16:** OIDC setup documented with Authelia-specific walkthrough (most likely home server provider) plus generic OIDC section

### Claude's Discretion
- Exact wording of FAQ entries based on observed pitfalls in prior phases
- Screenshot selection and placement within the README
- Systemd unit file details (user service vs system, restart policy)
- Development section depth (build, test, project structure overview)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Context
- `.planning/PROJECT.md` — Core value, constraints, current milestone scope
- `.planning/REQUIREMENTS.md` — DOCS-01 requirement definition
- `CLAUDE.md` — Technology stack, conventions, architecture overview

### Code References (for accurate documentation)
- `cmd/screwsbox/main.go` — All env var parsing (DB_PATH, SESSION_TTL, REDIS_URL, PORT, OIDC_*)
- `internal/session/redis.go` — Redis connection behavior (PING on startup, fail-fast)
- `internal/server/routes.go` — All HTTP routes for API reference
- `go.mod` — Dependencies list for build requirements

### Prior Phase Summaries (feature documentation sources)
- `.planning/phases/16-redis-sessions/16-01-SUMMARY.md` — Redis session store behavior
- `.planning/phases/14-oidc-authentication/` — OIDC flow details
- `.planning/phases/15-data-export-import/` — Export/import format and flow
- `.planning/phases/13-admin-panel-shell/` — Admin panel structure

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- No existing README.md — creating from scratch
- CLAUDE.md contains architecture overview that can inform the development section
- go.mod lists exact dependency versions for build docs

### Established Patterns
- All env vars parsed in cmd/screwsbox/main.go with os.Getenv
- Dev mode via `go run -tags dev .` (live reload from disk)
- Single binary via `go build` with go:embed for assets
- WAL mode SQLite with foreign keys

### Integration Points
- README.md at repo root
- Dockerfile at repo root (new file)
- docker-compose.yml at repo root (new file)
- .env.example at repo root (new file)

</code_context>

<specifics>
## Specific Ideas

- Authelia as the primary OIDC provider example (home server context)
- Docker Compose profiles pattern for optional Redis
- Grid coordinate system (columns = numbers, rows = letters, e.g., "3B") needs ASCII diagram
- Screenshots should show: grid view, search with highlights, admin panel

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 17-documentation*
*Context gathered: 2026-04-06*
