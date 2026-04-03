<!-- GSD:project-start source:PROJECT.md -->
## Project

**Screws Box**

Aplikacja webowa do zarządzania organizerem na drobne elementy złączne (śruby, podkładki, nakrętki itp.). Prezentuje półkę jako konfigurowalną siatkę pojemników (np. 5x10), umożliwia dodawanie elementów z tagami/kategoriami i wyszukiwanie po nazwie lub tagu z wizualnym podświetleniem pozycji na siatce. Dostępna w sieci domowej.

**Core Value:** Szybkie znalezienie pozycji pojemnika (np. "3B") po wpisaniu nazwy lub tagu elementu.

### Constraints

- **Stack**: Go (Golang) backend, plain HTML frontend (plugin frontend-design do projektowania UI), SQLite baza danych
- **Frontend design**: Użyć pluginu `frontend-design` do zaprojektowania interfejsu
- **Deployment**: Dostępna w sieci domowej (nasłuch na 0.0.0.0, nie 127.0.0.1)
- **Simplicity**: Jeden binary Go + plik SQLite, minimalna konfiguracja
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## Recommended Stack
### Backend
| Component | Choice | Version | Confidence | Rationale |
|-----------|--------|---------|-----------|-----------|
| Language | Go | 1.26+ | HIGH | Project constraint |
| Router | chi | v5.2.5 | HIGH | 100% net/http compatible, ~1000 LOC, built-in middleware. Verified on pkg.go.dev (2026-02-05) |
| SQLite driver | modernc.org/sqlite | v1.48.0 | HIGH | CGo-free port of SQLite 3.51.3, 3,444 downstream importers. Single-binary deployment. Verified on pkg.go.dev (2026-03-27) |
| Templating | html/template (stdlib) | - | HIGH | Project constraint ("plain HTML"). No build step needed. |
### Frontend
| Component | Choice | Version | Confidence | Rationale |
|-----------|--------|---------|-----------|-----------|
| Interactivity | htmx | 2.x | MEDIUM | Search-as-you-type via HTML attributes, no JS build step, ~14KB. Server-rendered HTML fragments. |
| CSS Framework | Custom (app.css) | - | HIGH | Custom design system with DM Sans/DM Mono fonts, design tokens, light/dark mode. Pico CSS was REMOVED in Phase 6 — do NOT use or reference Pico CSS. |
| Custom JS | Vanilla JS | - | HIGH | Minimal JS for grid interactions where htmx isn't sufficient |
### Infrastructure
| Component | Choice | Rationale |
|-----------|--------|-----------|
| Database | SQLite (file) | Single file, zero config, embedded in Go binary |
| Asset embedding | go:embed | Single binary deployment, no external file paths |
| Build | `go build` | No Makefile needed, single command |
## Total External Go Dependencies: 2
## What NOT to Use
| Rejected | Why |
|----------|-----|
| mattn/go-sqlite3 | Requires CGo — breaks single-binary cross-compilation |
| Gin / Echo / Fiber | Heavier than needed, chi is lighter and stdlib-compatible |
| templ | Pre-stable (v0.3), adds build complexity for ~5 templates |
| React / Vue / Svelte | SPA is overkill, server-rendered HTML + htmx is simpler |
| Tailwind CSS | Requires build step, custom app.css is sufficient |
| Pico CSS | REMOVED in Phase 6 — caused specificity wars with custom styles. Do not re-introduce. |
| GORM / ent | ORM adds abstraction over simple SQLite queries |
## Search Strategy
## Open Questions
- Exact htmx 2.x version — verify at build time from htmx.org
- FTS5 behavior with modernc.org/sqlite — needs quick validation spike (high confidence it works, worth confirming)
## Roadmap Implications
- **Phase 1:** Project skeleton — Go module, chi router, modernc/sqlite, embedded templates, static assets
- **Phase 2:** Grid display + container CRUD — CSS Grid for chessboard layout
- **Phase 3:** Search with htmx — the core value proposition
- FTS5 support needs early validation spike
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

- **CSS**: Custom design system in `static/css/app.css` (design tokens, reset, typography, forms, buttons) + `static/css/grid.css` (grid-specific). All styles use CSS custom properties from `app.css` (e.g. `var(--accent)`, `var(--bg-raised)`). No CSS framework.
- **Fonts**: DM Sans (body) + DM Mono (coordinates, headers) loaded from Google Fonts in `layout.html`
- **Dark mode**: `@media (prefers-color-scheme: dark)` overrides on `:root` tokens — no manual toggle
- **Frontend JS**: Vanilla JS in IIFE pattern (`(function() { 'use strict'; ... })();`), no build step
- **Panel pattern**: Floating `position: fixed` panel appended to `document.body`, not inline grid expansion
- **Language**: All UI text in English. No Polish strings in templates or JS.
- **Templates**: Go `html/template` with `layout.html` base + block overrides per page
- **API**: JSON REST endpoints under `/api/`, handlers in `handlers.go`, store methods in `store.go`
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

- **Single binary**: `go build` produces one executable with all assets embedded via `go:embed`
- **Dev mode**: `go run -tags dev .` reads files from disk (live reload)
- **Entry**: `main.go` → `routes.go` (chi router) → handlers
- **Data flow**: `handlers.go` → `store.go` (SQL) → SQLite via modernc.org/sqlite
- **Models**: `models.go` — Cell, GridData, Item, Container structs
- **Templates**: `templates/` — layout.html (base), grid.html (main page)
- **Static assets**: `static/css/app.css` (design system), `static/css/grid.css` (grid+panel), `static/js/grid.js` (CRUD interactions)
- **Database**: Single `screws_box.db` file, WAL mode, foreign keys ON
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
