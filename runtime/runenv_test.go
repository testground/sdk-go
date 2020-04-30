package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseKeyValues(t *testing.T) {
	type args struct {
		in []string
	}
	tests := []struct {
		name    string
		args    args
		wantRes map[string]string
		wantErr bool
	}{
		{
			name: "empty, int, string, bool",
			args: args{
				[]string{
					"TEST_INSTANCE_ROLE=",
					"TEST_ARTIFACTS=/artifacts",
					"TEST_SIDECAR=true",
				},
			},
			wantErr: false,
			wantRes: map[string]string{
				"TEST_INSTANCE_ROLE": "",
				"TEST_ARTIFACTS":     "/artifacts",
				"TEST_SIDECAR":       "true",
			},
		},
		{
			name: "empty, string, int, complex",
			args: args{
				[]string{
					"TEST_BRANCH=",
					"TEST_RUN=e765696a-bdf2-408e-8b39-aeb0e90c0ff6",
					"TEST_GROUP_INSTANCE_COUNT=200",
					"TEST_GROUP_ID=single",
					"TEST_INSTANCE_PARAMS=bucket_size=2|n_find_peers=1|timeout_secs=300|auto_refresh=true|random_walk=false|n_bootstrap=1",
					"TEST_SUBNET=30.38.0.0/16",
				},
			},
			wantErr: false,
			wantRes: map[string]string{
				"TEST_BRANCH":               "",
				"TEST_RUN":                  "e765696a-bdf2-408e-8b39-aeb0e90c0ff6",
				"TEST_GROUP_INSTANCE_COUNT": "200",
				"TEST_GROUP_ID":             "single",
				"TEST_INSTANCE_PARAMS":      "bucket_size=2|n_find_peers=1|timeout_secs=300|auto_refresh=true|random_walk=false|n_bootstrap=1",
				"TEST_SUBNET":               "30.38.0.0/16",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRes, err := ParseKeyValues(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKeyValues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("ParseKeyValues() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func TestAllEvents(t *testing.T) {
	re, cleanup := RandomTestRunEnv(t)
	t.Cleanup(cleanup)

	re.RecordStart()
	re.RecordFailure(fmt.Errorf("bang"))
	re.RecordCrash(fmt.Errorf("terrible bang"))
	re.RecordMessage("i have something to %s", "say")
	re.RecordSuccess()

	if err := re.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.OpenFile(re.TestOutputsPath+"/run.out", os.O_RDONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	require := require.New(t)

	var i int
	for dec := json.NewDecoder(file); dec.More(); {
		var m = struct {
			Event Event `json:"event"`
		}{}
		if err := dec.Decode(&m); err != nil {
			t.Fatal(err)
		}

		switch evt := m.Event; i {
		case 0:
			require.Equal(EventTypeMessage, evt.Type)
			require.Condition(func() bool { return strings.HasPrefix(evt.Message, "InfluxDB unavailable") })
		case 1:
			require.Equal(EventTypeStart, evt.Type)
			require.Equal(evt.Runenv.TestPlan, re.TestPlan)
			require.Equal(evt.Runenv.TestCase, re.TestCase)
			require.Equal(evt.Runenv.TestRun, re.TestRun)
			require.Equal(evt.Runenv.TestGroupID, re.TestGroupID)
		case 2:
			require.Equal(EventTypeFinish, evt.Type)
			require.Equal(EventOutcomeFailed, evt.Outcome)
			require.Equal("bang", evt.Error)
		case 3:
			require.Equal(EventTypeFinish, evt.Type)
			require.Equal(EventOutcomeCrashed, evt.Outcome)
			require.Equal("terrible bang", evt.Error)
			require.NotEmpty(evt.Stacktrace)
		case 4:
			require.Equal(EventTypeMessage, evt.Type)
		case 5:
			require.Equal(evt.Type, EventTypeFinish)
			require.Equal(evt.Outcome, EventOutcomeOK)
		}
		i++
	}
}

func TestMetricsRecordedInFile(t *testing.T) {
	test := func(f func(*RunEnv) *MetricsApi, file string) func(t *testing.T) {
		return func(t *testing.T) {
			re, cleanup := RandomTestRunEnv(t)
			t.Cleanup(cleanup)

			api := f(re)

			names := []string{"point1", "point2", "counter1", "meter1", "timer1"}
			types := []string{"point", "counter", "meter", "timer"}
			api.SetFrequency(200 * time.Millisecond)
			api.RecordPoint("point1", 123)
			api.RecordPoint("point2", 123)
			api.NewCounter("counter1").Inc(50)
			api.NewMeter("meter1").Mark(50)
			api.NewTimer("timer1").Update(5 * time.Second)

			time.Sleep(1 * time.Second)

			_ = re.Close()

			file, err := os.OpenFile(filepath.Join(re.TestOutputsPath, file), os.O_RDONLY, 0644)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()

			var metrics []*Metric
			for dec := json.NewDecoder(file); dec.More(); {
				var m *Metric
				if err := dec.Decode(&m); err != nil {
					t.Fatal(err)
				}
				metrics = append(metrics, m)
			}

			require := require.New(t)

			na := make(map[string]struct{})
			ty := make(map[string]struct{})
			for _, m := range metrics {
				require.Greater(m.Timestamp, int64(0))
				na[m.Name] = struct{}{}
				ty[m.Type.String()] = struct{}{}
				require.NotZero(len(m.Measures))
			}

			namesActual := make([]string, 0, len(na))
			for k := range na {
				namesActual = append(namesActual, k)
			}

			typesActual := make([]string, 0, len(ty))
			for k := range ty {
				typesActual = append(typesActual, k)
			}

			require.ElementsMatch(names, namesActual)
			require.ElementsMatch(types, typesActual)
		}
	}

	t.Run("diagnostics", test((*RunEnv).D, "diagnostics.out"))
	t.Run("results", test((*RunEnv).R, "results.out"))
}

func TestDiagnosticsDispatchedToInfluxDB(t *testing.T) {
	InfluxBatching = false
	tc := &testClient{}
	TestInfluxDBClient = tc

	re, cleanup := RandomTestRunEnv(t)
	t.Cleanup(cleanup)

	re.D().RecordPoint("foo", 1234)
	re.D().RecordPoint("foo", 1234)
	re.D().RecordPoint("foo", 1234)
	re.D().RecordPoint("foo", 1234)

	require := require.New(t)

	tc.RLock()
	require.Len(tc.batchPoints, 4)
	tc.RUnlock()

	re.D().SetFrequency(500 * time.Millisecond)
	re.D().NewCounter("counter").Inc(100)
	re.D().NewHistogram("histogram1", re.D().NewUniformSample(100)).Update(123)

	time.Sleep(1500 * time.Millisecond)

	tc.RLock()
	if l := len(tc.batchPoints); l != 6 && l != 8 && l != 10 {
		t.Fatalf("expected length to be 6, 8, or 10; was: %d", l)
	}
	tc.RUnlock()

	_ = re.Close()
}

func TestResultsDispatchedOnClose(t *testing.T) {
	InfluxBatching = false
	tc := &testClient{}
	TestInfluxDBClient = tc

	re, cleanup := RandomTestRunEnv(t)
	t.Cleanup(cleanup)

	re.R().RecordPoint("foo", 1234)
	re.R().RecordPoint("foo", 1234)
	re.R().RecordPoint("foo", 1234)
	re.R().RecordPoint("foo", 1234)

	require := require.New(t)

	tc.RLock()
	require.Empty(tc.batchPoints)
	tc.RUnlock()

	re.R().SetFrequency(500 * time.Millisecond)
	re.R().NewCounter("counter").Inc(100)
	re.R().NewHistogram("histogram1", re.D().NewUniformSample(100)).Update(123)

	time.Sleep(1500 * time.Millisecond)

	tc.RLock()
	require.Empty(tc.batchPoints)
	tc.RUnlock()

	_ = re.Close()

	tc.RLock()
	require.NotEmpty(tc.batchPoints)
	tc.RUnlock()
}
