---
phase: 15-data-export-import
plan: "02"
subsystem: admin-export-import-ui
tags: [admin, export, import, ui, two-step-flow]
dependency_graph:
  requires: [15-01]
  provides: [import-validate-handler, import-confirm-handler, admin-data-section]
  affects: [admin.html, admin.css, admin.js, handler_export.go, routes.go]
tech_stack:
  added: []
  patterns: [pending-import-token, two-step-import, crypto-rand-token, maxbytes-reader]
key_files:
  created: []
  modified:
    - internal/server/handler_export.go
    - internal/server/routes.go
    - internal/server/templates/admin.html
    - internal/server/static/css/admin.css
    - internal/server/static/js/admin.js
decisions:
  - "In-memory pending import map with 5min TTL (single-user app, acceptable)"
  - "Export via window.location.href for immediate browser download"
  - "textContent-only DOM manipulation for XSS prevention"
metrics:
  duration: "2m"
  completed: "2026-04-06"
  tasks_completed: 2
  tasks_total: 3
---

# Phase 15 Plan 02: Admin Export/Import UI Summary

Import validate+confirm handlers with two-step pending token flow, and complete admin Data section UI with export download, file upload, validation summary, destructive warning, and all interaction states.

## What Was Built

### Task 1: Import Handlers (e5865f0)
- `handleImportValidate`: 10MB MaxBytesReader limit, JSON decode into ExportData, version/structure validation, item/tag counting for summary, crypto/rand token generation, pending import storage with 5min TTL
- `handleImportConfirm`: Token lookup with single-use deletion, TTL expiry check, calls ImportAllData, returns success/error messages
- Routes: POST `/api/import/validate` and `/api/import/confirm` inside auth+CSRF protected group

### Task 2: Admin Data Section UI (17e4dff)
- Added "Data" nav item in sidebar (between Authentication and Sessions)
- Data Export: button triggers `window.location.href = '/api/export'` for immediate download
- Data Import: file input with `.json` accept filter, Validate File button
- Validation summary: grid layout showing shelf name/dimensions, container/item/tag counts
- Destructive warning banner: "This will replace ALL existing data. This cannot be undone."
- Confirm flow: "Replace All Data" danger button, success message, auto-reload after 2s
- Error flow: validation errors list, "Try Another File" button
- All dynamic content via `textContent` (no innerHTML) for XSS safety

## Deviations from Plan

None - plan executed exactly as written.

## Verification

- `go build ./...` -- passes
- `go vet ./...` -- passes
- `go test ./... -count=1` -- all 6 packages pass
- Task 3 (human-verify) -- pending browser round-trip verification

## Known Stubs

None.
