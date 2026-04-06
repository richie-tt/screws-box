# Roadmap: Screws Box

## Milestones

- ✅ **v1.0 MVP** - Phases 1-10 (shipped 2026-04-03)
- 🚧 **v1.1 Search, Auth & Admin** - Phases 11-17 (in progress)

## Phases

<details>
<summary>v1.0 MVP (Phases 1-10) - SHIPPED 2026-04-03</summary>

### Phase 1: Project Skeleton
**Goal**: A deployable Go binary exists that serves HTTP on the local network with no feature content yet
**Depends on**: Nothing
**Requirements**: INFR-02, INFR-03
**Success Criteria** (what must be TRUE):
  1. Running `go run .` (or the built binary) starts a server listening on `0.0.0.0:8080` reachable from another device on the LAN
  2. A request to `/` returns a valid HTTP 200 response (can be a placeholder page)
  3. Sending SIGTERM or Ctrl-C shuts the server down gracefully (no hung process)
  4. `go build` produces a single binary with no external file dependencies (assets embedded via `go:embed`)
  5. `-tags dev` build reads templates/static files from disk; production build serves from embedded FS
**Plans:** 1/1 plans complete
Plans:
- [x] 01-01-PLAN.md

### Phase 2: Database Foundation
**Goal**: The application opens SQLite with all correctness pragmas set and the full normalized schema in place before any feature code runs
**Depends on**: Phase 1
**Requirements**: INFR-01
**Plans:** 1/1 plans complete
Plans:
- [x] 02-01-PLAN.md

### Phase 3: Grid UI Design
**Goal**: A static HTML mockup defines the visual design for the chessboard grid, container cells, and search bar
**Depends on**: Phase 1
**Requirements**: GRID-02, GRID-04
**Plans:** 1/1 plans complete
Plans:
- [x] 03-01-PLAN.md

### Phase 4: Grid Rendering
**Goal**: The server renders the real configurable grid with correct labels and live item counts from the database
**Depends on**: Phase 2, Phase 3
**Requirements**: GRID-01, GRID-03
**Plans:** 1/1 plans complete
Plans:
- [x] 04-01-PLAN.md

### Phase 5: Item CRUD Backend
**Goal**: All item and tag data operations are available as JSON API endpoints backed by the Store layer
**Depends on**: Phase 2
**Requirements**: ITEM-02, ITEM-04, ITEM-05
**Plans:** 2/2 plans complete
Plans:
- [x] 05-01-PLAN.md
- [x] 05-02-PLAN.md

### Phase 6: Item CRUD Frontend
**Goal**: A user can add, edit, and delete items in any container using only the browser
**Depends on**: Phase 4, Phase 5
**Requirements**: ITEM-01, ITEM-03
**Plans:** 2/2 plans complete
Plans:
- [x] 06-01-PLAN.md
- [x] 06-02-PLAN.md

### Phase 7: Tag Autocomplete
**Goal**: When typing tags, users see suggestions drawn from existing tags in the database
**Depends on**: Phase 5, Phase 6
**Requirements**: ITEM-06
**Plans:** 1/1 plans complete
Plans:
- [x] 07-01-PLAN.md

### Phase 8: Search Backend
**Goal**: A search endpoint returns all items and their container positions matching a query
**Depends on**: Phase 5
**Requirements**: SRCH-02, SRCH-03
**Plans:** 1/1 plans complete
Plans:
- [x] 08-01-PLAN.md

### Phase 9: Search Frontend
**Goal**: Users can find any part by typing its name or tag and see which containers hold it
**Depends on**: Phase 6, Phase 8
**Requirements**: SRCH-01, SRCH-04, SRCH-05, SRCH-06
**Plans:** 2/2 plans complete
Plans:
- [x] 09-01-PLAN.md
- [x] 09-02-PLAN.md

### Phase 10: Resilience
**Goal**: The grid can be safely resized without orphaning items
**Depends on**: Phase 6
**Requirements**: GRID-05
**Plans:** 2/2 plans complete
Plans:
- [x] 10-01-PLAN.md
- [x] 10-02-PLAN.md

</details>

### v1.1 Search, Auth & Admin (In Progress)

**Milestone Goal:** Multi-tag search filtering, OIDC authentication, dedicated admin panel, optional Redis session persistence, project documentation.

