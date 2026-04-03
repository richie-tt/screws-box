# Requirements: Screws Box

**Defined:** 2026-04-02
**Core Value:** Szybkie znalezienie pozycji pojemnika (np. "3B") po wpisaniu nazwy lub tagu elementu.

## v1 Requirements

### Grid Display

- [x] **GRID-01**: User can configure shelf dimensions (rows x columns, e.g., 5x10)
- [x] **GRID-02**: Grid displays as visual chessboard with column numbers (1,2,3...) and row letters (A,B,C...)
- [x] **GRID-03**: Each container shows its label (e.g., "3B") and item count
- [x] **GRID-04**: Grid is responsive — usable on phone/tablet in workshop
- [x] **GRID-05**: Grid resize warns about items in removed containers and blocks if items would be orphaned

### Item Management

- [x] **ITEM-01**: User can add item to container by clicking container on grid
- [x] **ITEM-02**: Item has name and multiple tags (e.g., "m6", "sprężynowa", "powiększona")
- [x] **ITEM-03**: User can edit item name and tags
- [x] **ITEM-04**: User can delete item
- [x] **ITEM-05**: One container can hold multiple different items
- [x] **ITEM-06**: Tag autocomplete suggests existing tags when adding/editing items

### Search

- [ ] **SRCH-01**: User can search by typing in search field — results appear as-you-type
- [ ] **SRCH-02**: Search matches item names and tags
- [ ] **SRCH-03**: Search is case-insensitive and supports partial matching ("m6" finds "M6", "spręż" finds "sprężynowa")
- [x] **SRCH-04**: Results display as list with item name, tags, and container position (e.g., "3B")
- [x] **SRCH-05**: Matching containers are visually highlighted on the grid
- [ ] **SRCH-06**: Keyboard navigation through search results

### Infrastructure

- [x] **INFR-01**: Data stored in SQLite database
- [x] **INFR-02**: App accessible on local network (0.0.0.0, not just localhost)
- [x] **INFR-03**: Single Go binary deployment (assets embedded via go:embed)

## v2 Requirements

### Quality of Life

- **QOL-01**: Bulk import — paste list of items to add multiple at once
- **QOL-02**: Print view — printable reference to tape on shelf
- **QOL-03**: Drag-and-drop items between containers
- **QOL-04**: FTS5 full-text search for large datasets

### Multi-Shelf

- **SHELF-01**: Support multiple shelves/organizers
- **SHELF-02**: Switch between shelves

## Out of Scope

| Feature | Reason |
|---------|--------|
| Quantity tracking / inventory | Not needed — you either have screws or you don't |
| Authentication / login | Home network, trusted environment |
| Mobile native app | Responsive web is sufficient |
| Barcode / QR scanning | Hardware dependency, overkill for home workshop |
| Purchase / ordering integration | Different product entirely |
| Photo per item | Storage complexity, standard hardware doesn't need photos |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| GRID-01 | Phase 4 | Complete |
| GRID-02 | Phase 3 | Complete |
| GRID-03 | Phase 4 | Complete |
| GRID-04 | Phase 3 | Complete |
| GRID-05 | Phase 10 | Complete |
| ITEM-01 | Phase 6 | Complete |
| ITEM-02 | Phase 5 | Complete |
| ITEM-03 | Phase 6 | Complete |
| ITEM-04 | Phase 5 | Complete |
| ITEM-05 | Phase 5 | Complete |
| ITEM-06 | Phase 7 | Complete |
| SRCH-01 | Phase 9 | Pending |
| SRCH-02 | Phase 8 | Pending |
| SRCH-03 | Phase 8 | Pending |
| SRCH-04 | Phase 9 | Complete |
| SRCH-05 | Phase 9 | Complete |
| SRCH-06 | Phase 9 | Pending |
| INFR-01 | Phase 2 | Complete |
| INFR-02 | Phase 1 | Complete |
| INFR-03 | Phase 1 | Complete |

**Coverage:**
- v1 requirements: 20 total
- Mapped to phases: 20
- Unmapped: 0

---
*Requirements defined: 2026-04-02*
*Last updated: 2026-04-02 after roadmap creation — all requirements mapped*
