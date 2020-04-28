package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/hashicorp/go-multierror"
	influxdb2 "github.com/influxdata/influxdb-client-go"
	"go.uber.org/zap"
)

// RunEnv encapsulates the context for this test run.
type RunEnv struct {
	RunParams

	logger *zap.Logger

	diagnostics *MetricsApi
	results     *MetricsApi
	influxdb    influxdb2.InfluxDBClient
	wapi        influxdb2.WriteApi
	tags        map[string]string

	wg        sync.WaitGroup
	closeCh   chan struct{}
	assetsErr error

	unstructured struct {
		files []*os.File
		ch    chan *os.File
	}
	structured struct {
		loggers []*zap.Logger
		ch      chan *zap.Logger
	}
}

func (re *RunEnv) SLogger() *zap.SugaredLogger {
	return re.logger.Sugar()
}

// NewRunEnv constructs a runtime environment from the given runtime parameters.
func NewRunEnv(params RunParams) *RunEnv {
	re := &RunEnv{
		RunParams: params,
		closeCh:   make(chan struct{}),
	}
	re.initLogger()

	re.structured.ch = make(chan *zap.Logger)
	re.unstructured.ch = make(chan *os.File)

	re.wg.Add(1)
	go re.manageAssets()

	var dsinks = []SinkFn{LogSinkJSON(re, "diagnostics.out")}
	client, err := NewInfluxDBClient()
	if err == nil {
		re.tags = map[string]string{
			"plan":     re.TestPlan,
			"case":     re.TestCase,
			"run":      re.TestRun,
			"group_id": re.TestGroupID,
		}

		re.influxdb = client
		re.wapi = client.WriteApi("testground", "diagnostics")
		dsinks = append(dsinks, WriteToInfluxDB(re))

		re.wg.Add(1)
		go re.monitorInfluxDBErrors()
	} else {
		re.RecordMessage("InfluxDB unavailable; no metrics will be dispatched: %s", err)
	}

	re.diagnostics = newMetricsApi(re, metricsApiOpts{
		freq:  1 * time.Second,
		sinks: dsinks,
	})

	re.results = newMetricsApi(re, metricsApiOpts{
		freq:  1 * time.Second,
		sinks: []SinkFn{LogSinkJSON(re, "results.out")},
	})

	return re
}

// R returns a metrics object for results.
func (re *RunEnv) R() *MetricsApi {
	return re.results
}

// D returns a metrics object for diagnostics.
func (re *RunEnv) D() *MetricsApi {
	return re.diagnostics
}

func (re *RunEnv) monitorInfluxDBErrors() {
	defer re.wg.Done()

	for {
		select {
		case err := <-re.wapi.Errors():
			if err == nil {
				continue
			}
			re.RecordMessage("failed while writing to InfluxDB: %s", err)
		case <-re.closeCh:
			return
		}
	}
}

func (re *RunEnv) manageAssets() {
	defer re.wg.Done()

	var err *multierror.Error
	defer func() { re.assetsErr = err.ErrorOrNil() }()

	for {
		select {
		case f := <-re.unstructured.ch:
			re.unstructured.files = append(re.unstructured.files, f)
		case l := <-re.structured.ch:
			re.structured.loggers = append(re.structured.loggers, l)
		case <-re.closeCh:
			for _, f := range re.unstructured.files {
				err = multierror.Append(err, f.Close())
			}
			for _, l := range re.structured.loggers {
				err = multierror.Append(err, l.Sync())
			}
			return
		}
	}
}

func (re *RunEnv) Close() error {
	var err *multierror.Error

	// close diagnostics; this stops the ticker and any further observations on
	// runenv.D() will fail/panic.
	err = multierror.Append(re.diagnostics.Close())

	// close results; no more results via runenv.R() can be recorded.
	err = multierror.Append(re.results.Close())

	if re.influxdb != nil {
		// Next, we reopen the results.out file, and upload all points to InfluxDB
		// using the blocking API.
		results, err2 := os.OpenFile(filepath.Join(re.TestOutputsPath, "results.out"), os.O_RDONLY, 0666)
		if err2 == nil {
			// batchInsertInfluxDB will record errors via runenv.RecordMessage().
			err2 = re.batchInsertInfluxDB(results)
		}
		err = multierror.Append(err, err2)
	}

	// Flush the immediate InfluxDB writer.
	if re.wapi != nil {
		re.wapi.Flush()
		re.wapi.Close()
	}

	// This close stops monitoring the wapi errors channel, and closes assets.
	close(re.closeCh)
	re.wg.Wait()
	err = multierror.Append(err, re.assetsErr)

	// Now we're ready to close InfluxDB.
	if re.influxdb != nil {
		re.influxdb.Close()
	}

	if l := re.logger; l != nil {
		_ = l.Sync()
	}

	return err.ErrorOrNil()
}

func (re *RunEnv) batchInsertInfluxDB(results *os.File) error {
	var (
		count  int
		points []*influxdb2.Point
	)

	wapib := re.influxdb.WriteApiBlocking("testground", "results")
	for dec := json.NewDecoder(results); dec.More(); {
		var m Metric
		if err := dec.Decode(&m); err != nil {
			re.RecordMessage("failed to decode Metric from results.out: %s", err)
			continue
		}

		// NewPoint copies all tags and fields, so this is thread-safe.
		p := influxdb2.NewPoint(m.Name, re.tags, m.Measures, time.Unix(0, m.Timestamp))
		p.AddTag("type", m.Type.String())
		points = append(points, p)
		count++

		// upload a batch every 500 points, or if this is the last point.
		if count%500 == 0 || !dec.More() {
			logger := func(n uint, err error) {
				re.RecordMessage("failed to upload result points on attempt %d to InfluxDB: %s", n, err)
			}

			write := func() error {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				return wapib.WritePoint(ctx, points...)
			}

			// retry 5 times, with a delay of 1 seconds, and the default jitter, logging each attempt
			// into the runenv.
			err := retry.Do(write, retry.Attempts(5), retry.Delay(1*time.Second), retry.OnRetry(logger))

			if err != nil {
				re.RecordMessage("failed completely to upload a batch of result points to InfluxDB: %s", err)
			}
			points = points[:0]
		}
	}
	return nil
}

// CurrentRunEnv populates a test context from environment vars.
func CurrentRunEnv() *RunEnv {
	re, _ := ParseRunEnv(os.Environ())
	return re
}

// ParseRunEnv parses a list of environment variables into a RunEnv.
func ParseRunEnv(env []string) (*RunEnv, error) {
	p, err := ParseRunParams(env)
	if err != nil {
		return nil, err
	}

	return NewRunEnv(*p), nil
}
