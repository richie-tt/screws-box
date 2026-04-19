# NavBar Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the NavBar for visual consistency -- replace mixed text/icon links with a uniform icon system, add avatar pill with dropdown, add settings sidebar icons, and paginate the tags section.

**Architecture:** Pure frontend changes to Go HTML templates, CSS, and vanilla JS. One backend change: pass `AuthMethod` from the session to both template data structs so the dropdown can show "Local admin" or the OIDC provider name.

**Tech Stack:** Go html/template, vanilla CSS (design tokens from app.css), vanilla JS (IIFE pattern matching existing code)

---

### Task 1: Pass AuthMethod to Template Data

**Files:**
- Modify: `internal/server/handler.go:224-255` (getDisplayName, handleGrid)
- Modify: `internal/server/handler.go:258-334` (SettingsData, handleSettings)
- Modify: `internal/model/model.go:47-58` (GridData struct)

- [ ] **Step 1: Add AuthMethod field to GridData**

In `internal/model/model.go`, add `AuthMethod` to the `GridData` struct:

```go
// GridData is the view model for the grid template.
type GridData struct {
	ShelfName       string
	Rows            int
	Cols            int
	ColNumbers      []int
	Grid            []Row
	Error           string
	AuthEnabled     bool
	AuthUser        string
	AuthHasPassword bool
	DisplayName     string
	AuthMethod      string
}
```

- [ ] **Step 2: Add AuthMethod field to SettingsData**

In `internal/server/handler.go`, add `AuthMethod` to `SettingsData`:

```go
type SettingsData struct {
	ShelfName        string
	Rows             int
	Cols             int
	AuthEnabled      bool
	AuthUser         string
	AuthHasPassword  bool
	DisplayName      string
	AuthMethod       string
	OIDCEnabled      bool
	OIDCIssuer       string
	OIDCClientID     string
	OIDCDisplayName  string
	OIDCSecretStatus string
	SessionStoreType string
	SessionCount     int
	Sessions         []SessionInfo
	CurrentSessionID string
	SessionTTL       int64
}
```

- [ ] **Step 3: Create getAuthMethod helper and update handlers**

In `internal/server/handler.go`, add a helper below `getDisplayName`:

```go
// getAuthMethod returns the auth method for the current session ("local" or "oidc").
func (srv *Server) getAuthMethod(r *http.Request) string {
	sess := srv.sessions.GetSession(r)
	if sess == nil {
		return ""
	}
	return sess.AuthMethod
}
```

In `handleGrid`, after `data.DisplayName = srv.getDisplayName(r)`, add:

```go
data.AuthMethod = srv.getAuthMethod(r)
```

In `handleSettings`, after `DisplayName: srv.getDisplayName(r),`, add:

```go
AuthMethod: srv.getAuthMethod(r),
```

- [ ] **Step 4: Run tests to verify nothing breaks**

Run: `go test ./... -count=1`
Expected: All tests pass (struct changes are additive, zero-value "" is safe)

- [ ] **Step 5: Commit**

```bash
git add internal/model/model.go internal/server/handler.go
git commit -m "feat: pass AuthMethod to grid and settings templates"
```

---

### Task 2: Avatar Pill and Dropdown CSS

**Files:**
- Modify: `internal/server/static/css/app.css` (append new styles)

- [ ] **Step 1: Add avatar pill styles to app.css**

Append to the end of `internal/server/static/css/app.css`:

