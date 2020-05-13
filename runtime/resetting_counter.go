package runtime

import (
	"sync/atomic"

	"github.com/rcrowley/go-metrics"
)

// GetOrRegisterCounter returns an existing Counter or constructs and registers
// a new StandardCounter.
func GetOrRegisterResettingCounter(name string, r metrics.Registry) metrics.Counter {
	if nil == r {
		r = metrics.DefaultRegistry
	}
	return r.GetOrRegister(name, NewResettingCounter).(Counter)
}

// NewResettingCounter constructs a new StandardCounter.
func NewResettingCounter() Counter {
	if metrics.UseNilMetrics {
		return metrics.NilCounter{}
	}
	return &StandardResettingCounter{0}
}

// NewRegisteredResettingCounter constructs and registers a new StandardCounter.
func NewRegisteredResettingCounter(name string, r metrics.Registry) Counter {
	c := NewResettingCounter()
	if nil == r {
		r = metrics.DefaultRegistry
	}
	r.Register(name, c)
	return c
}

// StandardResettingCounter is the standard implementation of a Counter and uses the
// sync/atomic package to manage a single int64 value. It resets when Snapshot() is called.
type StandardResettingCounter struct {
	count int64
}

// Clear sets the counter to zero.
func (c *StandardResettingCounter) Clear() {
	atomic.StoreInt64(&c.count, 0)
}

// Count returns the current count.
func (c *StandardResettingCounter) Count() int64 {
	return atomic.LoadInt64(&c.count)
}

// Dec decrements the counter by the given amount.
func (c *StandardResettingCounter) Dec(i int64) {
	atomic.AddInt64(&c.count, -i)
}

// Inc increments the counter by the given amount.
func (c *StandardResettingCounter) Inc(i int64) {
	atomic.AddInt64(&c.count, i)
}

// Snapshot returns a read-only copy of the counter, and resets it.
func (c *StandardResettingCounter) Snapshot() Counter {
	currentValue := atomic.SwapInt64(&c.count, 0)
	return metrics.CounterSnapshot(currentValue)
}
