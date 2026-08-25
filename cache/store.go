package cache

import (
	"sync"
	"time"

	"github.com/r1sh4bhh/adaptive-caching/types"
)

// Store is the object store: a key → *Entry map with exact byte accounting.
//
// The store holds the objects; the policy holds the metadata. The store DOES
// NOT DECIDE WHAT TO EVICT — that is the EvictionPolicy's job. All the store
// does is admit, look up, remove and count bytes, which is precisely why it
// can survive a policy switch untouched.
//
// Accounting note: an entry's byte cost is the caller-supplied size, not
// len(Value). Trace replay caches object sizes without their payloads, so
// Value may legitimately be nil while Size is non-zero.
//
// Store is safe for concurrent use.
type Store struct {
	mu       sync.RWMutex
	items    map[string]*types.Entry
	bytes    int64
	keyBytes int64
	capacity int64
}

// NewStore returns an empty store with the given byte capacity.
func NewStore(capacity int64) *Store {
	return &Store{
		items:    make(map[string]*types.Entry),
		capacity: capacity,
	}
}

// Get returns the entry for key without mutating access metadata.
func (s *Store) Get(key string) (*types.Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.items[key]
	return e, ok
}

// Touch records an access on an existing entry and returns it.
func (s *Store) Touch(key string, now time.Time) (*types.Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[key]
	if !ok {
		return nil, false
	}
	e.LastAccessTime = now
	e.AccessCount++
	return e, true
}

// Contains reports whether key is resident.
func (s *Store) Contains(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.items[key]
	return ok
}

// Insert admits an object, replacing any existing entry for the same key.
//
// It returns ErrInvalidSize for a negative size, ErrObjectTooLarge if the
// object cannot fit even in an empty cache, and ErrCapacityExceeded if it does
// not fit right now — freeing space is the caller's (i.e. the policy's) job.
func (s *Store) Insert(key string, val types.Value, size int64, now time.Time) (*types.Entry, error) {
	if size < 0 {
		return nil, ErrInvalidSize
	}
	if size > s.capacity {
		return nil, ErrObjectTooLarge
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, replacing := s.items[key]
	projected := s.bytes + size
	if replacing {
		projected -= existing.Size
	}
	if projected > s.capacity {
		return nil, ErrCapacityExceeded
	}

	if replacing {
		s.bytes -= existing.Size
		existing.Value = val
		existing.Size = size
		existing.LastAccessTime = now
		existing.AccessCount++
		s.bytes += size
		return existing, nil
	}

	e := &types.Entry{
		Key:            key,
		Value:          val,
		Size:           size,
		InsertionTime:  now,
		LastAccessTime: now,
		AccessCount:    1,
	}
	s.items[key] = e
	s.bytes += size
	s.keyBytes += int64(len(key))
	return e, nil
}

// Remove deletes a key and returns the removed entry.
func (s *Store) Remove(key string) (*types.Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[key]
	if !ok {
		return nil, false
	}
	delete(s.items, key)
	s.bytes -= e.Size
	s.keyBytes -= int64(len(key))
	return e, true
}

// Len returns the object count.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// Bytes returns the payload bytes currently resident.
func (s *Store) Bytes() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bytes
}

// KeyBytes returns the total length of all resident keys, for the metadata
// overhead metric.
func (s *Store) KeyBytes() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.keyBytes
}

// Capacity returns the byte capacity.
func (s *Store) Capacity() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.capacity
}

// Free returns the unused bytes.
func (s *Store) Free() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.capacity - s.bytes
}

// Entries returns a snapshot slice of the resident entries, for
// EvictionPolicy.Rebuild during a policy switch. The pointers alias live
// entries; callers must not mutate them.
func (s *Store) Entries() []*types.Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*types.Entry, 0, len(s.items))
	for _, e := range s.items {
		out = append(out, e)
	}
	return out
}

// Clear removes everything.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]*types.Entry)
	s.bytes = 0
	s.keyBytes = 0
}
