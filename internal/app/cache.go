package app

import (
	"context"
	"sync"
	"time"
)

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

func (fc *flightCache[K, V]) Load(key K, fetch func() (V, error)) (V, error) {
	fe, follower := fc.acquireFlight(key)
	if follower {
		<-fe.done
		return fe.value, fe.err
	}
	val, err := fetch()
	fc.finishFlight(key, fe, val, err)
	return val, err
}

func (fc *flightCache[K, V]) LoadWithTTL(key K, ttl time.Duration, fetch func() (V, error)) (V, error) {
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
		if _, ok := fc.values[key]; ok {
			fc.expiries[key] = time.Now().Add(ttl)
		}
		fc.mu.Unlock()
	}
	return val, err
}

func (fc *flightCache[K, V]) Get(key K) (V, bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	v, ok := fc.values[key]
	return v, ok
}

func (fc *flightCache[K, V]) InvalidateAll() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.gen++
	clear(fc.values)
	clear(fc.expiries)
}

func (fc *flightCache[K, V]) ProbeLoad(key K, ctx context.Context, fetch func() (V, error)) (V, error) {
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
