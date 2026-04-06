package session

import "time"

// Session holds per-session state on the server side.
type Session struct {
	ID           string
	Username     string
	AuthMethod   string // "local" or "oidc"
	DisplayName  string
	CSRFToken    string
	CreatedAt    time.Time
	LastActivity time.Time
}
