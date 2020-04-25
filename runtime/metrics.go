package runtime

import (
	"io"
	"time"

	"github.com/rcrowley/go-metrics"
)

// Type aliases to hide implementation details in the APIs.
type (
	Counter      = metrics.Counter
	EWMA         = metrics.EWMA
	Gauge        = metrics.Gauge
	GaugeFloat64 = metrics.GaugeFloat64
	Histogram    = metrics.Histogram
	Meter        = metrics.Meter
	Sample       = metrics.Sample
	Timer        = metrics.Timer
)

type Metrics struct {
	runenv *RunEnv
}

func (*Metrics) NewCounter(name string) Counter {
	return metrics.GetOrRegister(name, metrics.NewCounter()).(metrics.Counter)
}

func (*Metrics) NewEWMA(name string, alpha float64) EWMA {
	return metrics.GetOrRegister(name, metrics.NewEWMA(alpha)).(metrics.EWMA)
}

func (*Metrics) NewGauge(name string) Gauge {
	return metrics.GetOrRegister(name, metrics.NewGauge()).(metrics.Gauge)
}

func (*Metrics) NewGaugeFloat64(name string) GaugeFloat64 {
	return metrics.GetOrRegister(name, metrics.NewGaugeFloat64()).(metrics.GaugeFloat64)
}

func (*Metrics) NewFunctionalGauge(f func() int64) Gauge {
	return metrics.NewFunctionalGauge(f)
}

func (*Metrics) NewFunctionalGaugeFloat64(f func() float64) GaugeFloat64 {
	return metrics.NewFunctionalGaugeFloat64(f)
}

func (*Metrics) NewHistogram(name string, s metrics.Sample) Histogram {
	return metrics.GetOrRegister(name, metrics.NewHistogram(s)).(metrics.Histogram)
}

func (*Metrics) NewMeter(name string) Meter {
	return metrics.GetOrRegister(name, metrics.NewMeter()).(metrics.Meter)
}

func (*Metrics) NewExpDecaySample(name string, reservoirSize int, alpha float64) Sample {
	return metrics.NewExpDecaySample(reservoirSize, alpha)
}

func (*Metrics) NewUniformSample(reservoirSize int) Sample {
	return metrics.NewUniformSample(reservoirSize)
}

func (*Metrics) NewTimer(name string) Timer {
	return metrics.GetOrRegister(name, metrics.NewTimer()).(metrics.Timer)
}

func (*Metrics) WriteJson(duration time.Duration, w io.Writer) {
	metrics.WriteJSON(metrics.DefaultRegistry, duration, w)
}
