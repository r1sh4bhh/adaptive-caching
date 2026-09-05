// Command adaptive-cache is the project's acceptance demo: it loads a
// config file, constructs the cache with the named eviction policy,
// drives a trivial synthetic request loop and prints a Frame as JSON
// at the configured rate (default 10 Hz).
//
// Everything it prints comes from the observation layer — the cache
// itself has no idea a frame emitter exists. P2 added the policy
// registry and the --policy flag; before P2 the cache ran with a nil
// policy.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/r1sh4bhh/adaptive-caching/cache"
	"github.com/r1sh4bhh/adaptive-caching/cache/policy"
	"github.com/r1sh4bhh/adaptive-caching/config"
	"github.com/r1sh4bhh/adaptive-caching/events"
	"github.com/r1sh4bhh/adaptive-caching/types"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "adaptive-cache:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "configs/default.yaml", "path to the YAML config file")
	duration := flag.Duration("duration", 5*time.Second, "how long to run before exiting")
	seed := flag.Int64("seed", 1, "seed for the placeholder request generator")
	policyName := flag.String("policy", "", "eviction policy (overrides cache.policy in the config; one of "+policyFlagChoices()+")")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	// CLI --policy overrides the config's cache.policy.
	if *policyName != "" {
		cfg.Cache.Policy = *policyName
	}

	// Resolve the policy by name. "none" means no policy at all
	// (P1's behaviour); every other name must be registered.
	var pol policy.EvictionPolicy
	name := types.PolicyName(cfg.Cache.Policy)
	if name != types.PolicyNone {
		p, ok := policy.New(name)
		if !ok {
			return fmt.Errorf("unknown eviction policy %q (known: %v)",
				cfg.Cache.Policy, policy.Names())
		}
		pol = p
	}

	bus := events.NewBus()
	defer bus.Close()

	// A subscriber that proves the bus is wired up. It is intentionally small
	// and may drop: Publish must never wait for it.
	eventCh := bus.Subscribe("stdout-events", cfg.Events.BusBuffer,
		events.TypeSwitch, events.TypeDetection, events.TypeTuning, events.TypeScenarioMark)

	c := cache.New(cache.Options{
		Capacity:          cfg.Cache.Capacity.Bytes(),
		Policy:            pol,
		Bus:               bus,
		RequestSampleRate: cfg.Events.RequestSampleRate,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *duration)
	defer cancel()

	start := time.Now()
	bus.Publish(events.Event{Seq: 0, Timestamp: start, Type: events.TypeRunStart})

	go drainEvents(ctx, eventCh)
	go generate(ctx, c, *seed)

	enc := json.NewEncoder(os.Stdout)
	ticker := time.NewTicker(cfg.FrameInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			bus.Publish(events.Event{Seq: c.Seq(), Timestamp: time.Now(), Type: events.TypeRunEnd})
			return nil
		case tick := <-ticker.C:
			if err := enc.Encode(buildFrame(c, cfg, start, tick, *duration)); err != nil {
				return fmt.Errorf("encode frame: %w", err)
			}
		}
	}
}

// buildFrame aggregates the current state into one UI frame. In P8 this moves
// into a proper frame aggregator behind the event bus; the shape is frozen now
// so that it can.
func buildFrame(c *cache.Core, cfg *config.Config, start, now time.Time, total time.Duration) events.Frame {
	s := c.Stats()
	mem := c.Memory()

	elapsed := now.Sub(start)
	progress := 0.0
	if total > 0 {
		progress = float64(elapsed) / float64(total)
		if progress > 1 {
			progress = 1
		}
	}
	throughput := 0.0
	if elapsed > 0 {
		throughput = float64(s.TotalRequests) / elapsed.Seconds()
	}

	return events.Frame{
		Seq:             c.Seq(),
		WallClock:       now,
		Progress:        progress,
		HitRate:         s.HitRate,
		ByteHitRate:     s.ByteHitRate,
		Throughput:      throughput,
		P50:             s.LatencyP50,
		P95:             s.LatencyP95,
		P99:             s.LatencyP99,
		Policy:          string(c.PolicyName()),
		PolicyResidency: s.TotalRequests,
		Workload:        types.WorkloadUnknown.String(),
		Confidence:      0,
		BytesUsed:       s.BytesUsed,
		Capacity:        c.Capacity(),
		ObjectCount:     s.ObjectCount,
		MetadataBytes:   mem.MetadataBytes,
		Params:          map[string]float64{},
	}
}

// generate drives a placeholder request stream so the demo has
// something to report. Real trace sources arrive in P3. With an
// eviction policy installed (P2+), the stream exercises the policy:
// objects are admitted, evicted, and the cache reaches a steady
// hit rate that depends on the policy and the workload shape.
func generate(ctx context.Context, c *cache.Core, seed int64) {
	rng := rand.New(rand.NewSource(seed))
	payload := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		key := fmt.Sprintf("key-%d", rng.Intn(2000))
		if _, ok := c.Get(key); !ok {
			// With a nil policy nothing can be evicted, so once the cache is
			// full every insert is refused. That is the documented P1
			// behaviour, not an error.
			_ = c.Put(key, payload, int64(len(payload)))
		}
	}
}

// drainEvents keeps the demo's event subscriber moving so it does not
// accumulate drops. Nothing here can slow the cache down even if it stalls.
func drainEvents(ctx context.Context, ch <-chan events.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
		}
	}
}

// policyFlagChoices returns a comma-separated list of every registered
// policy name plus "none", for use in the --policy help text. The list
// is computed at startup so adding a new policy in cache/policy/ shows
// up automatically in the CLI's self-description.
func policyFlagChoices() string {
	names := policy.Names()
	out := "none"
	for _, n := range names {
		out += ", " + string(n)
	}
	return out
}
