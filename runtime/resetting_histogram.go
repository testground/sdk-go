package runtime

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
)

// Initial slice capacity for the values stored in a ResettingHistogram
const InitialResettingHistogramSliceCap = 10

// NewResettingHistogram constructs a new StandardResettingHistogram
func NewResettingHistogram() Histogram {
	if metrics.UseNilMetrics {
		return NilResettingHistogram{}
	}
	return &StandardResettingHistogram{
		values: make([]int64, 0, InitialResettingHistogramSliceCap),
	}
}

// NilResettingHistogram is a no-op ResettingHistogram.
type NilResettingHistogram struct {
}

// Values is a no-op.
func (NilResettingHistogram) Values() []int64 { return nil }

// Snapshot is a no-op.
func (NilResettingHistogram) Snapshot() Histogram {
	return &ResettingHistogramSnapshot{
		values: []int64{},
	}
}

// Time is a no-op.
func (NilResettingHistogram) Time(func()) {}

// Update is a no-op.
func (NilResettingHistogram) Update(int64) {}

func (NilResettingHistogram) Clear() {}

func (NilResettingHistogram) Count() int64 { return 0 }

func (NilResettingHistogram) Variance() float64 { return 0 }

func (NilResettingHistogram) Min() int64 { return 0 }

func (NilResettingHistogram) Max() int64 { return 0 }

func (NilResettingHistogram) Sum() int64 { return 0 }

func (NilResettingHistogram) StdDev() float64 { return 0 }

func (NilResettingHistogram) Sample() Sample { return metrics.NilSample{} }

// Percentiles panics.
func (NilResettingHistogram) Percentiles([]float64) []float64 {
	panic("Percentiles called on a NilResettingHistogram")
}

func (NilResettingHistogram) Percentile(float64) float64 {
	panic("Percentiles called on a NilResettingHistogram")
}

// Mean panics.
func (NilResettingHistogram) Mean() float64 {
	panic("Mean called on a NilResettingHistogram")
}

// UpdateSince is a no-op.
func (NilResettingHistogram) UpdateSince(time.Time) {}

// StandardResettingHistogram is used for storing aggregated values for timers, which are reset on every flush interval.
type StandardResettingHistogram struct {
	values []int64
	mutex  sync.Mutex
}

// Values returns a slice with all measurements.
func (t *StandardResettingHistogram) Values() []int64 {
	return t.values
}

func (t *StandardResettingHistogram) Count() int64 { return int64(len(t.values)) }

func (t *StandardResettingHistogram) Max() int64 { return int64(len(t.values)) }

func (t *StandardResettingHistogram) Min() int64 { return int64(len(t.values)) }

func (t *StandardResettingHistogram) StdDev() float64 { return 0 }

func (t *StandardResettingHistogram) Variance() float64 { return 0 }

func (t *StandardResettingHistogram) Sum() int64 { return 0 }

func (t *StandardResettingHistogram) Sample() Sample {
	return metrics.NilSample{}
}

// Snapshot resets the timer and returns a read-only copy of its contents.
func (t *StandardResettingHistogram) Snapshot() Histogram {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	currentValues := t.values
	t.values = make([]int64, 0, InitialResettingHistogramSliceCap)

	return &ResettingHistogramSnapshot{
		values: currentValues,
	}
}

// Percentiles panics.
func (t *StandardResettingHistogram) Percentiles([]float64) []float64 {
	panic("Percentiles called on a StandardResettingHistogram")
}

func (t *StandardResettingHistogram) Percentile(float64) float64 {
	panic("Percentile called on a StandardResettingHistogram")
}

// Mean panics.
func (t *StandardResettingHistogram) Mean() float64 {
	panic("Mean called on a StandardResettingHistogram")
}

func (t *StandardResettingHistogram) Clear() {
}

// Record the duration of the execution of the given function.
func (t *StandardResettingHistogram) Time(f func()) {
	ts := time.Now()
	f()
	t.Update(int64(time.Since(ts)))
}

// Record the duration of an event.
func (t *StandardResettingHistogram) Update(d int64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.values = append(t.values, d)
}

