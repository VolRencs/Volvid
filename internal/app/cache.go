package app

import (
	"context"
	"sync"
	"time"
)

// flightEntry tracks a single in-flight fetch so that concurrent callers
// for the same key join the running fetch instead of starting their own.
type flightEntry[V any] struct {
	done  chan struct{}
	value V
	err   error
}

// FlightCache is a small generic cache with singleflight-style
// deduplication of concurrent fetches for the same key.
//
// Only successful fetches are cached; failed fetches wake the followers
// with the error and leave no value behind.
type FlightCache[K comparable, V any] struct {
	mu       sync.Mutex
	values   map[K]V
	expiries map[K]time.Time
	flights  map[K]*flightEntry[V]
}

func newFlightCache[K comparable, V any]() *FlightCache[K, V] {
	return &FlightCache[K, V]{
		values:   make(map[K]V),
		expiries: make(map[K]time.Time),
		flights:  make(map[K]*flightEntry[V]),
	}
}

// acquireFlight returns the in-flight entry for key and true when the
// caller should wait for it. Otherwise it registers a new entry and
// returns false, making the caller the leader that runs fetch.
func (fc *FlightCache[K, V]) acquireFlight(key K) (*flightEntry[V], bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if fe, ok := fc.flights[key]; ok {
		return fe, true
	}
	fe := &flightEntry[V]{done: make(chan struct{})}
	fc.flights[key] = fe
	return fe, false
}

// finishFlight publishes the fetch result, wakes the followers and caches
// the value when fetch succeeded.
func (fc *FlightCache[K, V]) finishFlight(key K, fe *flightEntry[V], val V, err error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fe.value = val
	fe.err = err
	close(fe.done)
	if fc.flights[key] == fe {
		delete(fc.flights, key)
	}
	if err == nil {
		fc.values[key] = val
	}
}

// Load returns the cached value for key or runs fetch exactly once
// across concurrent callers.
func (fc *FlightCache[K, V]) Load(key K, fetch func() (V, error)) (V, error) {
	fe, follower := fc.acquireFlight(key)
	if follower {
		<-fe.done
		return fe.value, fe.err
	}
	val, err := fetch()
	fc.finishFlight(key, fe, val, err)
	return val, err
}

// LoadWithTTL returns the cached value for key when it exists and its
// TTL has not elapsed; otherwise it behaves like Load and stamps a fresh
// TTL on success. Expiry is kept inside the cache under the same mutex,
// so readers and writers cannot race.
func (fc *FlightCache[K, V]) LoadWithTTL(key K, ttl time.Duration, fetch func() (V, error)) (V, error) {
	fc.mu.Lock()
	if exp, ok := fc.expiries[key]; ok && time.Now().Before(exp) {
		if v, ok := fc.values[key]; ok {
			fc.mu.Unlock()
			return v, nil
		}
	}
	fc.mu.Unlock()
	val, err := fc.Load(key, fetch)
	if err == nil {
		fc.mu.Lock()
		fc.expiries[key] = time.Now().Add(ttl)
		fc.mu.Unlock()
	}
	return val, err
}

// Get returns the cached value for key, if present.
func (fc *FlightCache[K, V]) Get(key K) (V, bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	v, ok := fc.values[key]
	return v, ok
}

// InvalidateAll drops cached values and forgets in-flight fetches.
// In-flight leaders still publish their results, but followers that
// arrive after invalidation start a fresh fetch.
func (fc *FlightCache[K, V]) InvalidateAll() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	clear(fc.values)
	clear(fc.expiries)
	clear(fc.flights)
}

// ProbeLoad is Load with a context-aware wait: a follower returns
// ctx.Err() when ctx finishes first, without cancelling the leader.
func (fc *FlightCache[K, V]) ProbeLoad(key K, ctx context.Context, fetch func() (V, error)) (V, error) {
	if v, ok := fc.Get(key); ok {
		return v, nil
	}
	fe, follower := fc.acquireFlight(key)
	if follower {
		select {
		case <-fe.done:
			return fe.value, fe.err
		case <-ctx.Done():
			var zero V
			return zero, ctx.Err()
		}
	}
	val, err := fetch()
	fc.finishFlight(key, fe, val, err)
	return val, err
}
