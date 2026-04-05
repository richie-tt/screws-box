---
phase: 12-search-enhancement
plan: 01
subsystem: api
tags: [sqlite, group-concat, batch-query, search, go]

# Dependency graph
requires:
  - phase: 09-search-frontend
    provides: existing SearchItems and SearchItemsByTags store methods, handleSearch handler
provides:
  - SearchItemsBatch store method replacing N+1 with GROUP_CONCAT batch SQL
  - SearchResult and SearchResponse model types with matched_on metadata
  - Updated handleSearch handler returning total_count and matched_on
affects: [12-search-enhancement plan 02 (frontend), search-frontend]

# Tech tracking
tech-stack:
  added: []
  patterns: [GROUP_CONCAT batch query, LIMIT 51 count trick, computeMatchedOn server-side, subquery for full tag list in filtered path]

key-files:
  created: []
  modified:
    - internal/model/model.go
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/server/handler.go
    - internal/server/handler_test.go

key-decisions:
  - "Subquery for tag_list in filtered path avoids GROUP_CONCAT only returning matched tags"
  - "LIMIT 51 trick: fetch 51, if hit, run COUNT(*) subquery for exact total_count"
  - "computeMatchedOn runs server-side after SQL, not in SQL, for clarity and testability"

patterns-established:
  - "Batch search pattern: single SQL with GROUP_CONCAT for tags, no N+1 GetItem calls"
  - "SearchResponse envelope: {results: [...], total_count: N} for all search endpoints"

requirements-completed: [SRCH-01, SRCH-02, SRCH-03, SRCH-04]

# Metrics
duration: 4min
completed: 2026-04-06
---

# Phase 12 Plan 01: Search Batch Backend Summary

**GROUP_CONCAT batch search replacing N+1 queries with matched_on metadata, description search, multi-tag AND filtering, and 50-item cap with total_count**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-05T22:29:26Z
- **Completed:** 2026-04-05T22:33:16Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Replaced N+1 search pattern (SearchItems + GetItem per row) with single GROUP_CONCAT batch query
- Two SQL code paths: no-tags (name OR exact tag OR description) and with-tags (AND logic on tags, text on name+description only)
- API response now includes matched_on array and total_count field
- Results capped at 50 with accurate total_count via LIMIT 51 trick
- 14 new store tests + 4 new handler tests covering all search behaviors

## Task Commits

Each task was committed atomically:

1. **Task 1: Create SearchItemsBatch store method with batch SQL** - `ee56ef5` (feat)
2. **Task 2: Update handler and StoreService interface for batch search** - `a048e76` (feat)

_Note: Task 1 followed TDD flow (RED: tests fail on missing method, GREEN: implement and pass)_

## Files Created/Modified
- `internal/model/model.go` - Added SearchResult and SearchResponse types
- `internal/store/store.go` - Added SearchItemsBatch, searchBatchCount, computeMatchedOn methods
- `internal/store/store_test.go` - 14 TestSearchBatch* test functions
- `internal/server/handler.go` - Updated StoreService interface and handleSearch to use SearchItemsBatch
- `internal/server/handler_test.go` - 4 new handler tests, updated 5 existing tests for new response shape

## Decisions Made
- Used subquery for tag_list in filtered path to return ALL item tags, not just the matched filter tags
- LIMIT 51 trick avoids always running COUNT(*) -- only runs when results exceed 50
- computeMatchedOn is a pure Go function, not SQL-based, for testability and clarity
- Kept old SearchItems/SearchItemsByTags methods intact for backward compatibility during transition

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated existing search handler tests for new response shape**
- **Found during:** Task 2 (handler update)
- **Issue:** 5 existing tests (TestHandleSearchByName, TestHandleSearchByTag, TestHandleSearchEmpty, TestHandleSearchMissingParam, TestHandleSearchCaseInsensitive) decoded response as `map[string][]model.ItemResponse` but new format is `model.SearchResponse`
- **Fix:** Updated all 5 tests to decode into `model.SearchResponse` and access `resp.Results` instead of `resp["results"]`
- **Files modified:** internal/server/handler_test.go
- **Verification:** `go test ./... -count=1` all pass
- **Committed in:** a048e76 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Necessary fix for existing tests broken by response format change. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- SearchItemsBatch backend ready for plan 02 (frontend integration)
- API returns matched_on and total_count needed for dropdown results, filter chips, and highlight marking
- Old SearchItems/SearchItemsByTags still in codebase for safe removal after frontend verified

---
*Phase: 12-search-enhancement*
*Completed: 2026-04-06*
