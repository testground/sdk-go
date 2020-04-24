package runtime

import (
	"time"

	"github.com/rcrowley/go-metrics"
)

// Type aliases to hide implementation details in the APIs.
type (
	Counter   = metrics.Counter
	Gauge     = metrics.Gauge
	Histogram = metrics.Histogram
	Meter     = metrics.Meter
	Timer     = metrics.Timer
	Sample    = metrics.Sample
	EWMA      = metrics.EWMA
	Logger    = metrics.Logger
)

type Metrics struct {
	runenv *RunEnv
}

func (*Metrics) NewCounter(name string) Counter {
	return metrics.GetOrRegister(name, metrics.NewCounter()).(metrics.Counter)
}

func (*Metrics) NewGauge(name string) Gauge {
	return metrics.GetOrRegister(name, metrics.NewGauge()).(metrics.Gauge)
}

func (*Metrics) NewGaugeFloat64(name string) Gauge {
	return metrics.GetOrRegister(name, metrics.NewGaugeFloat64()).(metrics.Gauge)
}

func (*Metrics) NewHistogram(name string, s metrics.Sample) Histogram {
	return metrics.GetOrRegister(name, metrics.NewHistogram(s)).(metrics.Histogram)
}
func (*Metrics) NewMeter(name string) Meter {
	return metrics.GetOrRegister(name, metrics.NewMeter()).(metrics.Meter)
}

func (*Metrics) NewTimer(name string) Timer {
	return metrics.GetOrRegister(name, metrics.NewTimer()).(metrics.Timer)
}

func (*Metrics) NewExpDecaySample(name string, reservoirSize int, alpha float64) Sample {
	return metrics.GetOrRegister(name, metrics.NewExpDecaySample(reservoirSize, alpha)).(metrics.Sample)
}

func (*Metrics) NewUniformSample(name string, reservoirSize int) Sample {
	return metrics.GetOrRegister(name, metrics.NewUniformSample(reservoirSize)).(metrics.Sample)
}

func (*Metrics) NewEWMA(name string, alpha float64) EWMA {
	return metrics.GetOrRegister(name, metrics.NewEWMA(alpha)).(metrics.EWMA)
}

func (*Metrics) Log(duration time.Duration, l Logger) {
	metrics.Log(metrics.DefaultRegistry, duration, l)
}
