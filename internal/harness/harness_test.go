package harness_test

import (
	"context"
	"testing"
	"time"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/harness"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

func TestRunWithFakeBackends(t *testing.T) {
	t.Parallel()
	fakeOK := &runner.Fake{
		BackendName: "fake-ok",
		Avail:       true,
		Latency:     5 * time.Millisecond,
		Resources:   runner.ResourceUsage{PeakRSSBytes: 4096, MaxNProc: 16},
		Outcomes: map[runner.ProbeID]runner.ProbeResult{
			runner.ProbeReadHostFile:        {ID: runner.ProbeReadHostFile, Outcome: runner.OutcomeAllowed},
			runner.ProbeWriteOutsideWorkdir: {ID: runner.ProbeWriteOutsideWorkdir, Outcome: runner.OutcomeBlocked, Detail: "ro"},
			runner.ProbeOpenNetworkSocket:   {ID: runner.ProbeOpenNetworkSocket, Outcome: runner.OutcomeBlocked},
			runner.ProbeForkUnderLimit:      {ID: runner.ProbeForkUnderLimit, Outcome: runner.OutcomeBlocked},
		},
	}
	fakeSkip := &runner.Fake{
		BackendName: "fake-skip",
		Avail:       false,
		Reason:      "not installed",
	}

	rep, err := harness.Run(context.Background(), harness.Options{
		Backends: []runner.Backend{fakeOK, fakeSkip},
		Workload: runner.Workload{
			WorkDir: t.TempDir(),
			Probes:  runner.AllProbes,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Results) != 2 {
		t.Fatalf("got %d results", len(rep.Results))
	}
	if rep.Results[0].Skipped || len(rep.Results[0].Probes) != 4 {
		t.Fatalf("unexpected ok result: %+v", rep.Results[0])
	}
	if !rep.Results[1].Skipped || rep.Results[1].SkipReason != "not installed" {
		t.Fatalf("unexpected skip result: %+v", rep.Results[1])
	}
	if rep.Disclaimer == "" {
		t.Fatal("missing disclaimer")
	}
}
