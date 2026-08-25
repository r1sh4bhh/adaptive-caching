package events

import (
	"sync"
	"testing"
	"time"
)

// TestPublishDoesNotBlockOnStalledSubscriber is the most important test in P1.
//
// A subscriber that never reads its channel must cause DROPS, not backpressure.
// If Publish could block here, every latency measurement in the project would
// silently include the consumer's stall.
func TestPublishDoesNotBlockOnStalledSubscriber(t *testing.T) {
	b := NewBus()
	const buf = 8
	const publishes = 1000

	// Subscribe and never read from the channel.
	_ = b.Subscribe("stalled", buf, TypeHit)

	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		for i := 0; i < publishes; i++ {
			b.Publish(Event{Seq: uint64(i), Type: TypeHit, Timestamp: time.Now()})
		}
		done <- time.Since(start)
	}()

	select {
	case elapsed := <-done:
		if elapsed > 2*time.Second {
			t.Fatalf("Publish loop took %v: it is not effectively non-blocking", elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked on a stalled subscriber: this corrupts every latency measurement")
	}

	dropped := b.DroppedCount("stalled")
	if dropped == 0 {
		t.Fatal("expected non-zero DroppedCount for a stalled subscriber")
	}
	if want := uint64(publishes - buf); dropped != want {
		t.Fatalf("DroppedCount = %d, want %d (published %d, buffer %d)", dropped, want, publishes, buf)
	}
}

func TestSubscribeReceivesEvents(t *testing.T) {
	b := NewBus()
	ch := b.Subscribe("reader", 4)

	b.Publish(Event{Seq: 7, Type: TypeMiss})

	select {
	case e := <-ch:
		if e.Seq != 7 || e.Type != TypeMiss {
			t.Fatalf("got %+v, want Seq=7 Type=miss", e)
		}
	case <-time.After(time.Second):
		t.Fatal("no event delivered")
	}
	if got := b.DroppedCount("reader"); got != 0 {
		t.Fatalf("DroppedCount = %d, want 0", got)
	}
}

func TestTypeFiltering(t *testing.T) {
	b := NewBus()
	filtered := b.Subscribe("filtered", 16, TypeSwitch, TypeDetection)
	all := b.Subscribe("all", 16)

	b.Publish(Event{Seq: 1, Type: TypeHit})
	b.Publish(Event{Seq: 2, Type: TypeSwitch})
	b.Publish(Event{Seq: 3, Type: TypeMiss})
	b.Publish(Event{Seq: 4, Type: TypeDetection})

	if got, want := len(filtered), 2; got != want {
		t.Fatalf("filtered subscriber received %d events, want %d", got, want)
	}
	if got, want := len(all), 4; got != want {
		t.Fatalf("unfiltered subscriber received %d events, want %d", got, want)
	}

	e := <-filtered
	if e.Type != TypeSwitch {
		t.Fatalf("first filtered event is %v, want switch", e.Type)
	}
	e = <-filtered
	if e.Type != TypeDetection {
		t.Fatalf("second filtered event is %v, want detection", e.Type)
	}
}

func TestUnsubscribe(t *testing.T) {
	b := NewBus()
	ch := b.Subscribe("temp", 4)

	b.Publish(Event{Seq: 1, Type: TypeHit})
	b.Unsubscribe("temp")

	// The buffered event is still readable, then the channel is closed.
	if e := <-ch; e.Seq != 1 {
		t.Fatalf("got Seq=%d, want 1", e.Seq)
	}
	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed after Unsubscribe")
	}

	// Publishing after Unsubscribe must not panic on the closed channel.
	b.Publish(Event{Seq: 2, Type: TypeHit})

	if got := b.DroppedCount("temp"); got != 0 {
		t.Fatalf("DroppedCount for unknown subscriber = %d, want 0", got)
	}
	b.Unsubscribe("temp") // idempotent
}

func TestResubscribeReplacesPreviousSubscription(t *testing.T) {
	b := NewBus()
	old := b.Subscribe("dup", 4)
	fresh := b.Subscribe("dup", 4)

	if _, ok := <-old; ok {
		t.Fatal("previous channel should be closed when a name is re-subscribed")
	}
	b.Publish(Event{Seq: 1, Type: TypeHit})
	if e := <-fresh; e.Seq != 1 {
		t.Fatalf("got Seq=%d, want 1", e.Seq)
	}
}

func TestConcurrentPublishAndSubscribe(t *testing.T) {
	b := NewBus()
	defer b.Close()

	const publishers = 8
	const perPublisher = 500

	drain := b.Subscribe("drain", 64)
	stop := make(chan struct{})
	var received int
	var drained sync.WaitGroup
	drained.Add(1)
	go func() {
		defer drained.Done()
		for {
			select {
			case _, ok := <-drain:
				if !ok {
					return
				}
				received++
			case <-stop:
				return
			}
		}
	}()

	_ = b.Subscribe("stalled", 1, TypeHit)

	var wg sync.WaitGroup
	for p := 0; p < publishers; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			for i := 0; i < perPublisher; i++ {
				b.Publish(Event{Seq: uint64(p*perPublisher + i), Type: TypeHit})
			}
		}(p)
	}

	// Churn subscriptions concurrently to exercise the lock discipline.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			b.Subscribe("churn", 2, TypeMiss)
			b.Unsubscribe("churn")
			_ = b.DroppedCount("stalled")
		}
	}()

	wg.Wait()
	close(stop)
	drained.Wait()

	if received == 0 {
		t.Fatal("draining subscriber received nothing")
	}
	if b.DroppedCount("stalled") == 0 {
		t.Fatal("expected drops on the stalled subscriber")
	}
}

func TestClosedBusPublishIsNoOp(t *testing.T) {
	b := NewBus()
	ch := b.Subscribe("x", 1)
	b.Close()
	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed")
	}
	b.Publish(Event{Type: TypeHit})
	b.Close() // idempotent

	after := b.Subscribe("y", 1)
	if _, ok := <-after; ok {
		t.Fatal("subscribing to a closed bus should yield a closed channel")
	}
}

func TestBusImplementsInterface(t *testing.T) {
	var _ Bus = NewBus()
}