```css
/* --- Avatar Pill --- */
.avatar-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px 4px 4px;
  border: 1px solid var(--border);
  border-radius: 20px;
  cursor: pointer;
  background: transparent;
  color: var(--text);
  font-family: inherit;
  font-size: 13px;
  line-height: 1;
  transition: border-color 0.15s, background-color 0.15s;
  position: relative;
}
.avatar-pill:hover {
  border-color: var(--border-strong);
  background: var(--bg-inset);
}

.avatar-initials {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: var(--accent);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.avatar-name {
  font-weight: 400;
}

.avatar-chevron {
  flex-shrink: 0;
  transition: transform 0.15s;
}
.avatar-pill[aria-expanded="true"] .avatar-chevron {
  transform: rotate(180deg);
}

/* Hide name on narrow screens, keep initials */
@media (max-width: 480px) {
  .avatar-name { display: none; }
  .avatar-pill { padding: 3px; gap: 0; }
  .avatar-chevron { display: none; }
}

/* --- Avatar Dropdown --- */
.avatar-dropdown-wrap {
  position: relative;
}

.avatar-dropdown {
  display: none;
  position: absolute;
  right: 0;
  top: calc(100% + 6px);
  width: 220px;
  background: var(--bg-raised);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: var(--shadow-md);
  z-index: 100;
  overflow: hidden;
}
.avatar-dropdown[data-open="true"] {
  display: block;
}

.avatar-dropdown-header {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}
.avatar-dropdown-header .avatar-full-name {
  font-weight: 600;
  font-size: 14px;
}
.avatar-dropdown-header .avatar-auth-type {
  font-size: 12px;
  color: var(--text-muted);
}

.avatar-dropdown-menu {
  padding: 4px;
}
.avatar-dropdown-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 4px;
  text-decoration: none;
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
}
.avatar-dropdown-item:hover {
  background: var(--bg-inset);
  text-decoration: none;
}
.avatar-dropdown-item svg {
  flex-shrink: 0;
}
.avatar-dropdown-divider {
  height: 1px;
  background: var(--border);
  margin: 4px 12px;
}
.avatar-dropdown-item.danger {
  color: var(--danger);
}
```

- [ ] **Step 2: Verify CSS parses correctly**

Run: `go run -tags dev ./cmd/screwsbox` and open http://localhost:8080 in a browser. Confirm the page loads without CSS errors in the browser console. Stop the server.

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/css/app.css
git commit -m "feat: add avatar pill and dropdown CSS styles"
```

---

### Task 3: Grid Page NavBar Template

**Files:**
- Modify: `internal/server/templates/grid.html:10-22` (header_actions block)

- [ ] **Step 1: Replace the header_actions block in grid.html**

Replace the entire `{{define "header_actions"}}...{{end}}` block (lines 10-22) with:

```html
{{define "header_actions"}}
<a href="/settings" class="ghost" aria-label="Settings" title="Settings">
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="3"/>
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
    </svg>
</a>
{{if .AuthEnabled}}
<div class="avatar-dropdown-wrap">
    <button class="avatar-pill" aria-expanded="false" aria-haspopup="true" id="avatar-pill">
        <span class="avatar-initials" data-display-name="{{.DisplayName}}"></span>
        <span class="avatar-name">{{.DisplayName}}</span>
        <svg class="avatar-chevron" width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2.5 4L5 6.5 7.5 4"/></svg>
    </button>
    <div class="avatar-dropdown" id="avatar-dropdown">
        <div class="avatar-dropdown-header">
            <div class="avatar-full-name">{{.DisplayName}}</div>
            <div class="avatar-auth-type">{{if eq .AuthMethod "oidc"}}OIDC{{else}}Local admin{{end}}</div>
        </div>
        <div class="avatar-dropdown-menu">
            <a href="/settings" class="avatar-dropdown-item">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
                Settings
            </a>
            <div class="avatar-dropdown-divider"></div>
            <a href="/logout" class="avatar-dropdown-item danger">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
                Sign out
            </a>
        </div>
    </div>
</div>
{{end}}
{{end}}
```

- [ ] **Step 2: Verify template parses**

Run: `go build ./cmd/screwsbox`
Expected: Builds successfully (no template parse errors at build time; runtime check in next step)

- [ ] **Step 3: Commit**

```bash
git add internal/server/templates/grid.html
git commit -m "feat: grid NavBar with gear icon and avatar pill dropdown"
```

---

### Task 4: Avatar Dropdown JavaScript (grid.js)

**Files:**
- Modify: `internal/server/static/js/grid.js` (append avatar logic at end of IIFE, before closing `})()`)

- [ ] **Step 1: Add avatar initials derivation and dropdown logic to grid.js**

Find the last line of grid.js (should be `})();`). Insert the following block just before it:

```javascript
  // --- Avatar Pill: Initials + Dropdown ---

  (function initAvatarDropdown() {
    var pill = document.getElementById('avatar-pill');
    if (!pill) return;

    // Derive initials from display name
    var initialsEl = pill.querySelector('.avatar-initials');
    if (initialsEl) {
      var name = initialsEl.getAttribute('data-display-name') || '';
      var parts = name.trim().split(/\s+/);
      var initials;
      if (parts.length >= 2) {
        initials = parts[0][0] + parts[parts.length - 1][0];
      } else if (parts[0] && parts[0].length >= 2) {
        initials = parts[0].substring(0, 2);
      } else {
        initials = parts[0] ? parts[0][0] : '?';
      }
      initialsEl.textContent = initials.toUpperCase();
    }

    // Shorten name to first name only
    var nameEl = pill.querySelector('.avatar-name');
    if (nameEl) {
      var fullName = nameEl.textContent.trim();
      var firstName = fullName.split(/\s+/)[0];
      nameEl.textContent = firstName;
    }

    var dropdown = document.getElementById('avatar-dropdown');
    if (!dropdown) return;

    pill.addEventListener('click', function(e) {
      e.stopPropagation();
      var isOpen = dropdown.getAttribute('data-open') === 'true';
      dropdown.setAttribute('data-open', isOpen ? 'false' : 'true');
      pill.setAttribute('aria-expanded', isOpen ? 'false' : 'true');
    });

    document.addEventListener('click', function(e) {
      if (!pill.contains(e.target) && !dropdown.contains(e.target)) {
        dropdown.setAttribute('data-open', 'false');
        pill.setAttribute('aria-expanded', 'false');
      }
    });

    document.addEventListener('keydown', function(e) {
      if (e.key === 'Escape' && dropdown.getAttribute('data-open') === 'true') {
        dropdown.setAttribute('data-open', 'false');
        pill.setAttribute('aria-expanded', 'false');
        pill.focus();
      }
    });
  })();
