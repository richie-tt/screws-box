# Roadmap: Screws Box

**Milestone:** v1
**Granularity:** Fine
**Requirements:** 20 v1 requirements
**Created:** 2026-04-02

## Phases

- [ ] **Phase 1: Project Skeleton** — Running Go binary with chi, embedded assets, 0.0.0.0 binding, graceful shutdown
- [x] **Phase 2: Database Foundation** — SQLite store with correct pragmas, normalized schema, coordinate system (completed 2026-04-02)
- [x] **Phase 3: Grid UI Design** — Visual chessboard design via frontend-design plugin, responsive layout (completed 2026-04-02)
- [ ] **Phase 4: Grid Rendering** — Server-rendered grid with cell labels, item counts, configurable dimensions
- [x] **Phase 5: Item CRUD Backend** — Store + API endpoints for create/read/update/delete items with tags (completed 2026-04-03)
- [ ] **Phase 6: Item CRUD Frontend** — Click container to add/edit/delete items; form wired to API
- [ ] **Phase 7: Tag Autocomplete** — Existing tags suggested when adding or editing items
- [ ] **Phase 8: Search Backend** — SearchItems query with exact tag match and LIKE on names, /api/search endpoint
- [ ] **Phase 9: Search Frontend** — As-you-type results, grid cell highlighting, keyboard navigation
- [ ] **Phase 10: Resilience** — Grid resize guard: warn on occupied cells, block if items would be orphaned

## Phase Details

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
**Plans:** 1 plan
Plans:
- [x] 01-01-PLAN.md — Go project skeleton with chi router, embedded assets, graceful shutdown, placeholder page, and tests

### Phase 2: Database Foundation
**Goal**: The application opens SQLite with all correctness pragmas set and the full normalized schema in place before any feature code runs
**Depends on**: Phase 1
**Requirements**: INFR-01
**Success Criteria** (what must be TRUE):
  1. App starts without error and a `screws_box.db` file is created on first run
  2. A Store struct exists with `Open()` that sets `PRAGMA journal_mode=WAL`, `PRAGMA foreign_keys=ON`, and `PRAGMA busy_timeout=5000` before returning
  3. Schema contains tables: `shelf`, `container`, `item`, `tag`, `item_tag` — visible via `sqlite3 .schema`
  4. A single `labelFor(col, row int) string` function in `models.go` returns correct labels (e.g., col=3 row=2 → "3B")
  5. Deleting a container cascades correctly to its items (foreign key constraint enforced, not silently ignored)
**Plans:** 1/1 plans complete
Plans:
- [x] 02-01-PLAN.md — SQLite Store with pragmas, 5-table schema, labelFor(), default shelf seeding, and tests

### Phase 3: Grid UI Design
**Goal**: A static HTML mockup defines the visual design for the chessboard grid, container cells, and search bar — ready for server-side wiring
**Depends on**: Phase 1
**Requirements**: GRID-02, GRID-04
**Success Criteria** (what must be TRUE):
  1. Opening the static mockup in a browser shows a chessboard-style grid with column numbers across the top and row letters down the side
  2. Each cell clearly displays its coordinate label (e.g., "3B") in a consistent position
  3. The layout is usable on a 375px-wide phone screen without horizontal scroll for a 5x10 grid
  4. A search input field is visible and prominently placed above or beside the grid
  5. Container cells have a visually distinct highlighted state (for search results) that is clearly different from the default state
**Plans:** 1/1 plans complete
Plans:
- [x] 03-01-PLAN.md — Chessboard grid CSS + HTML mockup with sticky headers, search bar, responsive layout, and highlight states
**UI hint**: yes

### Phase 4: Grid Rendering
**Goal**: The server renders the real configurable grid with correct labels and live item counts from the database
**Depends on**: Phase 2, Phase 3
**Requirements**: GRID-01, GRID-03
**Success Criteria** (what must be TRUE):
  1. A shelf configuration (rows x columns) stored in the database is read at startup and drives the rendered grid dimensions
  2. Every cell displays its correct coordinate label (e.g., "1A", "3B", "10E") matching `labelFor()` output
  3. A cell that contains items shows the item count; an empty cell shows zero or an empty indicator
  4. Changing the shelf dimensions in the database and restarting re-renders the grid with the new size
**Plans:** 1 plan
Plans:
- [x] 04-01-PLAN.md — Store GetGridData + dynamic grid template wired to live DB data on GET /
**UI hint**: yes

### Phase 5: Item CRUD Backend
**Goal**: All item and tag data operations are available as JSON API endpoints backed by the Store layer
**Depends on**: Phase 2
**Requirements**: ITEM-02, ITEM-04, ITEM-05
**Success Criteria** (what must be TRUE):
  1. `POST /api/items` creates an item in a specific container with a name and one or more tags; returns the created item as JSON
  2. `PUT /api/items/:id` updates an item's name and replaces its tag set; returns updated item
  3. `DELETE /api/items/:id` removes the item and its tag associations; the container still exists afterward
  4. A container can hold multiple different items — adding a second item to the same container succeeds
  5. Tags are stored in the normalized `item_tag` junction table, not as a comma-separated string
