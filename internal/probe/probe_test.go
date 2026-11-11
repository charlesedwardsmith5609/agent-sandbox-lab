package probe_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/probe"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

func TestReadAndWriteProbes(t *testing.T) {
	t.Parallel()
	work := t.TempDir()
	host := filepath.Join(t.TempDir(), "host.txt")
	if err := os.WriteFile(host, []byte("secret-ish"), 0o600); err != nil {
		t.Fatal(err)
	}

	results := probe.RunAll(
		[]runner.ProbeID{runner.ProbeReadHostFile, runner.ProbeWriteOutsideWorkdir, runner.ProbeOpenNetworkSocket},
		work,
		host,
		8,
	)
	if len(results) != 3 {
		t.Fatalf("got %d", len(results))
	}
	if results[0].Outcome != runner.OutcomeAllowed {
		t.Fatalf("expected host read allowed: %+v", results[0])
	}
	// Writing under /tmp is usually allowed for a normal process.
	if results[1].Outcome != runner.OutcomeAllowed && results[1].Outcome != runner.OutcomeBlocked {
		t.Fatalf("unexpected write outcome: %+v", results[1])
	}
	if results[2].Outcome != runner.OutcomeAllowed && results[2].Outcome != runner.OutcomeBlocked {
		t.Fatalf("unexpected network outcome: %+v", results[2])
	}
}

func TestParseProbeList(t *testing.T) {
	t.Parallel()
	got := probe.ParseProbeList("read_host_file, open_network_socket")
	if len(got) != 2 || got[0] != runner.ProbeReadHostFile || got[1] != runner.ProbeOpenNetworkSocket {
		t.Fatalf("%v", got)
	}
	all := probe.ParseProbeList("")
	if len(all) != len(runner.AllProbes) {
		t.Fatalf("%v", all)
	}
}
