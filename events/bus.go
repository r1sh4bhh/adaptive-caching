package events

import (
	"sync"
	"sync/atomic"
)

// Bus is the typed publish/subscribe transport of the observation layer.
//
// FROZEN after P1: see docs/DECISIONS.md before changing.
type Bus interface {
	// Publish delivers an event to every interested subscriber. It MUST be
	// non-blocking.
	Publish(Event)
	// Subscribe registers a named subscriber with a bounded channel of buf
	// events. Passing no types subscribes to every type.
	Subscribe(name string, buf int, types ...Type) <-chan Event
	// Unsubscribe removes a subscriber and closes its channel.
	Unsubscribe(name string)
	// DroppedCount reports how many events were dropped for a subscriber
	// because its channel was full — observability on the observability.
	DroppedCount(name string) uint64
}

// subscriber is one registered consumer.
type subscriber struct {
	name    string
	ch      chan Event
	all     bool
	filter  [maxType]bool
	dropped atomic.Uint64
}

// wants reports whether this subscriber is interested in events of type t.
func (s *subscriber) wants(t Type) bool {
	if s.all {
		return true
	}
	if int(t) >= maxType {
		return false
	}
	return s.filter[t]
}

// MemBus is the in-process implementation of Bus. It is safe for concurrent
// use by any number of publishers and subscribers.
//
// The single most important property of this type is that Publish never
// blocks. Every subscriber owns a bounded channel; when that channel is full
// the event is DROPPED and the subscriber's dropped counter is incremented.
//
// Rationale: a slow consumer — a browser dashboard, a stalled log writer, a
// paused debugger — must never exert backpressure on the cache. If it could,
// the cache's own latency measurements would silently record the consumer's
// slowness instead of the cache's, corrupting every latency number the project
// reports. Losing observability events is an acceptable, counted failure;
// distorting the measurements is not.
type MemBus struct {
	mu     sync.RWMutex
	subs   map[string]*subscriber
	order  []*subscriber
	closed bool
}

// NewBus returns a ready-to-use in-process bus.
func NewBus() *MemBus {
	return &MemBus{subs: make(map[string]*subscriber)}
}

// Publish delivers e to every subscriber registered for e.Type. It never
// blocks: a subscriber whose channel is full has the event dropped and its
// dropped counter incremented.
func (b *MemBus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for _, s := range b.order {
		if !s.wants(e.Type) {
			continue
		}
		select {
		case s.ch <- e:
		default:
			// Full: drop and count. NEVER block here.
			s.dropped.Add(1)
		}
	}
}

// Subscribe registers a named subscriber. buf is the channel capacity;
// negative values are treated as zero, and a zero-capacity subscriber only
// receives events while it is actively blocked on a receive.
//
// Passing no types subscribes to every event type. Re-subscribing under an
// existing name replaces (and closes) the previous subscription.
func (b *MemBus) Subscribe(name string, buf int, types ...Type) <-chan Event {
	if buf < 0 {
		buf = 0
	}
	s := &subscriber{
		name: name,
		ch:   make(chan Event, buf),
		all:  len(types) == 0,
	}
	for _, t := range types {
		if int(t) < maxType {
			s.filter[t] = true
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(s.ch)
		return s.ch
	}
	b.removeLocked(name)
	b.subs[name] = s
	b.order = append(b.order, s)
	return s.ch
}

// Unsubscribe removes the named subscriber and closes its channel. It is a
// no-op if no such subscriber exists.
func (b *MemBus) Unsubscribe(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.removeLocked(name)
}

// DroppedCount returns the number of events dropped for the named subscriber,
// or zero if it is unknown.
func (b *MemBus) DroppedCount(name string) uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if s, ok := b.subs[name]; ok {
		return s.dropped.Load()
	}
	return 0
}

// Close unsubscribes everyone and makes subsequent publishes no-ops. It is
// idempotent.
func (b *MemBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, s := range b.order {
		close(s.ch)
	}
	b.subs = make(map[string]*subscriber)
	b.order = nil
}

// removeLocked drops a subscriber. The caller must hold the write lock, which
// guarantees no Publish is in flight and therefore that closing the channel
// cannot race with a send.
func (b *MemBus) removeLocked(name string) {
	s, ok := b.subs[name]
	if !ok {
		return
	}
	delete(b.subs, name)
	for i, cur := range b.order {
		if cur == s {
			b.order = append(b.order[:i], b.order[i+1:]...)
			break
		}
	}
	close(s.ch)
}
