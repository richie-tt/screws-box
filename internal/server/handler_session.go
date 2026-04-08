package server

import (
	"fmt"
	"net/http"
	"regexp"
	"screws-box/internal/session"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
)

// SessionInfo is the JSON response for a single session in the admin API.
// It intentionally omits CSRFToken to prevent information disclosure (T-16-06).
type SessionInfo struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	AuthMethod   string `json:"auth_method"`
	DisplayName  string `json:"display_name"`
	CreatedAt    string `json:"created_at"`
	LastActivity string `json:"last_activity"`
	ExpiresIn    string `json:"expires_in"`
	IsCurrent    bool   `json:"is_current"`
}

// sessionIDPattern validates session IDs: exactly 64 hex characters (256-bit).
var sessionIDPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// formatExpiresIn returns a human-readable relative time until session expiry.
func formatExpiresIn(lastActivity time.Time, ttl time.Duration) string {
	remaining := ttl - time.Since(lastActivity)
	if remaining <= 0 {
		return "expired"
	}
	if remaining >= time.Hour {
		return fmt.Sprintf("%dh", int(remaining.Hours()))
	}
	if remaining >= time.Minute {
		return fmt.Sprintf("%dm", int(remaining.Minutes()))
	}
	return "<1m"
}

// mapSessionToInfo converts a session.Session to the API response type.
func mapSessionToInfo(s *session.Session, currentID string, ttl time.Duration) SessionInfo {
	return SessionInfo{
		ID:           s.ID,
		Username:     s.Username,
		AuthMethod:   s.AuthMethod,
		DisplayName:  s.DisplayName,
		CreatedAt:    s.CreatedAt.Format("2006-01-02 15:04"),
		LastActivity: s.LastActivity.Format("2006-01-02 15:04"),
		ExpiresIn:    formatExpiresIn(s.LastActivity, ttl),
		IsCurrent:    s.ID == currentID,
	}
}

// sortSessionInfos sorts sessions: current first, then by LastActivity descending.
func sortSessionInfos(infos []SessionInfo) {
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].IsCurrent != infos[j].IsCurrent {
			return infos[i].IsCurrent
		}
		return infos[i].LastActivity > infos[j].LastActivity
	})
}

// handleListSessions returns all active sessions as JSON.
// GET /api/sessions
func (srv *Server) handleListSessions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentSess := srv.sessions.GetSession(r)
		currentID := ""
		if currentSess != nil {
			currentID = currentSess.ID
		}

		sessions, err := srv.sessions.ListSessions(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list sessions"})
			return
		}

		infos := make([]SessionInfo, 0, len(sessions))
		for _, s := range sessions {
			infos = append(infos, mapSessionToInfo(s, currentID, srv.sessions.TTL()))
		}
		sortSessionInfos(infos)

		writeJSON(w, http.StatusOK, infos)
	}
}

// handleRevokeSession revokes a single session by ID.
// DELETE /api/sessions/{sessionID}
func (srv *Server) handleRevokeSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")

		// Validate format: must be 64 hex chars (T-16-07)
		if !sessionIDPattern.MatchString(sessionID) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid session ID format"})
			return
		}

		// Prevent self-revocation (T-16-05 / D-18)
		currentSess := srv.sessions.GetSession(r)
		if currentSess != nil && currentSess.ID == sessionID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot revoke your own session"})
			return
		}

		if err := srv.sessions.DeleteSession(r.Context(), sessionID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to revoke session"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// handleRevokeAllOthers revokes all sessions except the current user's.
// DELETE /api/sessions
func (srv *Server) handleRevokeAllOthers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentSess := srv.sessions.GetSession(r)
		currentID := ""
		if currentSess != nil {
			currentID = currentSess.ID
		}

		sessions, err := srv.sessions.ListSessions(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list sessions"})
			return
		}

		count := 0
		for _, s := range sessions {
			if s.ID != currentID {
				if err := srv.sessions.DeleteSession(r.Context(), s.ID); err != nil {
					continue
				}
				count++
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": count})
	}
}
