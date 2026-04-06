---
phase: 15-data-export-import
plan: 01
subsystem: data-export-import
tags: [export, import, store, handler, tdd]
dependency_graph:
  requires: []
  provides: [ExportAllData, ImportAllData, GET /api/export]
  affects: [internal/server/handler.go, internal/server/routes.go]
tech_stack:
  added: []
  patterns: [batch SQL queries grouped in Go, transactional clear-and-insert]
key_files:
  created:
    - internal/model/export.go
    - internal/store/export.go
    - internal/store/export_test.go
    - internal/server/handler_export.go
  modified:
    - internal/server/handler.go
    - internal/server/routes.go
    - internal/server/handler_test.go
decisions:
  - Batch SQL queries (2 total for items+tags) instead of per-container queries for export performance
  - Import only inserts containers from export data (not all grid positions), matching export behavior
metrics:
  duration: 185s
  completed: "2026-04-06T11:32:21Z"
  tasks: 2
  files: 7
---

# Phase 15 Plan 01: Export/Import Backend Core Summary

Batch-queried ExportAllData builds nested JSON tree (version=1, no DB IDs, position-based keys); ImportAllData does transactional clear-and-insert with rollback on error; GET /api/export triggers browser download with dated filename.

## What Was Built

### Task 1: Export model structs + store methods + tests (TDD)

- **internal/model/export.go**: ExportData, ExportShelf, ExportContainer, ExportItem structs. No DB IDs anywhere. Tags as `[]string`.
- **internal/store/export.go**: `ExportAllData` uses 3 batch queries (containers, items, tags) grouped in Go by container/item ID. `ImportAllData` validates version, runs full clear-and-insert in a single transaction with rollback on any error. Tag deduplication via `INSERT OR IGNORE INTO tag`.
- **internal/store/export_test.go**: 7 tests covering export with data, export empty, import replacing data, rollback on duplicate container, invalid version rejection, round-trip verification, and tag deduplication.

### Task 2: Export download handler + route + interface

- **internal/server/handler_export.go**: `handleExport()` returns pretty-printed JSON with `Content-Disposition: attachment; filename="screws-box-export-YYYY-MM-DD.json"`.
- **internal/server/handler.go**: Added `ExportAllData` and `ImportAllData` to `StoreService` interface.
- **internal/server/routes.go**: Added `r.Get("/export", srv.handleExport())` inside the authenticated `/api` route group.
- **internal/server/handler_test.go**: Added stub `ExportAllData` and `ImportAllData` methods to `mockStore` for interface compliance.

## Commits

| Task | Commit  | Message |
|------|---------|---------|
| 1 (RED) | 5848a63 | test(15-01): add failing tests for export/import store methods |
| 1 (GREEN) | 19a682a | feat(15-01): implement ExportAllData and ImportAllData store methods |
| 2 | ac16c21 | feat(15-01): add export download handler, route, and StoreService interface |

## Verification Results

- `go test ./internal/store/ -run "TestExport|TestImport|TestRoundTrip" -v -count=1` -- 7/7 PASS
- `go build ./...` -- OK
- `go vet ./...` -- OK
- `go test ./... -count=1` -- all packages PASS (no regressions)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated mockStore in handler_test.go**
- **Found during:** Task 2
- **Issue:** Adding methods to `StoreService` interface broke compilation of handler_test.go because `mockStore` did not implement the new methods.
- **Fix:** Added stub `ExportAllData` and `ImportAllData` methods to `mockStore`.
- **Files modified:** internal/server/handler_test.go
- **Commit:** ac16c21

## Self-Check: PASSED
