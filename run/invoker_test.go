package run

import (
	"context"
	"os"
	"testing"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/stretchr/testify/require"
)

func init() {
	syncClient := sync.NewInmemClient()
	InitSyncClientFactory = func(_ context.Context, _ *runtime.RunEnv) sync.Interface {
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
	defer cleanup()

	for k, v := range env.ToEnvVars() {
		_ = os.Setenv(k, v)
	}

	// we simulate starting many instances by calling invoke multiple times.
	// all invocations are backed by the same inmem sync service instance.
	Invoke(test)
	Invoke(test)
	Invoke(test)
}
