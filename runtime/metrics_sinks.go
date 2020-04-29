package runtime

import (
	"encoding/json"
	"time"

	client "github.com/influxdata/influxdb1-client/v2"
)

func LogSinkJSON(re *RunEnv, filename string) MetricSinkFn {
	f, err := re.CreateRawAsset(filename)
	if err != nil {
		panic(err)
	}

	enc := json.NewEncoder(f)
	return func(m *Metric) error {
		return enc.Encode(m)
	}
}

func WriteToInfluxDBSink(re *RunEnv, name string) MetricSinkFn {
	return func(m *Metric) error {
		p, err := client.NewPoint(name, re.tags, m.Measures, time.Unix(0, m.Timestamp))
		if err != nil {
			return err
		}
		re.batcher.WritePoint(p)
		return nil
	}
}
