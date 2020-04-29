package runtime

import (
	"fmt"
	"os"
	"time"

	_ "github.com/influxdata/influxdb1-client" // this is important because of the bug in go mod
	client "github.com/influxdata/influxdb1-client/v2"
)

const EnvInfluxDBAddr = "INFLUXDB_ADDR"

var (
	// TestInfluxDBClient sets a client for testing. If this value is set,
	// NewInfluxDBClient will always return it.
	TestInfluxDBClient client.Client
)

func NewInfluxDBClient(re *RunEnv) (client.Client, error) {
	if TestInfluxDBClient != nil {
		return TestInfluxDBClient, nil
	}

	addr := os.Getenv(EnvInfluxDBAddr)
	if addr == "" {
		return nil, fmt.Errorf("no InfluxDB address in $%s env var", EnvInfluxDBAddr)
	}

	cfg := client.HTTPConfig{Addr: addr, Timeout: 5 * time.Second}
	return client.NewHTTPClient(cfg)
}