// Record the duration of an event that started at a time and ends now.
func (t *StandardResettingHistogram) UpdateSince(ts time.Time) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.values = append(t.values, int64(time.Since(ts)))
}

// ResettingHistogramSnapshot is a point-in-time copy of another ResettingHistogram.
type ResettingHistogramSnapshot struct {
	values              []int64
	mean                float64
	thresholdBoundaries []float64
	calculated          bool
}

// Snapshot returns the snapshot.
func (t *ResettingHistogramSnapshot) Snapshot() Histogram { return t }

// Time panics.
func (*ResettingHistogramSnapshot) Time(func()) {
	panic("Time called on a ResettingHistogramSnapshot")
}

func (t *ResettingHistogramSnapshot) Count() int64 { return int64(len(t.values)) }

// Update panics.
func (*ResettingHistogramSnapshot) Update(int64) {
	panic("Update called on a ResettingHistogramSnapshot")
}

// UpdateSince panics.
func (*ResettingHistogramSnapshot) UpdateSince(time.Time) {
	panic("UpdateSince called on a ResettingHistogramSnapshot")
}

// Values returns all values from snapshot.
func (t *ResettingHistogramSnapshot) Values() []int64 {
	return t.values
}

func (t *ResettingHistogramSnapshot) Min() int64 {
	return 0
}

func (t *ResettingHistogramSnapshot) Variance() float64 {
	return 0
}

func (t *ResettingHistogramSnapshot) Max() int64 {
	return 0
}

func (t *ResettingHistogramSnapshot) StdDev() float64 { return 0 }

func (t *ResettingHistogramSnapshot) Sum() int64 { return 0 }

func (t *ResettingHistogramSnapshot) Clear() {
}

func (t *ResettingHistogramSnapshot) Sample() Sample {
	return metrics.NilSample{}
}

// Percentile returns the boundaries for the input percentiles.
func (t *ResettingHistogramSnapshot) Percentile(percentile float64) float64 {
	t.calc([]float64{percentile})

	return t.thresholdBoundaries[0]
}

// Percentiles returns the boundaries for the input percentiles.
func (t *ResettingHistogramSnapshot) Percentiles(percentiles []float64) []float64 {
	t.calc(percentiles)

	return t.thresholdBoundaries
}

// Mean returns the mean of the snapshotted values
func (t *ResettingHistogramSnapshot) Mean() float64 {
	if !t.calculated {
		t.calc([]float64{})
	}

	return t.mean
}

func (t *ResettingHistogramSnapshot) calc(percentiles []float64) {
	sort.Sort(Int64Slice(t.values))

	count := len(t.values)
	if count > 0 {
		min := t.values[0]
		max := t.values[count-1]

		cumulativeValues := make([]int64, count)
		cumulativeValues[0] = min
		for i := 1; i < count; i++ {
			cumulativeValues[i] = t.values[i] + cumulativeValues[i-1]
		}

		t.thresholdBoundaries = make([]float64, len(percentiles))

		thresholdBoundary := max

		for i, pct := range percentiles {
			if count > 1 {
				var abs float64
				if pct >= 0 {
					abs = pct
				} else {
					abs = 100 + pct
				}
				// poor man's math.Round(x):
				// math.Floor(x + 0.5)
				indexOfPerc := int(math.Floor(((abs / 100.0) * float64(count)) + 0.5))
				if pct >= 0 && indexOfPerc > 0 {
					indexOfPerc -= 1 // index offset=0
				}
				thresholdBoundary = t.values[indexOfPerc]
			}

			t.thresholdBoundaries[i] = float64(thresholdBoundary)
		}

		sum := cumulativeValues[count-1]
		t.mean = float64(sum) / float64(count)
	} else {
		t.thresholdBoundaries = make([]float64, len(percentiles))
		t.mean = 0
	}

	t.calculated = true
}

// Int64Slice attaches the methods of sort.Interface to []int64, sorting in increasing order.
type Int64Slice []int64

func (s Int64Slice) Len() int           { return len(s) }
func (s Int64Slice) Less(i, j int) bool { return s[i] < s[j] }
func (s Int64Slice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
