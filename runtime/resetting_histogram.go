package runtime

import (
	"math"
	"sort"
	"sync"

	"github.com/rcrowley/go-metrics"
)

// Initial slice capacity for the values stored in a ResettingHistogram
const InitialResettingHistogramSliceCap = 10

// newResettingHistogram constructs a new StandardResettingHistogram
func newResettingHistogram() Histogram {
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

// Update is a no-op.
func (NilResettingHistogram) Update(int64) {}

// Clear is a no-op.
func (NilResettingHistogram) Clear() {}

func (NilResettingHistogram) Count() int64 {
	return 0
}

func (NilResettingHistogram) Variance() float64 {
	return 0.0
}

func (NilResettingHistogram) Min() int64 {
	return 0
}

func (NilResettingHistogram) Max() int64 {
	return 0
}

func (NilResettingHistogram) Sum() int64 {
	return 0
}

func (NilResettingHistogram) StdDev() float64 {
	return 0.0
}

func (NilResettingHistogram) Sample() Sample {
	return metrics.NilSample{}
}

func (NilResettingHistogram) Percentiles([]float64) []float64 {
	return nil
}

func (NilResettingHistogram) Percentile(float64) float64 {
	return 0.0
}

func (NilResettingHistogram) Mean() float64 {
	return 0.0
}

// StandardResettingHistogram is used for storing aggregated values for timers, which are reset on every flush interval.
type StandardResettingHistogram struct {
	values []int64
	mutex  sync.Mutex
}

func (t *StandardResettingHistogram) Count() int64 {
	panic("Count called on a StandardResettingHistogram")
}

func (t *StandardResettingHistogram) Max() int64 {
	panic("Max called on a StandardResettingHistogram")
}

func (t *StandardResettingHistogram) Min() int64 {
	panic("Min called on a StandardResettingHistogram")
}

func (t *StandardResettingHistogram) StdDev() float64 {
	panic("StdDev called on a StandardResettingHistogram")
}

func (t *StandardResettingHistogram) Variance() float64 {
	panic("Variance called on a StandardResettingHistogram")
}

func (t *StandardResettingHistogram) Sum() int64 {
	panic("Sum called on a StandardResettingHistogram")
}

func (t *StandardResettingHistogram) Sample() Sample {
	panic("Sample called on a StandardResettingHistogram")
}

func (t *StandardResettingHistogram) Percentiles([]float64) []float64 {
	panic("Percentiles called on a StandardResettingHistogram")
}

func (t *StandardResettingHistogram) Percentile(float64) float64 {
	panic("Percentile called on a StandardResettingHistogram")
}

func (t *StandardResettingHistogram) Mean() float64 {
	panic("Mean called on a StandardResettingHistogram")
}

// Values returns a slice with all measurements.
func (t *StandardResettingHistogram) Values() []int64 {
	return t.values
}

// Snapshot resets the timer and returns a read-only copy of its sorted contents.
func (t *StandardResettingHistogram) Snapshot() Histogram {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	currentValues := t.values
	t.values = make([]int64, 0, InitialResettingHistogramSliceCap)

	sort.Slice(currentValues, func(i, j int) bool { return currentValues[i] < currentValues[j] })

	return &ResettingHistogramSnapshot{
		values: currentValues,
	}
}

func (t *StandardResettingHistogram) Clear() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.values = make([]int64, 0, InitialResettingHistogramSliceCap)
}

// Record the duration of an event.
func (t *StandardResettingHistogram) Update(d int64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.values = append(t.values, d)
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

func (*ResettingHistogramSnapshot) Update(int64) {
	panic("Update called on a ResettingHistogramSnapshot")
}

func (t *ResettingHistogramSnapshot) Clear() {
	panic("Clear called on a ResettingHistogramSnapshot")
}

func (t *ResettingHistogramSnapshot) Sample() Sample {
	panic("Sample called on a ResettingHistogramSnapshot")
}

func (t *ResettingHistogramSnapshot) Count() int64 { return int64(len(t.values)) }

// Values returns all values from snapshot.
func (t *ResettingHistogramSnapshot) Values() []int64 {
	return t.values
}

func (t *ResettingHistogramSnapshot) Min() int64 {
	if len(t.values) > 0 {
		return t.values[0]
	}
	return 0
}

func (t *ResettingHistogramSnapshot) Variance() float64 {
	if 0 == len(t.values) {
		return 0.0
	}
	m := t.Mean()
	var sum float64
	for _, v := range t.values {
		d := float64(v) - m
		sum += d * d
	}
	return sum / float64(len(t.values))
}

func (t *ResettingHistogramSnapshot) Max() int64 {
	if len(t.values) > 0 {
		return t.values[len(t.values)-1]
	}
	return 0
}

func (t *ResettingHistogramSnapshot) StdDev() float64 {
	return math.Sqrt(t.Variance())
}

func (t *ResettingHistogramSnapshot) Sum() int64 {
	var sum int64
	for _, v := range t.values {
		sum += v
	}
	return sum
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
	count := len(t.values)
	if count == 0 {
		t.thresholdBoundaries = make([]float64, len(percentiles))
		t.mean = 0
		t.calculated = true
		return
	}

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
	t.calculated = true
}
