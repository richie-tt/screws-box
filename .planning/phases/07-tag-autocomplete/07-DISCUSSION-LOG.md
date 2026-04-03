# Phase 7: Tag Autocomplete - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-03
**Phase:** 07-tag-autocomplete
**Areas discussed:** Dropdown behavior, Selection interaction, Visual styling

---

## Dropdown Behavior

### Trigger
| Option | Description | Selected |
|--------|-------------|----------|
| On focus + any input | Show suggestions on 1+ chars. Fast for small tag sets. | ✓ |
| After 2+ characters | Wait for 2 chars before querying. | |
| On focus (show all) | Show all tags on focus, filter as typing. | |

**User's choice:** On focus + any input

### Max Results
| Option | Description | Selected |
|--------|-------------|----------|
| 5 suggestions | Compact, fits panel without scroll. | ✓ |
| 8 suggestions | More choices, may need scroll. | |
| All matching | Show every match, scrollable. | |

**User's choice:** 5 suggestions

---

## Selection Interaction

### Selection Method
| Option | Description | Selected |
|--------|-------------|----------|
| Click + keyboard | Click OR Arrow Down/Up + Enter. | ✓ |
| Click only | Mouse only, simpler code. | |
| Keyboard only | Arrow keys + Enter, no click. | |

**User's choice:** Click + keyboard

### Exact Match Behavior
| Option | Description | Selected |
|--------|-------------|----------|
| Add it directly | Enter always adds typed text. Dropdown is a shortcut. | ✓ |
| Auto-select the match | Highlight exact match, Enter selects it. | |

**User's choice:** Add it directly

---

## Visual Styling

| Option | Description | Selected |
|--------|-------------|----------|
| Inline below input | Small dropdown inside panel, uses design tokens. | ✓ |
| Floating popover | Absolute positioned, can overflow panel. | |
| You decide | Claude picks based on constraints. | |

**User's choice:** Inline below input

---

## Claude's Discretion

- Debounce timing for API calls
- Dropdown dismiss behavior
- Case sensitivity handling

## Deferred Ideas

None.
