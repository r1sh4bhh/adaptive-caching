package cache

import (
	"errors"
	"testing"
	"time"
)

func TestStoreInsertAndByteAccounting(t *testing.T) {
	s := NewStore(1000)
	now := time.Now()

	if _, err := s.Insert("a", []byte("hello"), 100, now); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if _, err := s.Insert("b", nil, 300, now); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if got, want := s.Bytes(), int64(400); got != want {
		t.Errorf("Bytes = %d, want %d", got, want)
	}
	if got, want := s.Len(), 2; got != want {
		t.Errorf("Len = %d, want %d", got, want)
	}
	if got, want := s.Free(), int64(600); got != want {
		t.Errorf("Free = %d, want %d", got, want)
	}
	if got, want := s.KeyBytes(), int64(2); got != want {
		t.Errorf("KeyBytes = %d, want %d", got, want)
	}
	if !s.Contains("a") || s.Contains("zzz") {
		t.Error("Contains is wrong")
	}
}

func TestStoreReplaceAdjustsBytes(t *testing.T) {
	s := NewStore(1000)
	now := time.Now()
	if _, err := s.Insert("a", nil, 100, now); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if _, err := s.Insert("a", nil, 250, now); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got, want := s.Bytes(), int64(250); got != want {
		t.Errorf("Bytes after replace = %d, want %d", got, want)
	}
	if got, want := s.Len(), 1; got != want {
		t.Errorf("Len after replace = %d, want %d", got, want)
	}
	if got, want := s.KeyBytes(), int64(1); got != want {
		t.Errorf("KeyBytes after replace = %d, want %d", got, want)
	}
}

func TestStoreCapacityErrors(t *testing.T) {
	s := NewStore(100)
	now := time.Now()

	if _, err := s.Insert("big", nil, 101, now); !errors.Is(err, ErrObjectTooLarge) {
		t.Errorf("oversized insert error = %v, want ErrObjectTooLarge", err)
	}
	if _, err := s.Insert("neg", nil, -1, now); !errors.Is(err, ErrInvalidSize) {
		t.Errorf("negative size error = %v, want ErrInvalidSize", err)
	}
	if _, err := s.Insert("a", nil, 80, now); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if _, err := s.Insert("b", nil, 40, now); !errors.Is(err, ErrCapacityExceeded) {
		t.Errorf("over-capacity insert error = %v, want ErrCapacityExceeded", err)
	}
	if got, want := s.Bytes(), int64(80); got != want {
		t.Errorf("failed insert changed accounting: Bytes = %d, want %d", got, want)
	}
}

func TestStoreZeroSizeObject(t *testing.T) {
	s := NewStore(10)
	if _, err := s.Insert("empty", nil, 0, time.Now()); err != nil {
		t.Fatalf("zero-size insert: %v", err)
	}
	if got, want := s.Len(), 1; got != want {
		t.Errorf("Len = %d, want %d", got, want)
	}
	if got, want := s.Bytes(), int64(0); got != want {
		t.Errorf("Bytes = %d, want %d", got, want)
	}
}

func TestStoreTouchAndRemove(t *testing.T) {
	s := NewStore(1000)
	base := time.Now()
	e, err := s.Insert("a", nil, 10, base)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if e.AccessCount != 1 {
		t.Errorf("AccessCount after insert = %d, want 1", e.AccessCount)
	}

	later := base.Add(time.Second)
	touched, ok := s.Touch("a", later)
	if !ok || touched.AccessCount != 2 || !touched.LastAccessTime.Equal(later) {
		t.Fatalf("Touch = %+v, ok=%v", touched, ok)
	}
	if _, ok := s.Touch("missing", later); ok {
		t.Error("Touch on a missing key should report false")
	}

	removed, ok := s.Remove("a")
	if !ok || removed.Key != "a" {
		t.Fatalf("Remove = %+v, ok=%v", removed, ok)
	}
	if s.Bytes() != 0 || s.Len() != 0 || s.KeyBytes() != 0 {
		t.Errorf("accounting not restored after Remove: bytes=%d len=%d keys=%d",
			s.Bytes(), s.Len(), s.KeyBytes())
	}
	if _, ok := s.Remove("a"); ok {
		t.Error("second Remove should report false")
	}
}

func TestStoreEntriesAndClear(t *testing.T) {
	s := NewStore(1000)
	now := time.Now()
	for _, k := range []string{"a", "b", "c"} {
		if _, err := s.Insert(k, nil, 10, now); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	if got, want := len(s.Entries()), 3; got != want {
		t.Errorf("Entries = %d, want %d", got, want)
	}
	s.Clear()
	if s.Len() != 0 || s.Bytes() != 0 || len(s.Entries()) != 0 {
		t.Error("Clear did not empty the store")
	}
	if got, want := s.Capacity(), int64(1000); got != want {
		t.Errorf("Clear changed Capacity: %d, want %d", got, want)
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	s := NewStore(1 << 20)
	now := time.Now()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			if _, err := s.Insert("a", nil, 1, now); err != nil {
				t.Errorf("Insert: %v", err)
				return
			}
			s.Remove("a")
		}
	}()
	for i := 0; i < 1000; i++ {
		s.Contains("a")
		_ = s.Bytes()
		_ = s.Entries()
	}
	<-done
	if s.Bytes() < 0 {
		t.Fatalf("byte accounting went negative: %d", s.Bytes())
	}
}
