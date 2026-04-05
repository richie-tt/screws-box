package server

import (
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// --- Rate Limiting ---

type ipLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rateBucket
	rate     rate.Limit
	burst    int
}

type rateBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPLimiter(r rate.Limit, burst int) *ipLimiter {
	ipl := &ipLimiter{
		limiters: make(map[string]*rateBucket),
		rate:     r,
		burst:    burst,
	}
	go ipl.cleanup()
	return ipl
}

func (ipl *ipLimiter) get(ip string) *rate.Limiter {
	ipl.mu.Lock()
	defer ipl.mu.Unlock()

	if b, ok := ipl.limiters[ip]; ok {
		b.lastSeen = time.Now()
		return b.limiter
	}

	limiter := rate.NewLimiter(ipl.rate, ipl.burst)
	ipl.limiters[ip] = &rateBucket{limiter: limiter, lastSeen: time.Now()}
	return limiter
}

func (ipl *ipLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		ipl.mu.Lock()
		for ip, b := range ipl.limiters {
			if time.Since(b.lastSeen) > 10*time.Minute {
				delete(ipl.limiters, ip)
			}
		}
		ipl.mu.Unlock()
	}
}

func clientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// newRateLimitAPI creates middleware that rate-limits API requests per IP.
func newRateLimitAPI() func(http.Handler) http.Handler {
	limiter := newIPLimiter(5, 10) // 5 req/s, burst 10
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.get(clientIP(r)).Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// newRateLimitLogin creates middleware that rate-limits login attempts per IP.
func newRateLimitLogin() func(http.Handler) http.Handler {
	limiter := newIPLimiter(0.5, 5) // 1 attempt per 2s, burst 5
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.get(clientIP(r)).Allow() {
				slog.Warn("login rate limit exceeded", "ip", clientIP(r)) //nolint:gosec // G706: structured logging, IP is a key-value pair
				http.Error(w, "too many login attempts, try again later", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- CSRF Protection ---

const csrfTokenName = "csrf_token"

// csrfProtect creates middleware that validates CSRF tokens on state-changing requests.
// For API endpoints (JSON), it checks the X-CSRF-Token header.
// For form submissions, it checks the csrf_token form field.
// GET/HEAD/OPTIONS requests are exempt.
// When auth is disabled, CSRF is skipped (no session to bind tokens to).
func (srv *Server) csrfProtect() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read-only methods are exempt.
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Skip CSRF when auth is disabled — no session to bind tokens to.
			settings, err := srv.store.GetAuthSettings(r.Context())
			if err != nil || !settings.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Validate CSRF: the X-CSRF-Token header must match the server-side CSRF token.
			expected := srv.sessions.GetCSRFToken(r)
			if expected == "" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			// Check header first (API/htmx), then form field.
			token := r.Header.Get("X-CSRF-Token")
			if token == "" {
				token = r.FormValue(csrfTokenName) //nolint:gosec // G120: CSRF token is a short string, body already limited upstream
			}

			if token != expected {
				http.Error(w, "forbidden: invalid CSRF token", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
