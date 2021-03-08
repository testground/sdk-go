package run

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/raulk/clock"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/stretchr/testify/require"
)

func init() {
	syncClient := sync.NewInmemClient()
	InitSyncClientFactory = func(_ context.Context, _ *runtime.RunEnv) sync.Client {
		return syncClient
	}
}

func TestInitializedInvoke(t *testing.T) {
	nextGlobalSeq := 0
	nextGroupSeq := make(map[string]int, 2)
	var test InitializedTestCaseFn = func(env *runtime.RunEnv, initCtx *InitContext) error {
		// the test case performs asserts on the RunEnv and InitContext.
		require.NotNil(t, env)
		require.NotNil(t, initCtx)
		require.NotNil(t, initCtx.SyncClient)
		require.NotNil(t, initCtx.NetClient)

		// keep track of the expected seq numbers.
		nextGlobalSeq++
		nextGroupSeq[env.TestGroupID] = nextGroupSeq[env.TestGroupID] + 1

		require.EqualValues(t, int64(nextGlobalSeq), initCtx.GlobalSeq)
		require.EqualValues(t, int64(nextGroupSeq[env.TestGroupID]), initCtx.GroupSeq)
		return nil
	}

	env, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)

	for k, v := range env.ToEnvVars() {
		_ = os.Setenv(k, v)
	}

	// we simulate starting many instances by calling invoke multiple times.
	// all invocations are backed by the same inmem sync service instance.
	Invoke(test)
	Invoke(test)
	Invoke(test)
}

func TestUninitializedInvoke(t *testing.T) {
	env, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)

	for k, v := range env.ToEnvVars() {
		_ = os.Setenv(k, v)
	}

	await := func(ch chan struct{}) {
		select {
		case <-ch:
		case <-time.After(1 * time.Second):
			t.Fatal("test function not invoked")
		}
	}

	// not using type alias.
	ch := make(chan struct{})
	Invoke(func(runenv *runtime.RunEnv) error {
		close(ch)
		return nil
	})
	await(ch)

	// using type alias.
	ch = make(chan struct{})
	Invoke(TestCaseFn(func(runenv *runtime.RunEnv) error {
		close(ch)
		return nil
	}))
	await(ch)

}

func TestProfiles(t *testing.T) {
	clk := clock.NewMock()
	_clk = clk

	env, cleanup := runtime.RandomTestRunEnv(t)
	t.Cleanup(cleanup)

	env.TestCaptureProfiles = map[string]string{
		"cpu":         "foo", // disregarded
		"heap":        "5s",
		"allocs":      "10s",
		"goroutine":   "30s",
		"block":       "1h", // will never be written
		"unsupported": "foo",
	}

	stop, err := captureProfiles(env)
	require.Error(t, err) // error due to "unsupported" profile.
	require.Equal(t, 1, err.(*multierror.Error).Len())
	defer stop()

	time.Sleep(500 * time.Millisecond) // allow goroutines to start.

	for i := 1; i <= 6; i++ {
		// advance the clock 5 seconds; sleep 500ms for goroutines to execute.
		clk.Add(5 * time.Second)
		time.Sleep(500 * time.Millisecond)
	}

	// stop.
	require.NoError(t, stop())

	// 1 cpu profile.
	matches, _ := filepath.Glob(filepath.Join(env.TestOutputsPath, "cpu.prof"))
	require.Len(t, matches, 1)

	// 6 heap profiles.
	matches, _ = filepath.Glob(filepath.Join(env.TestOutputsPath, "heap.*.prof"))
	require.Len(t, matches, 6)

	// 3 allocs profiles.
	matches, _ = filepath.Glob(filepath.Join(env.TestOutputsPath, "allocs.*.prof"))
	require.Len(t, matches, 3)

	// 1 goroutine profiles.
	matches, _ = filepath.Glob(filepath.Join(env.TestOutputsPath, "goroutine.*.prof"))
	require.Len(t, matches, 1)

	// no more profiles.
	matches, _ = filepath.Glob(filepath.Join(env.TestOutputsPath, "*.prof"))
	require.Len(t, matches, 11)
}
