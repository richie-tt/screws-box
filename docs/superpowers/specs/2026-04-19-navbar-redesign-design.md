# NavBar Redesign & Settings UI Improvements

## Summary

Redesign the NavBar across grid and settings pages for visual consistency: replace mixed text links and icons with a uniform icon-based system. Add an avatar pill with dropdown for user identity. Add icons to the settings sidebar. Paginate the tags section in settings.

## Design Decisions

All decisions were validated via interactive browser mockups.

### 1. Grid Page NavBar (Right Side)

**Layout:** Three elements in the header-actions area:

1. **Theme toggle** (sun/moon icon) -- unchanged from current implementation
2. **Gear icon** -- ghost-style icon button linking to `/settings`
3. **Avatar pill** -- pill-shaped button containing:
   - Initials circle (colored background, white text, e.g., "RT")
   - First name text (e.g., "Robert")
   - Chevron-down indicator

The standalone logout icon and raw text "Settings" link are removed from the NavBar.

**Initials derivation:** Extract first letter of first name + first letter of last name from `DisplayName`. If only one word, use first two letters. Fallback to generic user icon if no display name.

**Avatar pill styling:**
- Border: 1px solid `var(--border)`
- Border-radius: 20px (full pill)
- Padding: 4px 10px 4px 4px
- Initials circle: 26px diameter, `var(--accent)` background, white text, 11px font-size, 600 weight
- Name: 13px font-size
- Chevron: 10x10 SVG

### 2. Avatar Dropdown

Opens on pill click, closes on outside click or Escape key.

**Contents:**
- **Header section** (top, with bottom border):
  - Full name (bold, 14px): e.g., "Robert Tkocz"
  - Auth type label (muted, 12px): "Local admin" or OIDC display name
- **Menu items:**
  - Settings -- gear icon + "Settings" text, links to `/settings`
  - Divider (1px line)
  - Sign out -- door icon + "Sign out" text in `var(--danger)` color, links to `/logout`

**Dropdown styling:**
- Width: 220px
- Position: absolute, anchored to right edge of pill, below NavBar
- Background: `var(--bg-raised)`
- Border: 1px solid `var(--border)`
- Border-radius: 8px
- Shadow: `var(--shadow-md)`
- Menu items: 8px 12px padding, 4px border-radius on hover with `var(--bg-inset)` background

**Implementation:** Pure vanilla JS, no framework. Pattern matches the existing theme toggle approach. Event listener on document for outside-click dismissal.

### 3. Settings Page NavBar

**Left side:**
- Back arrow icon (left-pointing arrow, 18px) linking to `/`
- Title text: "Settings" (bold, 1.1rem) -- no subtitle

**Right side (two elements only):**
1. Theme toggle (sun/moon) -- same as grid page
2. Avatar pill -- same as grid page

No gear icon on settings page since the user is already in settings. The back arrow provides navigation back to the grid.

**Title area styling:**
- Flex row, center-aligned
- Arrow and title separated by 10px gap
- Arrow is a ghost-styled link (`a.ghost`)

### 4. Settings Sidebar Icons

Add 16x16 stroke icons to each sidebar nav item. Shorten labels where sensible.

| Current Label    | New Label    | Icon                    |
|------------------|--------------|-------------------------|
| Shelf Settings   | Shelf        | 4-square grid           |
| Tags             | Tags         | Tag label               |
| Housekeeping     | Housekeeping | Wrench                  |
| Authentication   | Auth         | Shield                  |
| Data             | Data         | Database cylinder       |
| Sessions (N)     | Sessions (N) | Users group             |

**Nav item styling:**
- `display: flex; align-items: center; gap: 10px`
- Icon color inherits from text color (muted for inactive, accent for active)
- Active item keeps existing left-border accent + background highlight

### 5. Tags Section Pagination

**Default state:** Show first 7 tags.

**"Load more" button:**
- Appears below the visible tags when total count exceeds 7
- Label: "Show more tags" + muted count text (e.g., "24 remaining")
- Styled as secondary button (transparent background, border)
- Centered, with a top border separator

**Behavior on click:**
- Append next 15 tags below the existing visible tags
- Update remaining count in button text
- When all tags are loaded, hide the button permanently
- All tags remain visible on the page (no collapsing back)

**Batch sizes:**
- Initial display: 7 tags
- Each "load more" click: 15 additional tags
- Example with 31 tags: 7 shown -> click -> 22 shown (15 more) -> click -> 31 shown (all), button hidden

**No server-side pagination needed** -- the API already returns all tags. Pagination is purely client-side: JavaScript hides tags beyond the current limit and reveals more on button click.

## Scope

### In scope
- Grid page NavBar: replace Settings text link + raw name + logout icon with gear icon + avatar pill
- Avatar dropdown with name, auth type, settings link, sign out
- Settings page NavBar: back arrow on left, title "Settings", theme + avatar pill on right
- Settings sidebar: add icons to all 6 nav items, shorten labels
- Tags section: paginate with "load more" (7 initial, 15 per batch)
- Dark mode support for all new components
- Responsive: hide first name on narrow screens, show only initials circle

### Out of scope
- Settings sidebar reordering or new sections
- Tag search/filter within settings
- Avatar image upload (initials only for now)
- Changes to the login page NavBar

## Files to Modify

| File | Changes |
|------|---------|
| `internal/server/templates/layout.html` | No changes to shared layout (header_actions block is per-template) |
| `internal/server/templates/grid.html` | Replace `header_actions` block: gear icon + avatar pill + dropdown HTML |
| `internal/server/templates/settings.html` | Replace `header_actions` block: back arrow in title area, theme + avatar pill. Add icons to sidebar nav. Add tags pagination markup. |
| `internal/server/static/css/app.css` | Avatar pill styles, dropdown styles, sidebar icon styles |
| `internal/server/static/js/grid.js` | Avatar dropdown open/close logic |
| `internal/server/static/js/settings.js` | Avatar dropdown logic (shared pattern) + tags "load more" pagination |

## Testing Considerations

- Dropdown opens on click, closes on outside click and Escape
- Dropdown positions correctly and doesn't overflow viewport
- Avatar initials derived correctly from various DisplayName formats (full name, single name, empty)
- Tags pagination: correct batch sizes, button text updates, button hides when all loaded
- Dark mode: all new elements respect theme tokens
- Responsive: pill collapses gracefully on narrow viewports
- Settings sidebar icons align properly with text
- CSRF token still accessible in dropdown sign-out link
