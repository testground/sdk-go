package runtime

import (
	"reflect"
	"sync"

	"github.com/rcrowley/go-metrics"
)

type TestgroundRegistry struct {
	metrics map[string]interface{}
	mutex   sync.RWMutex
}

// Create a new registry.
func NewRegistry() metrics.Registry {
	return &TestgroundRegistry{metrics: make(map[string]interface{})}
}

// Call the given function for each registered metric.
func (r *TestgroundRegistry) Each(f func(string, interface{})) {
	metrics := r.registered()
	for i := range metrics {
		kv := &metrics[i]
		f(kv.name, kv.value)
	}
}

// Get the metric by the given name or nil if none is registered.
func (r *TestgroundRegistry) Get(name string) interface{} {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.metrics[name]
}

// Gets an existing metric or creates and registers a new one. Threadsafe
// alternative to calling Get and Register on failure.
// The interface can be the metric to register if not found in registry,
// or a function returning the metric for lazy instantiation.
func (r *TestgroundRegistry) GetOrRegister(name string, i interface{}) interface{} {
	// access the read lock first which should be re-entrant
	r.mutex.RLock()
	metric, ok := r.metrics[name]
	r.mutex.RUnlock()
	if ok {
		return metric
	}

	// only take the write lock if we'll be modifying the metrics map
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if metric, ok := r.metrics[name]; ok {
		return metric
	}
	if v := reflect.ValueOf(i); v.Kind() == reflect.Func {
		i = v.Call(nil)[0].Interface()
	}
	r.register(name, i)
	return i
}

// Register the given metric under the given name.  Returns a DuplicateMetric
// if a metric by the given name is already registered.
func (r *TestgroundRegistry) Register(name string, i interface{}) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.register(name, i)
}

// Run all registered healthchecks.
func (r *TestgroundRegistry) RunHealthchecks() {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, i := range r.metrics {
		if h, ok := i.(metrics.Healthcheck); ok {
			h.Check()
		}
	}
}

// GetAll metrics in the Registry
func (r *TestgroundRegistry) GetAll() map[string]map[string]interface{} {
	data := make(map[string]map[string]interface{})
	r.Each(func(name string, i interface{}) {
		values := make(map[string]interface{})
		switch metric := i.(type) {
		case Counter:
			values["count"] = metric.Count()
		case metrics.Gauge:
			values["value"] = metric.Value()
		case metrics.GaugeFloat64:
			values["value"] = metric.Value()
		case metrics.Healthcheck:
			values["error"] = nil
			metric.Check()
			if err := metric.Error(); nil != err {
				values["error"] = metric.Error().Error()
			}
		case Histogram:
			h := metric.Snapshot()
			ps := h.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
			values["count"] = h.Count()
			values["min"] = h.Min()
			values["max"] = h.Max()
			values["mean"] = h.Mean()
			values["stddev"] = h.StdDev()
			values["median"] = ps[0]
			values["75%"] = ps[1]
			values["95%"] = ps[2]
			values["99%"] = ps[3]
			values["99.9%"] = ps[4]
		case Meter:
			m := metric.Snapshot()
			values["count"] = m.Count()
			values["1m.rate"] = m.Rate1()
			values["5m.rate"] = m.Rate5()
			values["15m.rate"] = m.Rate15()
			values["mean.rate"] = m.RateMean()
		case Timer:
			t := metric.Snapshot()
			ps := t.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
			values["count"] = t.Count()
			values["min"] = t.Min()
			values["max"] = t.Max()
			values["mean"] = t.Mean()
			values["stddev"] = t.StdDev()
			values["median"] = ps[0]
			values["75%"] = ps[1]
			values["95%"] = ps[2]
			values["99%"] = ps[3]
			values["99.9%"] = ps[4]
			values["1m.rate"] = t.Rate1()
			values["5m.rate"] = t.Rate5()
			values["15m.rate"] = t.Rate15()
			values["mean.rate"] = t.RateMean()
		}
		data[name] = values
	})
	return data
}

// Unregister the metric with the given name.
func (r *TestgroundRegistry) Unregister(name string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.stop(name)
	delete(r.metrics, name)
}

// Unregister all metrics.  (Mostly for testing.)
func (r *TestgroundRegistry) UnregisterAll() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for name, _ := range r.metrics {
		r.stop(name)
		delete(r.metrics, name)
	}
}

func (r *TestgroundRegistry) register(name string, i interface{}) error {
	if _, ok := r.metrics[name]; ok {
		return metrics.DuplicateMetric(name)
	}
	switch i.(type) {
	case Counter, metrics.Gauge, metrics.GaugeFloat64, metrics.Healthcheck, Histogram, Meter, Timer, ResettingTimer:
		r.metrics[name] = i
	}
	return nil
}

type metricKV struct {
	name  string
	value interface{}
}

func (r *TestgroundRegistry) registered() []metricKV {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	metrics := make([]metricKV, 0, len(r.metrics))
	for name, i := range r.metrics {
		metrics = append(metrics, metricKV{
			name:  name,
			value: i,
		})
	}
	return metrics
}

func (r *TestgroundRegistry) stop(name string) {
	if i, ok := r.metrics[name]; ok {
		if s, ok := i.(metrics.Stoppable); ok {
			s.Stop()
		}
	}
}
