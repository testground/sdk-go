package runtime

import (
	"fmt"
	"os"
	"time"

	"github.com/avast/retry-go"

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
		return nil, fmt.Errorf("no InfluxDB URL in $%s env var", EnvInfluxDBAddr)
	}

	cfg := client.HTTPConfig{Addr: addr, Timeout: 5}
	client, err := client.NewHTTPClient(cfg)
	if err != nil {
		return nil, err
	}

	ping := func() error {
		_, _, err := client.Ping(2 * time.Second)
		return err
	}
	err = retry.Do(ping,
		retry.Attempts(5),
		retry.MaxDelay(500*time.Millisecond),
		retry.OnRetry(func(n uint, err error) {
			re.RecordMessage("failed attempt number %d to ping InfluxDB at %s: %s", n, addr, err)
		}),
	)
	return client, err
}