```

- [ ] **Step 2: Test in browser**

Run: `go run -tags dev ./cmd/screwsbox`
Open http://localhost:8080. Verify:
1. Theme toggle icon is present
2. Gear icon links to /settings
3. Avatar pill shows initials circle + first name + chevron
4. Clicking pill opens dropdown with full name, auth type, Settings link, Sign out
5. Clicking outside or pressing Escape closes the dropdown
6. Test in both light and dark themes

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/grid.js
git commit -m "feat: avatar dropdown open/close logic on grid page"
```

---

### Task 5: Settings Page NavBar and Sidebar Icons

**Files:**
- Modify: `internal/server/templates/settings.html:1-39` (app_title, header_actions, sidebar nav)

- [ ] **Step 1: Update app_title block for settings**

Replace line 3 (`{{define "app_title"}}Screws Box -- Settings{{end}}`) with:

```html
{{define "app_title"}}
<a href="/" class="ghost settings-back-arrow" aria-label="Back to Grid" title="Back to Grid">
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5"/><polyline points="12 19 5 12 12 5"/></svg>
</a>
Settings
{{end}}
```

- [ ] **Step 2: Replace header_actions block in settings.html**

Replace the `{{define "header_actions"}}...{{end}}` block (lines 10-27) with:

```html
{{define "header_actions"}}
{{if .AuthEnabled}}
<div class="avatar-dropdown-wrap">
    <button class="avatar-pill" aria-expanded="false" aria-haspopup="true" id="avatar-pill">
        <span class="avatar-initials" data-display-name="{{.DisplayName}}"></span>
        <span class="avatar-name">{{.DisplayName}}</span>
        <svg class="avatar-chevron" width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2.5 4L5 6.5 7.5 4"/></svg>
    </button>
    <div class="avatar-dropdown" id="avatar-dropdown">
        <div class="avatar-dropdown-header">
            <div class="avatar-full-name">{{.DisplayName}}</div>
            <div class="avatar-auth-type">{{if eq .AuthMethod "oidc"}}OIDC{{else}}Local admin{{end}}</div>
        </div>
        <div class="avatar-dropdown-menu">
            <a href="/settings" class="avatar-dropdown-item">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
                Settings
            </a>
            <div class="avatar-dropdown-divider"></div>
            <a href="/logout" class="avatar-dropdown-item danger">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
                Sign out
            </a>
        </div>
    </div>
</div>
{{end}}
{{end}}
```

- [ ] **Step 3: Replace sidebar nav with icons**

Replace the `<nav class="settings-nav"...>` block (lines 32-39) with:

```html
        <nav class="settings-nav" aria-label="Settings sections">
            <a href="#shelf" class="settings-nav-item active">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg>
                Shelf
            </a>
            <a href="#tags" class="settings-nav-item">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg>
                Tags
            </a>
            <a href="#housekeeping" class="settings-nav-item">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>
                Housekeeping
            </a>
            <a href="#auth" class="settings-nav-item">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
                Auth
            </a>
            <a href="#data" class="settings-nav-item">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/></svg>
                Data
            </a>
            <a href="#sessions" class="settings-nav-item">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
                Sessions <small class="nav-badge" aria-live="polite">{{.SessionCount}}</small>
            </a>
        </nav>
```

