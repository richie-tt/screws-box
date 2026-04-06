package session

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSession(id, username string) *Session {
	now := time.Now()
	return &Session{
		ID:           id,
		Username:     username,
		CSRFToken:    "csrf-" + id,
		CreatedAt:    now,
		LastActivity: now,
	}
}

func TestMemoryStore_Create(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	t.Cleanup(func() { m.Close() })
	ctx := context.Background()

	sess := newTestSession("sess1", "alice")
	err := m.Create(ctx, sess)
	require.NoError(t, err)

	got, err := m.Get(ctx, "sess1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "alice", got.Username)
	assert.Equal(t, "csrf-sess1", got.CSRFToken)
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	t.Cleanup(func() { m.Close() })

	got, err := m.Get(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryStore_Delete(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	t.Cleanup(func() { m.Close() })
	ctx := context.Background()

	sess := newTestSession("sess1", "alice")
	require.NoError(t, m.Create(ctx, sess))

	err := m.Delete(ctx, "sess1")
	require.NoError(t, err)

	got, err := m.Get(ctx, "sess1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryStore_Touch(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	t.Cleanup(func() { m.Close() })
	ctx := context.Background()

	sess := newTestSession("sess1", "alice")
	sess.LastActivity = time.Now().Add(-10 * time.Minute)
	require.NoError(t, m.Create(ctx, sess))

	err := m.Touch(ctx, "sess1")
	require.NoError(t, err)

	got, err := m.Get(ctx, "sess1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.WithinDuration(t, time.Now(), got.LastActivity, 2*time.Second)
}

func TestMemoryStore_Expiry(t *testing.T) {
	m := NewMemoryStore(50*time.Millisecond, 5*time.Minute)
	t.Cleanup(func() { m.Close() })
	ctx := context.Background()

	sess := newTestSession("sess1", "alice")
	require.NoError(t, m.Create(ctx, sess))

	time.Sleep(100 * time.Millisecond)

	got, err := m.Get(ctx, "sess1")
	require.NoError(t, err)
	assert.Nil(t, got, "expired session should return nil")
}

func TestMemoryStore_SlidingWindow(t *testing.T) {
	m := NewMemoryStore(100*time.Millisecond, 5*time.Minute)
	t.Cleanup(func() { m.Close() })
	ctx := context.Background()

	sess := newTestSession("sess1", "alice")
	require.NoError(t, m.Create(ctx, sess))

	// Touch at 60ms to reset the sliding window
	time.Sleep(60 * time.Millisecond)
	require.NoError(t, m.Touch(ctx, "sess1"))

	// At 120ms from creation but only 60ms from last touch — should still be alive
	time.Sleep(60 * time.Millisecond)
	got, err := m.Get(ctx, "sess1")
	require.NoError(t, err)
	assert.NotNil(t, got, "session should survive due to Touch extending TTL")
}

func TestMemoryStore_BackgroundCleanup(t *testing.T) {
	// Use very short TTL and cleanup interval
	m := NewMemoryStore(50*time.Millisecond, 100*time.Millisecond)
	t.Cleanup(func() { m.Close() })
	ctx := context.Background()

	sess := newTestSession("sess1", "alice")
	require.NoError(t, m.Create(ctx, sess))

	// Wait for expiry + cleanup sweep
	time.Sleep(250 * time.Millisecond)

	// Session should be removed by background cleanup
	m.mu.RLock()
	_, exists := m.sessions["sess1"]
	m.mu.RUnlock()
	assert.False(t, exists, "background cleanup should have removed expired session")
}

func TestMemoryStore_Close(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	err := m.Close()
	require.NoError(t, err)
	// Should not panic on double close — but we only call once.
	// Just verify no goroutine leak by completing the test.
}

func TestMemoryStore_Close_ReturnsError(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	err := m.Close()
	assert.NoError(t, err, "Close should return nil error")
}

func TestMemoryStore_DeleteByAuthMethod(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	t.Cleanup(func() { m.Close() })
	ctx := context.Background()

	// Create 2 local + 1 oidc session
	local1 := newTestSession("local1", "alice")
	local1.AuthMethod = "local"
	local2 := newTestSession("local2", "bob")
	local2.AuthMethod = "local"
	oidc1 := newTestSession("oidc1", "carol")
	oidc1.AuthMethod = "oidc"

	require.NoError(t, m.Create(ctx, local1))
	require.NoError(t, m.Create(ctx, local2))
	require.NoError(t, m.Create(ctx, oidc1))

	count, err := m.DeleteByAuthMethod(ctx, "oidc")
	require.NoError(t, err)
	assert.Equal(t, 1, count, "should delete 1 oidc session")

	// Local sessions should remain
	got, err := m.Get(ctx, "local1")
	require.NoError(t, err)
	assert.NotNil(t, got)

	got, err = m.Get(ctx, "local2")
	require.NoError(t, err)
	assert.NotNil(t, got)

	// OIDC session should be gone
	got, err = m.Get(ctx, "oidc1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryStore_DeleteByAuthMethod_Empty(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	t.Cleanup(func() { m.Close() })

	count, err := m.DeleteByAuthMethod(context.Background(), "oidc")
	require.NoError(t, err)
	assert.Equal(t, 0, count, "empty store should return 0")
}

func TestMemoryStore_DeleteByAuthMethod_LocalOnly(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	t.Cleanup(func() { m.Close() })
	ctx := context.Background()

	local1 := newTestSession("local1", "alice")
	local1.AuthMethod = "local"
	oidc1 := newTestSession("oidc1", "bob")
	oidc1.AuthMethod = "oidc"
	oidc2 := newTestSession("oidc2", "carol")
	oidc2.AuthMethod = "oidc"

	require.NoError(t, m.Create(ctx, local1))
	require.NoError(t, m.Create(ctx, oidc1))
	require.NoError(t, m.Create(ctx, oidc2))

	count, err := m.DeleteByAuthMethod(ctx, "local")
	require.NoError(t, err)
	assert.Equal(t, 1, count, "should delete 1 local session")

	// OIDC sessions should remain
	got, err := m.Get(ctx, "oidc1")
	require.NoError(t, err)
	assert.NotNil(t, got)

	got, err = m.Get(ctx, "oidc2")
	require.NoError(t, err)
	assert.NotNil(t, got)
}

func TestMemoryStore_List(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	t.Cleanup(func() { m.Close() })
	ctx := context.Background()

	// Create 3 sessions, one with expired LastActivity
	s1 := newTestSession("s1", "alice")
	s2 := newTestSession("s2", "bob")
	s3 := newTestSession("s3", "carol")
	s3.LastActivity = time.Now().Add(-2 * time.Hour) // expired

	require.NoError(t, m.Create(ctx, s1))
	require.NoError(t, m.Create(ctx, s2))
	require.NoError(t, m.Create(ctx, s3))

	sessions, err := m.List(ctx)
	require.NoError(t, err)
	assert.Len(t, sessions, 2, "should return only non-expired sessions")

	usernames := map[string]bool{}
	for _, s := range sessions {
		usernames[s.Username] = true
	}
	assert.True(t, usernames["alice"])
	assert.True(t, usernames["bob"])
	assert.False(t, usernames["carol"], "expired session should not be listed")
}

func TestMemoryStore_List_Empty(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	t.Cleanup(func() { m.Close() })

	sessions, err := m.List(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, sessions, "should return empty slice, not nil")
	assert.Len(t, sessions, 0)
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	m := NewMemoryStore(time.Hour, 5*time.Minute)
	t.Cleanup(func() { m.Close() })
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "sess-" + time.Now().String() + "-" + string(rune('A'+n%26))
			sess := newTestSession(id, "user")
			_ = m.Create(ctx, sess)
			_, _ = m.Get(ctx, id)
			_ = m.Touch(ctx, id)
			_ = m.Delete(ctx, id)
		}(i)
	}
	wg.Wait()
}