**Plans:** 2/2 plans complete
Plans:
- [x] 05-01-PLAN.md — Store CRUD methods for items/tags with integration tests
- [x] 05-02-PLAN.md — HTTP handlers, validation, route registration, and handler tests

### Phase 6: Item CRUD Frontend
**Goal**: A user can add, edit, and delete items in any container using only the browser, with no manual API calls
**Depends on**: Phase 4, Phase 5
**Requirements**: ITEM-01, ITEM-03
**Success Criteria** (what must be TRUE):
  1. Clicking an empty container opens an add-item form (inline or modal) with name and tags fields
  2. Submitting the form creates the item and the grid cell immediately reflects the new item count without a full page reload
  3. Clicking a container that already has items offers an option to edit or delete each item
  4. Editing an item pre-fills the form with the current name and tags; saving updates the display
  5. Deleting an item removes it from the container; the cell count updates immediately
**Plans:** 2 plans
Plans:
- [ ] 06-01-PLAN.md — Remove dialog HTML, switch Polish to English, add expanded-cell/tag-chip/pulse CSS
- [ ] 06-02-PLAN.md — Complete grid.js rewrite for inline cell expansion CRUD with browser verification
**UI hint**: yes

### Phase 7: Tag Autocomplete
**Goal**: When typing tags, users see suggestions drawn from existing tags in the database, preventing fragmentation (M6 vs m6 vs M-6)
**Depends on**: Phase 5, Phase 6
**Requirements**: ITEM-06
**Success Criteria** (what must be TRUE):
  1. Typing into the tags field shows a dropdown of matching existing tags from the database
  2. Selecting a suggestion fills the tag field without requiring the user to finish typing
  3. A tag that does not yet exist can still be entered freely (autocomplete is a suggestion, not a constraint)
  4. The suggestion list updates as the user types (not a static pre-loaded list)
**Plans**: TBD
**UI hint**: yes

### Phase 8: Search Backend
**Goal**: A search endpoint returns all items and their container positions matching a query, using correct matching semantics
**Depends on**: Phase 5
**Requirements**: SRCH-02, SRCH-03
**Success Criteria** (what must be TRUE):
  1. `GET /api/search?q=m6` returns items whose name contains "m6" (case-insensitive) or whose tags exactly match "m6"
  2. Partial name matching works: searching "sprez" finds items named "sprezynowa" or "sprezyna"
  3. Tag search uses exact match via the junction table — "m6" does NOT match items tagged "m60"
  4. Each result includes the item name, all its tags, and the container position label (e.g., "3B")
  5. An empty query returns no results (not all items)
**Plans**: TBD

### Phase 9: Search Frontend
**Goal**: Users can find any part in the organizer by typing its name or tag and immediately see which containers hold it, both as a list and highlighted on the grid
**Depends on**: Phase 6, Phase 8
**Requirements**: SRCH-01, SRCH-04, SRCH-05, SRCH-06
**Success Criteria** (what must be TRUE):
  1. Typing in the search field triggers a fetch to `/api/search` and shows results without pressing Enter (as-you-type, debounced)
  2. Results appear as a list showing item name, tags, and container position (e.g., "M6 sprezynowa [m6, sprezynowa] -> 3B")
  3. Containers matching the search are visually highlighted on the grid; all other containers are visually de-emphasized
  4. Clearing the search field removes all highlights and the results list
  5. The user can navigate through results with keyboard arrow keys and press Enter/Space to focus the corresponding grid cell
**Plans**: TBD
**UI hint**: yes

### Phase 10: Resilience
**Goal**: The grid can be safely resized — users are warned about occupied positions and blocked from creating orphaned items
**Depends on**: Phase 6
**Requirements**: GRID-05
**Success Criteria** (what must be TRUE):
  1. Attempting to shrink the grid to a size that would remove containers with items shows a clear warning listing the affected containers and their contents
  2. The resize is blocked (not executed) if any items would be orphaned; the grid remains unchanged
  3. Resizing to a larger grid succeeds immediately and new empty cells appear in the correct positions
  4. Resizing a grid where all removed cells are empty succeeds without any warning

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Project Skeleton | 0/1 | Planning complete | - |
| 2. Database Foundation | 1/1 | Complete   | 2026-04-02 |
| 3. Grid UI Design | 1/1 | Complete   | 2026-04-02 |
| 4. Grid Rendering | 0/1 | Planning complete | - |
| 5. Item CRUD Backend | 2/2 | Complete   | 2026-04-03 |
| 6. Item CRUD Frontend | 0/2 | In Progress|  |
| 7. Tag Autocomplete | 0/? | Not started | - |
| 8. Search Backend | 0/? | Not started | - |
| 9. Search Frontend | 0/? | Not started | - |
| 10. Resilience | 0/? | Not started | - |

---
*Roadmap created: 2026-04-02*
*Last updated: 2026-04-03 after Phase 6 replanning*
