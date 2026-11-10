package oauth

import (
	"context"
	"sync"
	"time"
)

const (
	// RefreshMargin is how long before it expires a token is refreshed, so a call made with it still finds it valid.
	RefreshMargin = 5 * time.Minute
	// RefreshBackoff is how long after a failed refresh Backoff keeps its failure, so its callers do not try again.
	RefreshBackoff = 30 * time.Second
)

// Due reports whether a token that expires at expires, nil for never, is due for a refresh: it expires within
// RefreshMargin.
func Due(expires *time.Time) bool { return expires != nil && time.Until(*expires) < RefreshMargin }

// Gates lets one caller per key through at a time, such as one refresh of a user's token, and those that wait for
// it hold nothing meanwhile, such as a pooled connection. The zero value is ready. Do not copy one once it is in use.
// shortcut: a gate is never freed, one small channel per key used since start; prune if keys number in the millions.
type Gates[K comparable] struct {
	gates sync.Map // K -> chan struct{}
}

// Wait waits at key's gate until no other caller holds it, or until ctx ends. The caller calls unlock when done.
func (g *Gates[K]) Wait(ctx context.Context, key K) (unlock func(), err error) {
	v, _ := g.gates.LoadOrStore(key, make(chan struct{}, 1))
	gate := v.(chan struct{})
	select {
	case gate <- struct{}{}:
		return func() { <-gate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Backoff holds each key's last failed refresh until RefreshBackoff has passed, so the key's callers do not all try
// again meanwhile. The zero value is ready. Do not copy one once it is in use.
type Backoff[K comparable] struct {
	failures sync.Map // K -> failure
}

// failure is a refresh that failed, and until when Backoff holds it.
type failure struct {
	err   error
	until time.Time
}

// Fail records that key's refresh failed with err.
func (b *Backoff[K]) Fail(key K, err error) {
	b.failures.Store(key, failure{err, time.Now().Add(RefreshBackoff)})
}

// Failed returns why key's last refresh failed while that was under RefreshBackoff ago, else nil.
func (b *Backoff[K]) Failed(key K) error {
	if v, ok := b.failures.Load(key); ok && time.Now().Before(v.(failure).until) {
		return v.(failure).err
	}
	return nil
}

// Clear forgets key's failed refresh, as once a refresh works or a new token replaces the one that failed.
func (b *Backoff[K]) Clear(key K) { b.failures.Delete(key) }
