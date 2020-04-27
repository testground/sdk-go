package runtime

import (
	"os"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	influxdb2 "github.com/influxdata/influxdb-client-go"
	"go.uber.org/zap"
)

// RunEnv encapsulates the context for this test run.
type RunEnv struct {
	RunParams
	*logger

	diagnostics *MetricsApi
	results     *MetricsApi
	influxdb    influxdb2.InfluxDBClient
	wapi        influxdb2.WriteApi

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

// NewRunEnv constructs a runtime environment from the given runtime parameters.
func NewRunEnv(params RunParams) *RunEnv {
	re := &RunEnv{
		RunParams: params,
		logger:    newLogger(&params),
	}

	re.structured.ch = make(chan *zap.Logger)
	re.unstructured.ch = make(chan *os.File)

	re.wg.Add(1)
	go re.manageAssets()

	var dsinks = []SinkFn{LogSinkJSON(re, "diagnostics.out")}
	client, err := NewInfluxDBClient()
	if err == nil {
		re.influxdb = client
		wapi := client.WriteApi("testground", "diagnostics")
		dsinks = append(dsinks, WriteToInfluxDB(re, wapi))

		re.wg.Add(1)
		go re.monitorInfluxDBErrors()
	} else {
		re.logger.RecordMessage("InfluxDB unavailable; no metrics will be dispatched: %s", err)
	}

	re.diagnostics = newMetricsApi(re, metricsApiOpts{
		prefix: "diag.",
		freq:   1 * time.Second,
		sinks:  dsinks,
	})

	re.results = newMetricsApi(re, metricsApiOpts{
		prefix: "results.",
		freq:   1 * time.Second,
		sinks:  []SinkFn{LogSinkJSON(re, "results.out")},
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
	err = multierror.Append(re.diagnostics.Close())
	err = multierror.Append(re.results.Close())

	if l := re.logger; l != nil {
		_ = l.SLogger().Sync()
	}

	close(re.closeCh)
	re.wg.Wait()
	err = multierror.Append(err, re.assetsErr)

	if re.wapi != nil {
		re.wapi.Flush()
		re.wapi.Close()
	}

	if re.influxdb != nil {
		re.influxdb.Close()
	}

	return err.ErrorOrNil()
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