- [x] **Phase 11: Session Store Interface** - Extract session management into pluggable interface with memory implementation and configurable TTL (completed 2026-04-05)
- [x] **Phase 12: Search Enhancement** - Multi-tag AND filtering, description search, batch query optimization (completed 2026-04-05)
- [x] **Phase 13: Admin Panel Shell** - Dedicated /admin page with shelf settings migrated from grid modal (completed 2026-04-06)
- [x] **Phase 14: OIDC Authentication** - Login via OIDC provider with PKCE, local auth fallback, admin config UI (completed 2026-04-06)
- [x] **Phase 15: Data Export/Import** - JSON export download and validated import upload in admin panel (completed 2026-04-06)
- [ ] **Phase 16: Redis Sessions** - Optional Redis-backed sessions with active session listing and revocation
- [ ] **Phase 17: Documentation** - README.md with developer setup, user guide, configuration reference

## Phase Details

### Phase 11: Session Store Interface
**Goal**: Session management is abstracted behind a clean interface so that OIDC, Redis, and admin session listing can plug in without rewriting auth plumbing
**Depends on**: Nothing (v1.0 complete)
**Requirements**: SESS-01, SESS-03
**Success Criteria** (what must be TRUE):
  1. Existing login/logout/CSRF behavior works identically after refactor -- no regressions visible to the user
  2. Sessions expire after a configurable TTL (default 24h) -- a user who logs in and waits beyond TTL must log in again
  3. Session expiry uses sliding window -- active users are not kicked out mid-use
  4. The session store is injected via interface, not hardcoded -- visible in code structure (`internal/session/` package with `Store` interface)
**Plans:** 2/2 plans complete
Plans:
- [x] 11-01-PLAN.md — Create session package (Store interface, Session struct, MemoryStore, Manager)
- [x] 11-02-PLAN.md — Wire session package into server and parse SESSION_TTL

### Phase 12: Search Enhancement
**Goal**: Users can narrow search results by combining multiple tag filters with the text search, and search also covers item descriptions
**Depends on**: Nothing (v1.0 complete, independent of Phase 11)
**Requirements**: SRCH-01, SRCH-02, SRCH-03, SRCH-04
**Success Criteria** (what must be TRUE):
  1. User can select multiple tags from a filter bar with autocomplete; results show only items matching ALL selected tags (AND logic)
  2. Main search matches against item description field in addition to item name
  3. When tag filters are active, the main search text box searches only name and description (not tags) -- tags are handled exclusively by the filter bar
  4. Search results load without N+1 queries -- a single search with 50+ items returns in under 200ms (batch SQL)
**Plans:** 2/2 plans complete
Plans:
- [x] 12-01-PLAN.md — Batch search backend (SearchItemsBatch with GROUP_CONCAT, matched_on, total_count)
- [x] 12-02-PLAN.md — Unified search frontend (dropdown, filter chips, URL state, mark highlights)

### Phase 13: Admin Panel Shell
**Goal**: A dedicated admin page exists as the central hub for application settings, with shelf configuration migrated from the grid page
**Depends on**: Phase 11
**Requirements**: ADMN-01, ADMN-03
**Success Criteria** (what must be TRUE):
  1. Navigating to /admin shows a dedicated page with clear section navigation
  2. Shelf settings (grid resize, rename) work from the admin page identically to how they worked from the grid modal
  3. The grid page no longer contains the settings gear/modal -- settings live exclusively in admin
  4. Navigation between the grid page and admin page is available from both directions
**Plans:** 2/2 plans complete
Plans:
- [x] 13-01-PLAN.md — Create admin page (handler, template, CSS, JS with shelf/auth/resize forms)
- [x] 13-02-PLAN.md — Migrate grid page (remove settings, add Admin link, visual verification)
**UI hint**: yes

### Phase 14: OIDC Authentication
**Goal**: Users can log in via an external OIDC provider (Authelia, Google) and the admin can configure the provider from the admin panel
**Depends on**: Phase 11, Phase 13
**Requirements**: OIDC-01, OIDC-02, OIDC-03, OIDC-04, ADMN-02
**Success Criteria** (what must be TRUE):
  1. User can click an SSO button on the login page and complete authentication through an OIDC provider redirect flow
  2. The OIDC flow uses PKCE, state, and nonce parameters -- inspecting the redirect URL shows these parameters present
  3. Local username/password login remains functional when OIDC is enabled -- both login methods coexist on the login page
  4. Admin can configure the OIDC provider (issuer URL, client ID, client secret, display name) from the auth settings section of the admin panel
  5. When no OIDC provider is configured, the login page shows only local auth with no broken SSO button
