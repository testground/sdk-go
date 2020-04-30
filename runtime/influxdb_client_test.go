package runtime

import (
	"fmt"
	"sync"
	"time"

	_ "github.com/influxdata/influxdb1-client"
	client "github.com/influxdata/influxdb1-client/v2"
)

type testClient struct {
	sync.RWMutex

	fail        bool
	batchPoints []client.BatchPoints
}

var _ client.Client = (*testClient)(nil)

func (t *testClient) EnableFail(fail bool) {
	t.Lock()
	defer t.Unlock()

	t.fail = fail
}

func (t *testClient) Ping(_ time.Duration) (time.Duration, string, error) {
	return 0, "", nil
}

func (t *testClient) Write(bp client.BatchPoints) error {
	t.Lock()
	defer t.Unlock()

	t.batchPoints = append(t.batchPoints, bp)

	var err error
	if t.fail {
		err = fmt.Errorf("error")
	}
	return err
}

func (t *testClient) Query(_ client.Query) (*client.Response, error) {
	return nil, nil
}

func (t *testClient) QueryAsChunk(_ client.Query) (*client.ChunkedResponse, error) {
	return nil, nil
}

func (t *testClient) Close() error {
	return nil
}
