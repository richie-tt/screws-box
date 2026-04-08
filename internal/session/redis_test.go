package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRedisStore_BadURL(t *testing.T) {
	_, err := NewRedisStore("not-a-url", time.Hour)
	require.Error(t, err, "should fail with invalid URL")
	assert.Contains(t, err.Error(), "invalid REDIS_URL")
}

func TestRedisStore_InterfaceCompliance(_ *testing.T) {
	// Compile-time check — if this compiles, the interface is satisfied.
	var _ Store = (*RedisStore)(nil)
}