- [ ] **Step 4: Commit**

```bash
git add internal/server/templates/settings.html
git commit -m "feat: settings NavBar with back arrow, avatar pill, and sidebar icons"
```

---

### Task 6: Settings Page CSS (Back Arrow + Sidebar Icons)

**Files:**
- Modify: `internal/server/static/css/app.css` (append styles)
- Modify: `internal/server/static/css/settings.css` (update nav item styles)

- [ ] **Step 1: Add back arrow style to app.css**

Append to `internal/server/static/css/app.css`:

```css
/* --- Settings Back Arrow --- */
.settings-back-arrow {
  color: var(--text-muted);
  display: inline-flex;
  align-items: center;
  margin-right: 4px;
}
.settings-back-arrow:hover {
  color: var(--text);
  text-decoration: none;
}
```

- [ ] **Step 2: Update settings nav item styles for icons**

In `internal/server/static/css/settings.css`, find the `.settings-nav-item` rule and update it to include flex layout for icons. Change `display: block` to flex:

```css
.settings-nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  /* keep all existing properties unchanged (padding, color, border-left, etc.) */
}

.settings-nav-item svg {
  flex-shrink: 0;
  opacity: 0.6;
}
.settings-nav-item.active svg {
  opacity: 1;
}
```

- [ ] **Step 3: Test in browser**

Run: `go run -tags dev ./cmd/screwsbox`
Open http://localhost:8080/settings. Verify:
1. Back arrow appears to the left of "Settings" title
2. Right side has theme toggle + avatar pill (no gear icon)
3. Sidebar items have icons with shortened labels
4. Active sidebar item icon is fully opaque, inactive ones are dimmed

- [ ] **Step 4: Commit**

```bash
git add internal/server/static/css/app.css internal/server/static/css/settings.css
git commit -m "feat: settings page back arrow and sidebar icon styles"
```

---

### Task 7: Avatar Dropdown JavaScript (settings.js)

**Files:**
- Modify: `internal/server/static/js/settings.js` (append avatar logic)

- [ ] **Step 1: Add avatar dropdown logic to settings.js**

Append the following inside the IIFE, before the closing `})();`:

```javascript
  // --- Avatar Pill: Initials + Dropdown ---

  (function initAvatarDropdown() {
    var pill = document.getElementById('avatar-pill');
    if (!pill) return;

    var initialsEl = pill.querySelector('.avatar-initials');
    if (initialsEl) {
      var name = initialsEl.getAttribute('data-display-name') || '';
      var parts = name.trim().split(/\s+/);
      var initials;
      if (parts.length >= 2) {
        initials = parts[0][0] + parts[parts.length - 1][0];
      } else if (parts[0] && parts[0].length >= 2) {
        initials = parts[0].substring(0, 2);
      } else {
        initials = parts[0] ? parts[0][0] : '?';
      }
      initialsEl.textContent = initials.toUpperCase();
    }

    var nameEl = pill.querySelector('.avatar-name');
    if (nameEl) {
      var fullName = nameEl.textContent.trim();
      var firstName = fullName.split(/\s+/)[0];
      nameEl.textContent = firstName;
    }

    var dropdown = document.getElementById('avatar-dropdown');
    if (!dropdown) return;

    pill.addEventListener('click', function(e) {
      e.stopPropagation();
      var isOpen = dropdown.getAttribute('data-open') === 'true';
      dropdown.setAttribute('data-open', isOpen ? 'false' : 'true');
      pill.setAttribute('aria-expanded', isOpen ? 'false' : 'true');
    });

    document.addEventListener('click', function(e) {
      if (!pill.contains(e.target) && !dropdown.contains(e.target)) {
        dropdown.setAttribute('data-open', 'false');
        pill.setAttribute('aria-expanded', 'false');
      }
    });

    document.addEventListener('keydown', function(e) {
      if (e.key === 'Escape' && dropdown.getAttribute('data-open') === 'true') {
        dropdown.setAttribute('data-open', 'false');
        pill.setAttribute('aria-expanded', 'false');
        pill.focus();
      }
    });
  })();
```

- [ ] **Step 2: Test in browser**

