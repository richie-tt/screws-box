---
phase: 02-database-foundation
plan: 01
subsystem: database
tags: [sqlite, schema, store, models, seeding]
dependency_graph:
  requires: [01-01]
  provides: [Store, Shelf, Container, Item, Tag, labelFor]
  affects: [main.go]
tech_stack:
  added: [modernc.org/sqlite v1.48.0]
  patterns: [DSN pragma configuration, idempotent schema creation, seed-on-first-run]
key_files:
  created: [models.go, store.go, models_test.go, store_test.go]
  modified: [main.go, go.mod, go.sum]
decisions:
  - DSN pragma parameters for WAL, foreign_keys, busy_timeout (per-connection enforcement)
  - item_tag join table has no timestamps (not an entity per research recommendation)
  - Separate transactions for schema creation and shelf seeding
metrics:
  duration: 3min
  completed: 2026-04-02T20:37:30Z
  tasks: 3
  files: 7
---

# Phase 02 Plan 01: SQLite Store Layer Summary

SQLite persistence layer with DSN-configured pragmas (WAL, foreign_keys, busy_timeout), 5-table normalized schema, labelFor() coordinate system, and auto-seeded default shelf (5x10, 50 containers).

## Tasks Completed

| # | Task | Commit | Key Files |
|---|------|--------|-----------|
| 1 | Create models.go, store.go with full schema and seeding | ac7cc47 | models.go, store.go, go.mod, go.sum |
| 2 | Write comprehensive tests for models and store | d4b1bc5 | models_test.go, store_test.go |
| 3 | Wire Store into main.go startup | 66c170f | main.go |

## What Was Built

### models.go
- 4 struct types: Shelf, Container, Item (with nullable Description), Tag
- `labelFor(col, row)` function: converts grid coordinates to human labels (e.g., 3,2 -> "3B")

### store.go
- `Store` struct wrapping `*sql.DB`
- `Open(dbPath)`: opens SQLite with DSN pragmas `_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)`
- `createSchema()`: 5 tables (shelf, container, item, tag, item_tag) with CASCADE DELETE foreign keys + 4 indexes
- `seedDefaultShelf()`: creates "My Organizer" shelf (5 rows x 10 cols) with 50 containers on first run
- `Close()`: clean shutdown

### main.go changes
- Reads `DB_PATH` env var (default `./screws_box.db`)
- Opens Store before router setup, defers Close

### Test Coverage (8 tests, all passing with -race)
- TestLabelFor: 6 coordinate cases including boundaries
- TestStoreOpenCreatesFile: DB file creation verified
- TestPragmasSet: WAL, foreign_keys=1, busy_timeout=5000
- TestSchemaTablesExist: all 5 tables in sqlite_master
- TestDefaultShelfSeeded: "My Organizer" with 50 containers
- TestSeedIdempotent: second Open does not duplicate data
- TestCascadeDeleteContainerRemovesItems: FK cascade verified
- TestCascadeDeleteItemRemovesItemTags: FK cascade verified

## Decisions Made

1. **DSN pragma parameters** (not post-open Exec): ensures every connection from the pool gets WAL, foreign_keys, busy_timeout
2. **No timestamps on item_tag**: join table is not an entity; composite PK (item_id, tag_id) is sufficient
3. **Separate transactions**: schema DDL in one tx, shelf seeding in another -- cleaner error handling

## Deviations from Plan

None -- plan executed exactly as written.

## Known Stubs

None -- all data paths are fully wired.

## Verification Results

- `go test -v -count=1 -race ./...` -- PASS (8 tests, 0 failures)
- `go build -o /tmp/screws-box .` -- single binary (19MB)
- All must_haves truths verified by tests

## Self-Check: PASSED

All 5 created/modified source files exist. All 3 task commits verified in git log.
