package events

import (
	"testing"
	"time"
)

// TestBusMonitorIntervalFloor locks in the fix for the idle-CPU spin: the
// queue-size monitoring tick must never be faster than busMonitorMinInterval,
// regardless of the (microsecond-scale) FlushInterval defaults. Before the fix
// run() ticked at FlushInterval directly (down to 50µs), waking an idle bus tens
// of thousands of times per second.
func TestBusMonitorIntervalFloor(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"default-100us-floored", 100 * time.Microsecond, busMonitorMinInterval},
		{"fastest-50us-floored", 50 * time.Microsecond, busMonitorMinInterval},
		{"zero-falls-back-then-floored", 0, busMonitorMinInterval},
		{"sub-second-floored", 500 * time.Millisecond, busMonitorMinInterval},
		{"at-floor", busMonitorMinInterval, busMonitorMinInterval},
		{"above-floor-honored", 5 * time.Second, 5 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := busMonitorInterval(c.in); got != c.want {
				t.Errorf("busMonitorInterval(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// TestIdleBusDispatchesPromptlyNotOnMonitorTick proves dispatch is event-driven,
// not gated on the monitoring tick. It sets a deliberately slow FlushInterval
// (2s) so that a tick-gated implementation would take up to 2s to deliver, then
// asserts a single event published into an otherwise idle bus is delivered well
// under that. This guards against a naive "just slow the ticker" regression that
// would trade CPU for latency.
func TestIdleBusDispatchesPromptlyNotOnMonitorTick(t *testing.T) {
	cfg := DefaultBusConfig()
	cfg.FlushInterval = 2 * time.Second // monitor cadence; must NOT gate dispatch

	bus := NewLynxEventBus(cfg, BusTypeSystem, nil)
	defer bus.Close()

	got := make(chan struct{}, 1)
	bus.Subscribe(func(LynxEvent) {
		select {
		case got <- struct{}{}:
		default:
		}
	})

	// Let the worker settle into its idle blocking wait.
	time.Sleep(50 * time.Millisecond)

	bus.Publish(NewLynxEvent(EventSystemStart, "system", "idle-dispatch-test"))

	select {
	case <-got:
		// delivered promptly via the event-driven path
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("event not dispatched within 500ms; dispatch appears gated on the %v monitor tick", cfg.FlushInterval)
	}
}
