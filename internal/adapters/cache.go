package adapters

import (
	"context"
	"errors"
	"sync"
	"time"
)

var errCacheStale = errors.New("cache generation changed")

type flightEntry[V any] struct {
	done       chan struct{}
	generation uint64
	value      V
	err        error
}

type flightCache[K comparable, V any] struct {
	mu       sync.Mutex
	gen      uint64
	values   map[K]V
	expiries map[K]time.Time
	flights  map[K]*flightEntry[V]
}

func newFlightCache[K comparable, V any]() *flightCache[K, V] {
	return &flightCache[K, V]{
		values:   make(map[K]V),
		expiries: make(map[K]time.Time),
		flights:  make(map[K]*flightEntry[V]),
	}
}

func (fc *flightCache[K, V]) acquireFlight(key K) (*flightEntry[V], bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if fe, ok := fc.flights[key]; ok {
		return fe, true
	}
	fe := &flightEntry[V]{done: make(chan struct{}), generation: fc.gen}
	fc.flights[key] = fe
	return fe, false
}

func (fc *flightCache[K, V]) finishFlight(key K, fe *flightEntry[V], val V, err error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fe.value = val
	fe.err = err
	close(fe.done)
	if fc.flights[key] == fe {
		delete(fc.flights, key)
	}
	if fe.generation != fc.gen {
		return
	}
	if err == nil {
		fc.values[key] = val
	}
}

func (fc *flightCache[K, V]) fresh(key K) (V, bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	v, ok := fc.values[key]
	if !ok {
		var zero V
		return zero, false
	}
	if exp, hasExp := fc.expiries[key]; hasExp && !time.Now().Before(exp) {
		var zero V
		return zero, false
	}
	return v, true
}

func (fc *flightCache[K, V]) storeExpiry(key K, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if _, ok := fc.values[key]; ok {
		fc.expiries[key] = time.Now().Add(ttl)
	}
}

func (fc *flightCache[K, V]) awaitFlight(ctx context.Context, fe *flightEntry[V]) (V, error) {
	if ctx == nil {
		<-fe.done
		fc.mu.Lock()
		defer fc.mu.Unlock()
		if fe.generation != fc.gen {
			var zero V
			return zero, errCacheStale
		}
		return fe.value, fe.err
	}
	select {
	case <-fe.done:
		fc.mu.Lock()
		defer fc.mu.Unlock()
		if fe.generation != fc.gen {
			var zero V
			return zero, errCacheStale
		}
		return fe.value, fe.err
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	}
}

// Load returns the cached value for key, single-flighting concurrent fetches.
// A positive ttl bounds the entry's lifetime; ctx cancels a follower's wait.
func (fc *flightCache[K, V]) Load(key K, ttl time.Duration, ctx context.Context, fetch func() (V, error)) (V, error) {
	return fc.loadWithTTLAndCtx(key, ttl, ctx, fetch)
}

func (fc *flightCache[K, V]) Get(key K) (V, bool) {
	return fc.fresh(key)
}

func (fc *flightCache[K, V]) InvalidateAll() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.gen++
	clear(fc.values)
	clear(fc.expiries)
}

func (fc *flightCache[K, V]) loadWithTTLAndCtx(key K, ttl time.Duration, ctx context.Context, fetch func() (V, error)) (V, error) {
	if ttl > 0 {
		if v, ok := fc.fresh(key); ok {
			return v, nil
		}
	}
	for {
		fe, follower := fc.acquireFlight(key)
		if follower {
			val, err := fc.awaitFlight(ctx, fe)
			if err == errCacheStale {
				continue
			}
			return val, err
		}
		val, err := fetch()
		fc.finishFlight(key, fe, val, err)
		if err == nil && ttl > 0 {
			fc.storeExpiry(key, ttl)
		}
		return val, err
	}
}
