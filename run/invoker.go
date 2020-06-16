package run

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/debug"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

const (
	// These ports are the HTTP ports we'll attempt to bind to. If this instance
	// is running in a Docker container, binding to 6060 is safe. If it's a
	// local:exec run, these ports belong to the host, so starting more than one
	// instance will lead to a collision. Therefore we fallback to 0.
	HTTPPort         = 6060
	HTTPPortFallback = 0

	StateInitializedGlobal   = sync.State("initialized_global")
	StateInitializedGroupFmt = "initialized_group_%s"
)

// HTTPListenAddr will be set to the listener address _before_ the test case is
// invoked. If we were unable to start the listener, this value will be "".
var HTTPListenAddr string

// InitContext encapsulates a sync client, a net client, and global and
// group-scoped seq numbers assigned to this test instance by the sync service.
//
// The states we signal to acquire the global and group-scoped seq numbers are:
//  - initialized_global
//  - initialized_group_<id>
type InitContext struct {
	SyncClient sync.Interface
	NetClient  *network.Client
	GlobalSeq  int64
	GroupSeq   int64
}

// init can be safely invoked on a nil reference.
func (ic *InitContext) init(runenv *runtime.RunEnv) {
	var (
		grpstate  = sync.State(fmt.Sprintf(StateInitializedGroupFmt, runenv.TestGroupID))
		client    = sync.MustBoundClient(context.Background(), runenv)
		netclient = network.NewClient(ic.SyncClient, runenv)
	)

	netclient.MustWaitNetworkInitialized(context.Background())

	*ic = InitContext{
		SyncClient: client,
		NetClient:  netclient,
		GlobalSeq:  client.MustSignalEntry(context.Background(), StateInitializedGlobal),
		GroupSeq:   client.MustSignalEntry(context.Background(), grpstate),
	}
}

func (ic *InitContext) close() {
	if err := ic.SyncClient.Close(); err != nil {
		panic(err)
	}
}

type TestCaseFn func(env *runtime.RunEnv) error

// InitializedTestCaseFn allows users to indicate they want a basic
// initialization routine to be run before yielding control to the test case
// function itself.
//
// The initialization routine is common scaffolding that gets repeated across
// the test plans we've seen. We package it here in an attempt to keep your
// code try.
//
// It consists of:
//
//  1. Initializing a sync client, bound to the runenv.
//  2. Initializing a net client.
//  3. Waiting for the network to initialize.
//  4. Claiming a global sequence number.
//  5. Claiming a group-scoped sequence number.
//
// The injected InitContext is a bundle containing the result, and you can use
// its objects in your test logic. In fact, you don't need to close them
// (sync client, net client), as the SDK manages that for you.
type InitializedTestCaseFn func(env *runtime.RunEnv, initCtx *InitContext) error

// InvokeMap takes a map of test case names and their functions, and calls the
// matched test case, or panics if the name is unrecognised.
//
// Supported function signatures are TestCaseFn and InitializedTestCaseFn.
// Refer to their respective godocs for more info.
func InvokeMap(cases map[string]interface{}) {
	runenv := runtime.CurrentRunEnv()
	defer runenv.Close()

	if fn, ok := cases[runenv.TestCase]; ok {
		invoke(runenv, fn)
	} else {
		msg := fmt.Sprintf("unrecognized test case: %s", runenv.TestCase)
		panic(msg)
	}
}

// Invoke runs the passed test-case and reports the result.
//
// Supported function signatures are TestCaseFn and InitializedTestCaseFn.
// Refer to their respective godocs for more info.
func Invoke(fn interface{}) {
	runenv := runtime.CurrentRunEnv()
	defer runenv.Close()

	invoke(runenv, fn)
}

func invoke(runenv *runtime.RunEnv, fn interface{}) {
	setupHTTPListener(runenv)

	runenv.RecordStart()

	var err error
	errfile, err := runenv.CreateRawAsset("run.err")
	if err != nil {
		runenv.RecordCrash(err)
		return
	}

	rd, wr, err := os.Pipe()
	if err != nil {
		runenv.RecordCrash(err)
		return
	}

	w := io.MultiWriter(errfile, os.Stderr)
	os.Stderr = wr

	ioDoneCh := make(chan struct{})
	go func() {
		defer close(ioDoneCh)

		_, err := io.Copy(w, rd)
		if err != nil && !strings.Contains(err.Error(), "file already closed") {
			runenv.RecordCrash(fmt.Errorf("stderr copy failed: %w", err))
			return
		}

		if err = errfile.Sync(); err != nil {
			runenv.RecordCrash(fmt.Errorf("stderr file tee sync failed failed: %w", err))
		}
	}()

	// Prepare the event.
	defer func() {
		if err := recover(); err != nil {
			// Handle panics by recording them in the runenv output.
			runenv.RecordCrash(err)

			// Developers expect panics to be recorded in run.err too.
			_, _ = fmt.Fprintln(os.Stderr, err)
			debug.PrintStack()
		}
	}()

	switch f := fn.(type) {
	case TestCaseFn:
		err = f(runenv)
	case InitializedTestCaseFn:
		ic := new(InitContext)
		ic.init(runenv)
		defer ic.close()
		err = f(runenv, ic)
	default:
		msg := fmt.Sprintf("unexpected function passed to Invoke*; expected types: TestCaseFn, InitializedTestCaseFn; was: %T", f)
		panic(msg)
	}

	switch err {
	case nil:
		runenv.RecordSuccess()
	default:
		runenv.RecordFailure(err)
	}

	_ = rd.Close()
	<-ioDoneCh
	runenv.RecordMessage("io closed")
}

func setupHTTPListener(runenv *runtime.RunEnv) {
	addr := fmt.Sprintf("0.0.0.0:%d", HTTPPort)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		addr = fmt.Sprintf("0.0.0.0:%d", HTTPPortFallback)
		if l, err = net.Listen("tcp", addr); err != nil {
			runenv.RecordMessage("error registering default http handler at: %s: %s", addr, err)
			return
		}
	}

	// DefaultServeMux already includes the pprof handler, add the
	// Prometheus handler.
	http.DefaultServeMux.Handle("/metrics", promhttp.Handler())

	HTTPListenAddr = l.Addr().String()

	runenv.RecordMessage("registering default http handler at: http://%s/ (pprof: http://%s/debug/pprof/)", HTTPListenAddr, HTTPListenAddr)

	go func() {
		_ = http.Serve(l, nil)
	}()
}
