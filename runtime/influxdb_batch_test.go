package runtime

import (
	"testing"
	"time"

	"github.com/avast/retry-go"
	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/stretchr/testify/require"
)

func TestLengthBatching(t *testing.T) {
	runenv, cleanup := RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

	tc := &testClient{}
	b := newBatcher(runenv, tc, 16, 24*time.Hour)

	writePoints(t, b, 0, 36)

	time.Sleep(1 * time.Second)

	require := require.New(t)

	// we should've received two batches.
	tc.RLock()
	require.Len(tc.batchPoints, 2)
	require.Len(tc.batchPoints[0].Points(), 16)
	require.Len(tc.batchPoints[1].Points(), 16)
	tc.RUnlock()

	require.NoError(b.Close())
	tc.RLock()
	require.Len(tc.batchPoints, 3)
	require.Len(tc.batchPoints[2].Points(), 4)
	tc.RUnlock()
}

func TestIntervalBatching(t *testing.T) {
	runenv, cleanup := RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

	tc := &testClient{}
	b := newBatcher(runenv, tc, 1000, 500*time.Millisecond)

	writePoints(t, b, 0, 10)

	time.Sleep(2 * time.Second)

	require := require.New(t)

	// we should've received two batches.
	tc.RLock()
	require.Len(tc.batchPoints, 1)
	require.Len(tc.batchPoints[0].Points(), 10)
	tc.RUnlock()

	require.NoError(b.Close())
	tc.RLock()
	require.Len(tc.batchPoints, 1)
	tc.RUnlock()
}

func TestBatchFailure(t *testing.T) {
	runenv, cleanup := RandomTestRunEnv(t)
	t.Cleanup(cleanup)
	defer runenv.Close()

	test := func(b *batcher) func(t *testing.T) {
		tc := &testClient{}
		b.client = tc

		return func(t *testing.T) {

			// Enable failures.
			tc.EnableFail(true)

			// Write three batches of 10 points each.
			writePoints(t, b, 0, 10)
			writePoints(t, b, 10, 10)
			writePoints(t, b, 20, 10)

			time.Sleep(2 * time.Second)

			require := require.New(t)

			// we should've received the same batch many times.
			tc.RLock()
			require.Greater(len(tc.batchPoints), 1)
			assertPointsExactly(t, tc.batchPoints[0], 0, 10)
			tc.RUnlock()

			// get out of failure mode.
			tc.EnableFail(false)

			// wait for the retries to be done.
			time.Sleep(2 * time.Second)

			// now the last four elements should be:
			// batch(0-9) (failed), batch(0-9) (ok), batch(10-19) (ok), batch(20-29) (ok)
			tc.RLock()
			require.Greater(len(tc.batchPoints), 1)
			assertPointsExactly(t, tc.batchPoints[len(tc.batchPoints)-4], 0, 10)
			assertPointsExactly(t, tc.batchPoints[len(tc.batchPoints)-3], 0, 10)
			assertPointsExactly(t, tc.batchPoints[len(tc.batchPoints)-2], 10, 10)
			assertPointsExactly(t, tc.batchPoints[len(tc.batchPoints)-1], 20, 10)
			tc.RUnlock()
		}
	}

	t.Run("batches_by_length", test(newBatcher(runenv, nil, 10, 24*time.Hour,
		retry.Attempts(3),
		retry.Delay(100*time.Millisecond),
	)))

	t.Run("batches_by_time", test(newBatcher(runenv, nil, 10, 100*time.Millisecond,
		retry.Attempts(3),
		retry.Delay(100*time.Millisecond),
	)))

}

func writePoints(t *testing.T, b *batcher, offset, count int) {
	t.Helper()

	for i := offset; i < offset+count; i++ {
		tags := map[string]string{}
		fields := map[string]interface{}{
			"i": i,
		}
		p, err := client.NewPoint("point", tags, fields)
		if err != nil {
			t.Fatal(err)
		}
		b.WritePoint(p)
	}
}

func assertPointsExactly(t *testing.T, bp client.BatchPoints, offset, length int) {
	t.Helper()

	if l := len(bp.Points()); l != length {
		t.Fatalf("length did not match; expected: %d, got %d", length, l)
	}

	for i, p := range bp.Points() {
		f, _ := p.Fields()
		if actual := f["i"].(int64); int64(i+offset) != actual {
			t.Fatalf("comparison failed; expected: %d, got %d", i+offset, actual)
		}
	}
}
