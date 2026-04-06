package session

import "context"

// Store defines the session persistence operations.
// All methods take context.Context as first parameter
// for compatibility with network-backed stores (Redis in Phase 16).
type Store interface {
	Create(ctx context.Context, sess *Session) error
	Get(ctx context.Context, id string) (*Session, error)
	Delete(ctx context.Context, id string) error
	Touch(ctx context.Context, id string) error
	DeleteByAuthMethod(ctx context.Context, method string) (int, error)
}