**Plans:** 4/4 plans complete
Plans:
- [x] 14-01-PLAN.md -- Session extensions + models + DB migrations + store OIDC CRUD
- [x] 14-02-PLAN.md -- OIDC package (config, AES-GCM cookie encryption, provider wrapper)
- [x] 14-03-PLAN.md -- OIDC flow handlers + routes + login template + header display
- [x] 14-04-PLAN.md -- Admin OIDC config UI + API + env var seeding + verification
**UI hint**: yes

### Phase 15: Data Export/Import
**Goal**: Users can back up all application data as a JSON file and restore from a backup, accessible from the admin panel
**Depends on**: Phase 13
**Requirements**: ADMN-04, ADMN-05
**Success Criteria** (what must be TRUE):
  1. Clicking export in admin downloads a JSON file containing all shelves, containers, items, and tags
  2. Uploading a previously exported JSON file restores the data with validation feedback (success count, skipped items, errors)
  3. Importing invalid JSON (wrong format, missing fields) shows a clear error message without corrupting existing data
  4. A round-trip test works: export -> clear data -> import -> all items and tags are back in correct containers
**Plans:** 2/2 plans complete
Plans:
- [x] 15-01-PLAN.md — Export model structs, store export/import methods, export handler, tests
- [x] 15-02-PLAN.md — Import handlers (validate/confirm), admin UI (template, CSS, JS)
**UI hint**: yes

### Phase 16: Redis Sessions
**Goal**: Sessions can optionally persist in Redis for restart survival, and the admin can see and revoke active sessions
**Depends on**: Phase 11, Phase 13
**Requirements**: SESS-02, ADMN-06
**Success Criteria** (what must be TRUE):
  1. Setting REDIS_URL env var activates Redis-backed sessions -- app logs confirm Redis store is active on startup
  2. Without REDIS_URL, the app falls back to in-memory sessions with no error or degraded behavior
  3. After app restart with Redis enabled, existing sessions survive -- user does not need to re-login
  4. Admin panel shows a list of active sessions with creation time and last activity
  5. Admin can revoke any session from the list -- the revoked user is forced to log in again on next request
**Plans**: TBD
**UI hint**: yes

### Phase 17: Documentation
**Goal**: A comprehensive README exists so a new developer or user can set up, configure, and use the application without reading source code
**Depends on**: Phase 14, Phase 16 (documents all features)
**Requirements**: DOCS-01
**Success Criteria** (what must be TRUE):
  1. README contains developer setup instructions that work on a fresh clone (go build, run, verify)
  2. README documents all environment variables (REDIS_URL, OIDC settings) with examples
  3. README includes a user guide section explaining grid usage, search, and admin panel
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 11 -> 12 -> 13 -> 14 -> 15 -> 16 -> 17
(Phase 12 is independent and can execute in parallel with Phase 11)

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Project Skeleton | v1.0 | 1/1 | Complete | 2026-04-02 |
| 2. Database Foundation | v1.0 | 1/1 | Complete | 2026-04-02 |
| 3. Grid UI Design | v1.0 | 1/1 | Complete | 2026-04-02 |
| 4. Grid Rendering | v1.0 | 1/1 | Complete | 2026-04-02 |
| 5. Item CRUD Backend | v1.0 | 2/2 | Complete | 2026-04-03 |
| 6. Item CRUD Frontend | v1.0 | 2/2 | Complete | 2026-04-03 |
| 7. Tag Autocomplete | v1.0 | 1/1 | Complete | 2026-04-03 |
| 8. Search Backend | v1.0 | 1/1 | Complete | 2026-04-03 |
| 9. Search Frontend | v1.0 | 2/2 | Complete | 2026-04-03 |
| 10. Resilience | v1.0 | 2/2 | Complete | 2026-04-03 |
| 11. Session Store Interface | v1.1 | 2/2 | Complete    | 2026-04-05 |
| 12. Search Enhancement | v1.1 | 2/2 | Complete    | 2026-04-05 |
| 13. Admin Panel Shell | v1.1 | 2/2 | Complete    | 2026-04-06 |
| 14. OIDC Authentication | v1.1 | 4/4 | Complete    | 2026-04-06 |
| 15. Data Export/Import | v1.1 | 2/2 | Complete    | 2026-04-06 |
| 16. Redis Sessions | v1.1 | 0/0 | Not started | - |
| 17. Documentation | v1.1 | 0/0 | Not started | - |

---
*Roadmap created: 2026-04-02*
*v1.1 phases added: 2026-04-05*
