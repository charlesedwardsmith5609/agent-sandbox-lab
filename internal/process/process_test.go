package process_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/process"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

func TestProcessBackendRun(t *testing.T) {
	self := buildASL(t)
	b := &process.Backend{Self: self}
	ok, reason := b.Available(context.Background())
	if !ok {
		t.Fatalf("process should always be available: %s", reason)
	}

	res, err := b.Run(context.Background(), runner.Workload{
		WorkDir:     t.TempDir(),
		Probes:      runner.AllProbes,
		HostFile:    "/etc/hosts",
		MaxNProc:    16,
		MaxMemoryMB: 128,
		Timeout:     20 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped {
		t.Fatalf("unexpected skip: %s", res.SkipReason)
	}
	if res.Error != "" {
		t.Fatalf("backend error: %s", res.Error)
	}
	if len(res.Probes) != len(runner.AllProbes) {
		t.Fatalf("probe count=%d want %d: %+v", len(res.Probes), len(runner.AllProbes), res.Probes)
	}
	if res.StartupLatency <= 0 {
		t.Fatal("expected positive startup latency")
	}
	// On a normal host, reading /etc/hosts should be allowed in the process backend.
	found := false
	for _, p := range res.Probes {
		if p.ID == runner.ProbeReadHostFile {
			found = true
			if p.Outcome != runner.OutcomeAllowed && p.Outcome != runner.OutcomeBlocked {
				t.Fatalf("unexpected read outcome: %+v", p)
			}
		}
	}
	if !found {
		t.Fatal("missing read_host_file probe")
	}
}

func buildASL(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "asl")
	cmd := exec.Command("go", "build", "-o", out, "github.com/charlesedwardsmith5609/agent-sandbox-lab/cmd/asl")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Dir = moduleRoot(t)
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, outBytes)
	}
	return out
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
