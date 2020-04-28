package test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"

	"github.com/testground/sdk-go/runtime"
)

// RandomRunEnv generates a random RunEnv for testing purposes.
func RandomRunEnv(t *testing.T) (re *runtime.RunEnv, cleanup func()) {
	t.Helper()

	b := make([]byte, 32)
	_, _ = rand.Read(b)

	_, subnet, _ := net.ParseCIDR("127.1.0.1/16")

	odir, err := ioutil.TempDir("", "testground-tests-*")
	if err != nil {
		t.Fatalf("failed to create temp output dir: %s", err)
	}

	rp := runtime.RunParams{
		TestPlan:               fmt.Sprintf("testplan-%d", rand.Uint32()),
		TestSidecar:            false,
		TestCase:               fmt.Sprintf("testcase-%d", rand.Uint32()),
		TestRun:                fmt.Sprintf("testrun-%d", rand.Uint32()),
		TestSubnet:             &runtime.IPNet{IPNet: *subnet},
		TestInstanceCount:      int(1 + (rand.Uint32() % 999)),
		TestInstanceRole:       "",
		TestInstanceParams:     make(map[string]string),
		TestGroupID:            fmt.Sprintf("group-%d", rand.Uint32()),
		TestStartTime:          time.Now(),
		TestGroupInstanceCount: int(1 + (rand.Uint32() % 999)),
		TestOutputsPath:        odir,
	}

	return runtime.NewRunEnv(rp), func() {
		_ = os.RemoveAll(odir)
	}
}
