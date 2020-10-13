package run

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testground/sdk-go/runtime"
)

func runWithTempDir(t *testing.T, f func()) *os.File {
	t.Helper()

	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpdir) })

	_ = os.Setenv(runtime.EnvTestOutputsPath, tmpdir)
	f()

	runout, err := os.Open(filepath.Join(tmpdir, "run.out"))
	if err != nil {
		t.Fatal(err)
	}

	return runout
}

func TestPanicFromMain(t *testing.T) {
	var tc TestCaseFn = func(runenv *runtime.RunEnv) error {
		panic("bang!")
	}

	runout := runWithTempDir(t, func() { Invoke(tc) })

	var last string
	for scanner := bufio.NewScanner(runout); scanner.Scan(); {
		last = scanner.Text()
	}

	if !strings.Contains(last, "\"crash_event\"") {
		t.Fatalf("expected crashed event; got: %s", last)
	}
}

func TestPanicFromChildGoroutine(t *testing.T) {
	var tc TestCaseFn = func(runenv *runtime.RunEnv) error {
		go func() {
			defer HandlePanics()
			panic("bang!")
		}()

		// test case hangs.
		time.Sleep(1 * time.Hour)
		return nil
	}

	runout := runWithTempDir(t, func() { Invoke(tc) })

	var last string
	for scanner := bufio.NewScanner(runout); scanner.Scan(); {
		last = scanner.Text()
	}

	if !strings.Contains(last, "\"crash_event\"") {
		t.Fatalf("expected crashed event; got: %s", last)
	}
}
