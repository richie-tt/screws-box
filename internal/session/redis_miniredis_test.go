package session

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMiniredisStore spins up an in-process Redis server and returns a connected
// RedisStore plus the underlying miniredis instance for direct manipulation
// (e.g. injecting fault data, fast-forwarding time).
func newMiniredisStore(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	store, err := NewRedisStore("redis://"+mr.Addr(), time.Hour)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store, mr
}

func mkSession(id, user, method string) *Session {
	return &Session{
		ID:           id,
		Username:     user,
		AuthMethod:   method,
		CSRFToken:    "csrf-" + id,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}
}

// --- Happy-path coverage --------------------------------------------------

func TestRedisStoreCreateGet(t *testing.T) {
	store, _ := newMiniredisStore(t)
	ctx := context.Background()

	sess := mkSession("abc", "alice", "local")
	require.NoError(t, store.Create(ctx, sess))

	got, err := store.Get(ctx, "abc")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "alice", got.Username)
	assert.Equal(t, "local", got.AuthMethod)
}

func TestRedisStoreGetMissing(t *testing.T) {
	store, _ := newMiniredisStore(t)
	got, err := store.Get(context.Background(), "no-such-id")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRedisStoreDelete(t *testing.T) {
	store, _ := newMiniredisStore(t)
	ctx := context.Background()
	require.NoError(t, store.Create(ctx, mkSession("d1", "u", "local")))
	require.NoError(t, store.Delete(ctx, "d1"))

	got, err := store.Get(ctx, "d1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRedisStoreTouch(t *testing.T) {
	store, _ := newMiniredisStore(t)
	ctx := context.Background()
	sess := mkSession("t1", "u", "local")
	old := time.Now().Add(-time.Hour)
	sess.LastActivity = old
	require.NoError(t, store.Create(ctx, sess))

	require.NoError(t, store.Touch(ctx, "t1"))

	got, err := store.Get(ctx, "t1")
	require.NoError(t, err)
	assert.True(t, got.LastActivity.After(old), "Touch should update LastActivity")
}

func TestRedisStoreTouchMissingIsNoOp(t *testing.T) {
	store, _ := newMiniredisStore(t)
	require.NoError(t, store.Touch(context.Background(), "no-such-id"))
}

func TestRedisStoreList(t *testing.T) {
	store, _ := newMiniredisStore(t)
	ctx := context.Background()
	require.NoError(t, store.Create(ctx, mkSession("l1", "a", "local")))
	require.NoError(t, store.Create(ctx, mkSession("l2", "b", "oidc")))

	sessions, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
}

func TestRedisStoreDeleteByAuthMethod(t *testing.T) {
	store, _ := newMiniredisStore(t)
	ctx := context.Background()
	require.NoError(t, store.Create(ctx, mkSession("a1", "u1", "local")))
	require.NoError(t, store.Create(ctx, mkSession("a2", "u2", "local")))
	require.NoError(t, store.Create(ctx, mkSession("a3", "u3", "oidc")))

	n, err := store.DeleteByAuthMethod(ctx, "local")
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	remaining, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, "u3", remaining[0].Username)
}

// --- Error / corruption paths ---------------------------------------------

// Get returns a wrapped error when the stored value isn't valid JSON,
// hitting the json.Unmarshal branch in Get.
func TestRedisStoreGetCorruptValue(t *testing.T) {
	store, mr := newMiniredisStore(t)
	require.NoError(t, mr.Set(keyPrefix+"corrupt", "not-json-payload"))

	_, err := store.Get(context.Background(), "corrupt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

// Touch on a corrupt value hits the json.Unmarshal branch in Touch.
func TestRedisStoreTouchCorruptValue(t *testing.T) {
	store, mr := newMiniredisStore(t)
	require.NoError(t, mr.Set(keyPrefix+"corrupt", "not-json-payload"))

	err := store.Touch(context.Background(), "corrupt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

// List skips corrupt entries silently and returns the rest.
func TestRedisStoreListSkipsCorrupt(t *testing.T) {
	store, mr := newMiniredisStore(t)
	require.NoError(t, mr.Set(keyPrefix+"corrupt", "not-json"))
	require.NoError(t, store.Create(context.Background(), mkSession("ok", "u", "local")))

	sessions, err := store.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "u", sessions[0].Username)
}

// DeleteByAuthMethod skips corrupt entries.
func TestRedisStoreDeleteByAuthMethodSkipsCorrupt(t *testing.T) {
	store, mr := newMiniredisStore(t)
	require.NoError(t, mr.Set(keyPrefix+"corrupt", "not-json"))
	require.NoError(t, store.Create(context.Background(), mkSession("ok", "u", "local")))

	n, err := store.DeleteByAuthMethod(context.Background(), "local")
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

// Operations on a closed RedisStore must return errors (covers err-paths
// that depend on the underlying redis client failing).

func TestRedisStoreClosedClientCreate(t *testing.T) {
	store, _ := newMiniredisStore(t)
	require.NoError(t, store.Close())

	err := store.Create(context.Background(), mkSession("z", "u", "local"))
	require.Error(t, err)
}

func TestRedisStoreClosedClientGet(t *testing.T) {
	store, _ := newMiniredisStore(t)
	require.NoError(t, store.Close())

	_, err := store.Get(context.Background(), "z")
	require.Error(t, err)
}

func TestRedisStoreClosedClientDelete(t *testing.T) {
	store, _ := newMiniredisStore(t)
	require.NoError(t, store.Close())

	err := store.Delete(context.Background(), "z")
	require.Error(t, err)
}

func TestRedisStoreClosedClientTouch(t *testing.T) {
	store, _ := newMiniredisStore(t)
	require.NoError(t, store.Close())

	err := store.Touch(context.Background(), "z")
	require.Error(t, err)
}

func TestRedisStoreClosedClientList(t *testing.T) {
	store, _ := newMiniredisStore(t)
	require.NoError(t, store.Close())

	_, err := store.List(context.Background())
	require.Error(t, err)
}

func TestRedisStoreClosedClientDeleteByAuthMethod(t *testing.T) {
	store, _ := newMiniredisStore(t)
	require.NoError(t, store.Close())

	_, err := store.DeleteByAuthMethod(context.Background(), "local")
	require.Error(t, err)
}

// NewRedisStore must error when the URL is parseable but Redis is
// unreachable. We close miniredis to simulate a down server.
func TestNewRedisStoreUnreachable(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()

	_, err := NewRedisStore("redis://"+addr, time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unreachable")
}

// NewRedisStore must succeed against a healthy server.
func TestNewRedisStoreHappy(t *testing.T) {
	mr := miniredis.RunT(t)
	store, err := NewRedisStore("redis://"+mr.Addr(), time.Hour)
	require.NoError(t, err)
	require.NotNil(t, store)
	_ = store.Close()
}
