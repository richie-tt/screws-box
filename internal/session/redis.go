package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var _ Store = (*RedisStore)(nil)

const keyPrefix = "session:"

// RedisStore implements Store using Redis as the backend.
// Sessions are stored as JSON with Redis EXPIRE for TTL management.
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisStore creates a RedisStore and pings Redis to verify connectivity.
// Returns an error if the URL is invalid or Redis is unreachable.
func NewRedisStore(redisURL string, ttl time.Duration) (*RedisStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_URL: %w", err)
	}
	client := redis.NewClient(opts)
	// Fail fast if Redis is unreachable (D-06).
	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis unreachable: %w", err)
	}
	return &RedisStore{client: client, ttl: ttl}, nil
}

// Create stores a session in Redis with the configured TTL.
func (r *RedisStore) Create(ctx context.Context, sess *Session) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return r.client.Set(ctx, keyPrefix+sess.ID, data, r.ttl).Err()
}

// Get retrieves a session by ID. Returns nil, nil if not found.
func (r *RedisStore) Get(ctx context.Context, id string) (*Session, error) {
	data, err := r.client.Get(ctx, keyPrefix+id).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &sess, nil
}

// Delete removes a session by ID.
func (r *RedisStore) Delete(ctx context.Context, id string) error {
	return r.client.Del(ctx, keyPrefix+id).Err()
}

// Touch updates the LastActivity timestamp and refreshes the Redis TTL.
// Gracefully handles already-expired sessions (redis.Nil).
func (r *RedisStore) Touch(ctx context.Context, id string) error {
	data, err := r.client.Get(ctx, keyPrefix+id).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil // session already gone
		}
		return fmt.Errorf("touch get session: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return fmt.Errorf("touch unmarshal session: %w", err)
	}
	sess.LastActivity = time.Now()
	updated, err := json.Marshal(&sess)
	if err != nil {
		return fmt.Errorf("touch marshal session: %w", err)
	}
	return r.client.Set(ctx, keyPrefix+id, updated, r.ttl).Err()
}

// DeleteByAuthMethod removes all sessions with the given auth method.
// Uses SCAN to iterate over session keys. Returns the count of deleted sessions.
func (r *RedisStore) DeleteByAuthMethod(ctx context.Context, method string) (int, error) {
	count := 0
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, keyPrefix+"*", 100).Result()
		if err != nil {
			return count, fmt.Errorf("scan sessions: %w", err)
		}
		for _, key := range keys {
			data, err := r.client.Get(ctx, key).Bytes()
			if err != nil {
				if err == redis.Nil {
					continue // expired between SCAN and GET
				}
				return count, fmt.Errorf("get session for delete: %w", err)
			}
			var sess Session
			if err := json.Unmarshal(data, &sess); err != nil {
				continue // skip corrupt entries
			}
			if sess.AuthMethod == method {
				if err := r.client.Del(ctx, key).Err(); err != nil {
					return count, fmt.Errorf("delete session: %w", err)
				}
				count++
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return count, nil
}

// List returns all active sessions stored in Redis.
// Uses SCAN to iterate over session keys. Skips expired keys gracefully.
func (r *RedisStore) List(ctx context.Context) ([]*Session, error) {
	result := []*Session{}
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, keyPrefix+"*", 100).Result()
		if err != nil {
			return result, fmt.Errorf("scan sessions: %w", err)
		}
		for _, key := range keys {
			data, err := r.client.Get(ctx, key).Bytes()
			if err != nil {
				if err == redis.Nil {
					continue // expired between SCAN and GET
				}
				return result, fmt.Errorf("get session for list: %w", err)
			}
			var sess Session
			if err := json.Unmarshal(data, &sess); err != nil {
				continue // skip corrupt entries
			}
			result = append(result, &sess)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return result, nil
}

// Close closes the Redis client connection.
func (r *RedisStore) Close() error {
	return r.client.Close()
}
