package runtime

import (
	"encoding/json"
	"time"

	"github.com/influxdata/influxdb-client-go"
)

func LogSinkJSON(re *RunEnv, filename string) SinkFn {
	f, err := re.CreateRawAsset(filename)
	if err != nil {
		panic(err)
	}

	enc := json.NewEncoder(f)
	return func(m *Metric) error {
		return enc.Encode(m)
	}
}

func WriteToInfluxDB(re *RunEnv) SinkFn {
	return func(m *Metric) error {
		// NewPoint copies all tags and fields, so this is thread-safe.
		p := influxdb2.NewPoint(m.Name, re.tags, m.Measures, time.Unix(0, m.Timestamp))
		p.AddTag("type", m.Type.String())
		re.wapi.WritePoint(p)
		return nil
	}
}
