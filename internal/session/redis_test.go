package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRedisStore_BadURL(t *testing.T) {
	_, err := NewRedisStore("not-a-url", time.Hour)
	assert.Error(t, err, "should fail with invalid URL")
	assert.Contains(t, err.Error(), "invalid REDIS_URL")
}

func TestRedisStore_InterfaceCompliance(t *testing.T) {
	// Compile-time check — if this compiles, the interface is satisfied.
	var _ Store = (*RedisStore)(nil)
}
