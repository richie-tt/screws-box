package session

import "time"

// Session holds per-session state on the server side.
type Session struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	AuthMethod   string    `json:"auth_method"` // "local" or "oidc"
	DisplayName  string    `json:"display_name"`
	CSRFToken    string    `json:"csrf_token"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
}