Open http://localhost:8080/settings. Verify avatar pill and dropdown work identically to the grid page.

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/settings.js
git commit -m "feat: avatar dropdown logic on settings page"
```

---

### Task 8: Tags Section "Load More" Pagination

**Files:**
- Modify: `internal/server/static/js/settings.js` (modify tags rendering to paginate)

- [ ] **Step 1: Add pagination state and logic to the tags section**

In `settings.js`, find the function that renders the tags table body (it populates `#tags-tbody`). You need to:

1. Add pagination state variables near the top of the IIFE (after the feedback helpers):

```javascript
  var TAGS_INITIAL = 7;
  var TAGS_BATCH = 15;
  var tagsLimit = TAGS_INITIAL;
  var allTags = [];
```

2. After the tags are fetched from the API and sorted, store them in `allTags`:

```javascript
  allTags = tags; // the full sorted array from the API
  tagsLimit = TAGS_INITIAL;
  renderVisibleTags();
```

3. Create the `renderVisibleTags` function that slices the array and creates a "load more" button:

```javascript
  function renderVisibleTags() {
    var tbody = document.getElementById('tags-tbody');
    // Clear existing rows
    while (tbody.firstChild) tbody.removeChild(tbody.firstChild);

    var visible = allTags.slice(0, tagsLimit);
    visible.forEach(function(tag) {
      tbody.appendChild(createTagRow(tag));
    });

    // Manage load-more button
    var existing = document.getElementById('tags-load-more');
    if (existing) existing.remove();

    var remaining = allTags.length - tagsLimit;
    if (remaining > 0) {
      var wrap = document.createElement('div');
      wrap.id = 'tags-load-more';
      wrap.style.cssText = 'margin-top:12px; text-align:center; padding-top:12px; border-top:1px solid var(--border)';
      var btn = document.createElement('button');
      btn.className = 'secondary';
      btn.textContent = 'Show more tags (' + remaining + ' remaining)';
      btn.addEventListener('click', function() {
        tagsLimit += TAGS_BATCH;
        renderVisibleTags();
      });
      wrap.appendChild(btn);
      var tableWrap = document.getElementById('tags-table-wrap');
      tableWrap.parentNode.insertBefore(wrap, tableWrap.nextSibling);
    }
  }
```

4. Extract the existing row-creation logic into a `createTagRow(tag)` function if it isn't already extracted. The function receives a tag object and returns a `<tr>` DOM element. Keep the existing rename/delete button wiring intact.

5. When the tag filter input changes, filter `allTags` and re-render:

```javascript
  // In the existing filter handler, replace direct tbody manipulation with:
  var filtered = allTags.filter(function(tag) {
    return tag.name.toLowerCase().indexOf(query) !== -1;
  });
  // Temporarily swap allTags for rendering, then restore
  var saved = allTags;
  allTags = filtered;
  tagsLimit = filtered.length; // show all when filtering
  renderVisibleTags();
  allTags = saved;
```

- [ ] **Step 2: Test in browser**

Open http://localhost:8080/settings and navigate to the Tags section. Verify:
1. Only 7 tags shown initially (if more than 7 exist)
2. "Show more tags (N remaining)" button appears below the table
3. Clicking loads 15 more, button updates remaining count
4. When all loaded, button disappears and all tags visible
5. Tag filter still works (shows all matching results, no pagination)
6. Rename/delete actions still work on visible tags
7. Sorting still works

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/settings.js
git commit -m "feat: tags section load-more pagination (7 initial, 15 per batch)"
```

---

### Task 9: Final Integration Test and Cleanup

**Files:**
- No new files

- [ ] **Step 1: Run Go tests**

Run: `go test ./... -count=1`
Expected: All tests pass

- [ ] **Step 2: Run linter**

Run: `golangci-lint run ./...`
Expected: No new warnings

- [ ] **Step 3: Full browser test**

Run: `go run -tags dev ./cmd/screwsbox`
Test the complete flow:
1. Grid page: theme toggle, gear icon -> settings, avatar pill -> dropdown -> settings / sign out
2. Settings page: back arrow -> grid, theme toggle, avatar pill + dropdown
3. Settings sidebar: all 6 items have icons, active state highlights correctly
4. Tags: shows 7, load more works, all tags visible after full load
5. Dark mode: toggle theme on both pages, verify all new elements use design tokens
6. Responsive: narrow the browser window, verify avatar pill collapses to initials only

- [ ] **Step 4: Commit any final tweaks**

If any visual adjustments were needed:

```bash
git add -A
git commit -m "fix: navbar redesign visual polish"
```
