package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/hashicorp/go-multierror"
	_ "github.com/influxdata/influxdb1-client" // this is important because of the bug in go mod
	client "github.com/influxdata/influxdb1-client/v2"
	"go.uber.org/zap"
)

var (
	InfluxBatching       = true
	InfluxBatchLength    = 128
	InfluxBatchInterval  = 1 * time.Second
	InfluxBatchRetryOpts = func(re *RunEnv) []retry.Option {
		return []retry.Option{
			retry.Attempts(5),
			retry.Delay(500 * time.Millisecond),
			retry.OnRetry(func(n uint, err error) {
				re.RecordMessage("failed to send batch to InfluxDB; attempt %d; err: %s", n, err)
			}),
		}
	}
)

// RunEnv encapsulates the context for this test run.
type RunEnv struct {
	RunParams

	logger *zap.Logger

	diagnostics *MetricsApi
	results     *MetricsApi
	influxdb    client.Client
	batcher     Batcher
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

	var dsinks = []MetricSinkFn{LogSinkJSON(re, "diagnostics.out")}
	client, err := NewInfluxDBClient(re)
	if err == nil {
		re.tags = map[string]string{
			"plan":     re.TestPlan,
			"case":     re.TestCase,
			"run":      re.TestRun,
			"group_id": re.TestGroupID,
		}

		re.influxdb = client
		if InfluxBatching {
			re.batcher = newBatcher(re, client, InfluxBatchLength, InfluxBatchInterval, InfluxBatchRetryOpts(re)...)
		} else {
			re.batcher = &nilBatcher{client}
		}

		dsinks = append(dsinks, WriteToInfluxDBSink(re, "diagnostics"))
	} else {
		re.RecordMessage("InfluxDB unavailable; no metrics will be dispatched: %s", err)
	}

	re.diagnostics = newMetricsApi(re, metricsApiOpts{
		freq:  1 * time.Second,
		sinks: dsinks,
	})

	re.results = newMetricsApi(re, metricsApiOpts{
		freq:  1 * time.Second,
		sinks: []MetricSinkFn{LogSinkJSON(re, "results.out")},
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
		// Next, we reopen the results.out file, and write all points to InfluxDB.
		results := filepath.Join(re.TestOutputsPath, "results.out")
		if file, errf := os.OpenFile(results, os.O_RDONLY, 0666); errf == nil {
			err = multierror.Append(err, re.batchInsertInfluxDB(file))
		} else {
			err = multierror.Append(err, errf)
		}
	}

	// Flush the immediate InfluxDB writer.
	if re.batcher != nil {
		err = multierror.Append(err, re.batcher.Close())
	}

	// This close stops monitoring the wapi errors channel, and closes assets.
	close(re.closeCh)
	re.wg.Wait()
	err = multierror.Append(err, re.assetsErr)

	// Now we're ready to close InfluxDB.
	if re.influxdb != nil {
		err = multierror.Append(err, re.influxdb.Close())
	}

	if l := re.logger; l != nil {
		_ = l.Sync()
	}

	return err.ErrorOrNil()
}

func (re *RunEnv) batchInsertInfluxDB(results *os.File) error {
	sink := WriteToInfluxDBSink(re, "results")

	for dec := json.NewDecoder(results); dec.More(); {
		var m Metric
		if err := dec.Decode(&m); err != nil {
			re.RecordMessage("failed to decode Metric from results.out: %s", err)
			continue
		}

		if err := sink(&m); err != nil {
			re.RecordMessage("failed to process Metric from results.out: %s", err)
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
