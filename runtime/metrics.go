package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-multierror"
	_ "github.com/influxdata/influxdb1-client"
	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/rcrowley/go-metrics"
)

type Metrics struct {
	re          *RunEnv
	diagnostics *MetricsApi
	results     *MetricsApi
	influxdb    client.Client
	batcher     Batcher
	tags        map[string]string
}

func newMetrics(re *RunEnv) *Metrics {
	m := &Metrics{re: re}

	var dsinks = []MetricSinkFn{m.logSinkJSON("diagnostics.out")}
	if client, err := NewInfluxDBClient(re); err == nil {
		m.tags = map[string]string{
			"plan":     re.TestPlan,
			"case":     re.TestCase,
			"run":      re.TestRun,
			"group_id": re.TestGroupID,
		}

		m.influxdb = client
		if InfluxBatching {
			m.batcher = newBatcher(re, client, InfluxBatchLength, InfluxBatchInterval, InfluxBatchRetryOpts(re)...)
		} else {
			m.batcher = &nilBatcher{client}
		}

		dsinks = append(dsinks, m.writeToInfluxDBSink("diagnostics"))
	} else {
		re.RecordMessage("InfluxDB unavailable; no metrics will be dispatched: %s", err)
	}

	m.diagnostics = newMetricsApi(re, metricsApiOpts{
		freq:        5 * time.Second,
		preregister: metrics.RegisterRuntimeMemStats,
		callbacks:   []func(metrics.Registry){metrics.CaptureRuntimeMemStatsOnce},
		sinks:       dsinks,
	})

	m.results = newMetricsApi(re, metricsApiOpts{
		freq:  1 * time.Second,
		sinks: []MetricSinkFn{m.logSinkJSON("results.out")},
	})

	return m
}

func (m *Metrics) R() *MetricsApi {
	return m.results
}

func (m *Metrics) D() *MetricsApi {
	return m.diagnostics
}

func (m *Metrics) Close() error {
	var err *multierror.Error

	// close diagnostics; this stops the ticker and any further observations on
	// runenv.D() will fail/panic.
	err = multierror.Append(err, m.diagnostics.Close())

	// close results; no more results via runenv.R() can be recorded.
	err = multierror.Append(err, m.results.Close())

	if m.influxdb != nil {
		// Next, we reopen the results.out file, and write all points to InfluxDB.
		results := filepath.Join(m.re.TestOutputsPath, "results.out")
		if file, errf := os.OpenFile(results, os.O_RDONLY, 0666); errf == nil {
			err = multierror.Append(err, m.batchInsertInfluxDB(file))
		} else {
			err = multierror.Append(err, errf)
		}
	}

	// Flush the immediate InfluxDB writer.
	if m.batcher != nil {
		err = multierror.Append(err, m.batcher.Close())
	}

	// Now we're ready to close InfluxDB.
	if m.influxdb != nil {
		err = multierror.Append(err, m.influxdb.Close())
	}

	return err.ErrorOrNil()
}

func (m *Metrics) batchInsertInfluxDB(results *os.File) error {
	sink := m.writeToInfluxDBSink("results")

	for dec := json.NewDecoder(results); dec.More(); {
		var me Metric
		if err := dec.Decode(&me); err != nil {
			m.re.RecordMessage("failed to decode Metric from results.out: %s", err)
			continue
		}

		if err := sink(&me); err != nil {
			m.re.RecordMessage("failed to process Metric from results.out: %s", err)
		}
	}
	return nil
}

func (m *Metrics) logSinkJSON(filename string) MetricSinkFn {
	f, err := m.re.CreateRawAsset(filename)
	if err != nil {
		panic(err)
	}

	enc := json.NewEncoder(f)
	return func(m *Metric) error {
		return enc.Encode(m)
	}
}

func (m *Metrics) writeToInfluxDBSink(measurementType string) MetricSinkFn {
	return func(metric *Metric) error {
		fields := make(map[string]interface{}, len(metric.Measures))
		for k, v := range metric.Measures {
			fields[k] = v

			measurementName := fmt.Sprintf("%s.%s.%s", measurementType, metric.Name, metric.Type.String())

			p, err := client.NewPoint(measurementName, m.tags, fields, time.Unix(0, metric.Timestamp))
			if err != nil {
				return err
			}
			m.batcher.WritePoint(p)
		}
		return nil
	}
}

func (m *Metrics) recordEvent(evt *Event) {
	if m.influxdb == nil {
		return
	}

	// this map copy is terrible; the influxdb v2 SDK makes points mutable.
	tags := make(map[string]string, len(m.tags)+1)
	for k, v := range m.tags {
		tags[k] = v
	}

	if evt.Outcome != "" {
		tags["outcome"] = string(evt.Outcome)
	}

	fields := map[string]interface{}{
		"error": evt.Error,
	}

	measurement := fmt.Sprintf("events.%s", string(evt.Type))
	p, err := client.NewPoint(measurement, tags, fields)
	if err != nil {
		m.re.RecordMessage("failed to create InfluxDB point: %s", err)
	}

	m.batcher.WritePoint(p)
}
