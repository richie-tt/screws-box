# Phase 7: Tag Autocomplete - Context

**Gathered:** 2026-04-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Suggest existing tags as user types in the tag input field — both in add-item form and edit-item form. Prevents tag fragmentation (m6 vs M6 vs M-6). Purely frontend work — backend API (`GET /api/tags?q=prefix`) already exists and is tested.

</domain>

<decisions>
## Implementation Decisions

### Dropdown Trigger
- **D-01:** Show autocomplete on focus + any input (1+ characters). No minimum character threshold.
- **D-02:** Max 5 suggestions shown at once. No scrolling needed.

### Selection Interaction
- **D-03:** Both click and keyboard selection supported. Arrow Down/Up highlights suggestions, Enter selects highlighted item.
- **D-04:** Enter without a highlighted suggestion adds the typed text as a new tag (same as current behavior). Dropdown is a shortcut, not mandatory.
- **D-05:** Typing exactly matching an existing tag and pressing Enter adds it directly — no auto-selection of the dropdown match.

### Visual Styling
- **D-06:** Inline dropdown directly below tag input field, inside the panel. Not a floating popover.
- **D-07:** Uses existing design tokens: `--bg-raised` background, `--border` border, `--shadow-md` shadow. Highlighted item uses `--accent-subtle` background.

### From Phase 6 (carried forward)
- **D-08:** Tag input is one-at-a-time, Enter to add chip (Phase 6 D-08, D-16).
- **D-09:** Edit mode tags use live API — add via POST, remove via DELETE (Phase 6 D-14, D-15).
- **D-10:** All UI text in English. Custom design system (app.css), no Pico CSS.

### Claude's Discretion
- Debounce timing for API calls (e.g., 150-300ms)
- Dropdown dismiss behavior (blur, Escape, click outside)
- Handling of case sensitivity in matching (backend already lowercases)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Backend API
- `handlers.go:319` — `handleListTags()` handler, `GET /api/tags?q=prefix`
- `store.go:603` — `ListTags(ctx, prefix)` query with prefix filtering
- `routes.go:34` — Route registration

### Frontend
- `static/js/grid.js` — Tag input in `renderAddForm()` (line ~313) and `renderInlineEdit()` (line ~513)
- `static/css/grid.css` — `.tag-chip`, `.tag-chips-container`, `.tag-hint` styles
- `static/css/app.css` — Design tokens (`--bg-raised`, `--border`, `--accent-subtle`, `--shadow-md`)

### Prior Decisions
- `.planning/phases/06-item-crud-frontend/06-CONTEXT.md` — D-08, D-14, D-15, D-16 (tag input behavior)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `apiCall()` utility in grid.js — async fetch wrapper, returns `{ ok, status, data }`
- Tag input `keydown` handler already exists in both add and edit forms — needs augmentation, not replacement
- `renderTagChips()` / `renderEditTagChips()` functions for chip rendering

### Established Patterns
- Event delegation on grid container for cell clicks
- IIFE pattern, vanilla JS, no build step
- Design tokens in CSS custom properties

### Integration Points
- Tag input in `renderAddForm()` — add autocomplete dropdown below the input
- Tag input in `renderInlineEdit()` — same autocomplete, but uses live API for tag add
- New CSS classes needed in `grid.css` for dropdown list styling

</code_context>

<specifics>
## Specific Ideas

No specific requirements — standard autocomplete pattern with the decisions above.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 07-tag-autocomplete*
*Context gathered: 2026-04-03*
