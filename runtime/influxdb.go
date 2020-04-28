package runtime

import (
	"context"
	"fmt"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go"
)

const (
	EnvInfluxDBURL       = "INFLUXDB_URL"
	EnvInfluxDBAuthToken = "INFLUXDB_AUTH"
)

func NewInfluxDBClient() (influxdb2.InfluxDBClient, error) {
	url := os.Getenv(EnvInfluxDBURL)
	if url == "" {
		return nil, fmt.Errorf("no InfluxDB URL in $%s env var", EnvInfluxDBURL)
	}

	auth := os.Getenv(EnvInfluxDBAuthToken)
	if auth == "" {
		return nil, fmt.Errorf("no InfluxDB auth token in $%s env var", EnvInfluxDBAuthToken)
	}

	opts := influxdb2.DefaultOptions()
	opts.SetMaxRetries(10)
	opts.SetHttpRequestTimeout(30)
	opts.SetUseGZip(true)

	client := influxdb2.NewClientWithOptions(url, auth, opts)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if ok, err := client.Ready(ctx); err != nil || !ok {
		return nil, fmt.Errorf("influxdb not ready: %w", err)
	}

	return client, nil
}
