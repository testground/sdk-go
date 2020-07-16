package run

import (
	"fmt"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/debug"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/testground/sdk-go"
	"github.com/testground/sdk-go/runtime"
)

const (
	// These ports are the HTTP ports we'll attempt to bind to. If this instance
	// is running in a Docker container, binding to 6060 is safe. If it's a
	// local:exec run, these ports belong to the host, so starting more than one
	// instance will lead to a collision. Therefore we fallback to 0.
	HTTPPort         = 6060
	HTTPPortFallback = 0
)

// HTTPListenAddr will be set to the listener address _before_ the test case is
// invoked. If we were unable to start the listener, this value will be "".
var HTTPListenAddr string

type TestCaseFn func(env *runtime.RunEnv) error

// InitializedTestCaseFn allows users to indicate they want a basic
// initialization routine to be run before yielding control to the test case
// function itself.
//
// The initialization routine is common scaffolding that gets repeated across
// the test plans we've seen. We package it here in an attempt to keep your
// code DRY.
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
	maybeSetupHTTPListener(runenv)

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

	// handle the copying of stderr into run.err.
	go func() {
		defer func() {
			_ = rd.Close()
			if sdk.Verbose {
				runenv.RecordMessage("io closed")
			}
		}()

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

	errCh := make(chan error)
	go func() {
		defer close(errCh)
		defer HandlePanics()

		switch f := fn.(type) {
		case TestCaseFn:
			errCh <- f(runenv)
		case InitializedTestCaseFn:
			ic := new(InitContext)
			ic.init(runenv)
			defer ic.close()
			errCh <- f(runenv, ic)
		default:
			msg := fmt.Sprintf("unexpected function passed to Invoke*; expected types: TestCaseFn, InitializedTestCaseFn; was: %T", f)
			panic(msg)
		}
	}()

	select {
	case err := <-errCh:
		switch err {
		case nil:
			runenv.RecordSuccess()
		default:
			runenv.RecordFailure(err)
		}
	case p := <-panicHandler:
		// propagate the panic.
		panic(p)
	}
}

func maybeSetupHTTPListener(runenv *runtime.RunEnv) {
	if HTTPListenAddr != "" {
		// already set up.
		return
	}

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
