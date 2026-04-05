package session

import (
	"context"
	"sync"
	"time"
)

// MemoryStore is an in-memory Store backed by a map with RWMutex.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
	done     chan struct{}
}

// NewMemoryStore creates a MemoryStore and starts background cleanup.
// cleanupInterval controls how often the background sweep runs.
// Call Close() to stop the cleanup goroutine.
func NewMemoryStore(ttl, cleanupInterval time.Duration) *MemoryStore {
	m := &MemoryStore{
		sessions: make(map[string]*Session),
		ttl:      ttl,
		done:     make(chan struct{}),
	}
	go m.cleanup(cleanupInterval)
	return m
}

// Create stores a session.
func (m *MemoryStore) Create(_ context.Context, sess *Session) error {
	m.mu.Lock()
	m.sessions[sess.ID] = sess
	m.mu.Unlock()
	return nil
}

// Get retrieves a session by ID. Returns nil, nil if not found or expired.
// Expired sessions are lazily deleted.
func (m *MemoryStore) Get(ctx context.Context, id string) (*Session, error) {
	m.mu.RLock()
	sess, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	if time.Since(sess.LastActivity) > m.ttl {
		_ = m.Delete(ctx, id)
		return nil, nil
	}
	return sess, nil
}

// Delete removes a session by ID.
func (m *MemoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
	return nil
}

// Touch updates the LastActivity timestamp for sliding window expiry.
func (m *MemoryStore) Touch(_ context.Context, id string) error {
	m.mu.Lock()
	if sess, ok := m.sessions[id]; ok {
		sess.LastActivity = time.Now()
	}
	m.mu.Unlock()
	return nil
}

// Close stops the background cleanup goroutine.
func (m *MemoryStore) Close() {
	close(m.done)
}

// cleanup periodically removes expired sessions.
func (m *MemoryStore) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.mu.Lock()
			now := time.Now()
			for id, sess := range m.sessions {
				if now.Sub(sess.LastActivity) > m.ttl {
					delete(m.sessions, id)
				}
			}
			m.mu.Unlock()
		case <-m.done:
			return
		}
	}
}
