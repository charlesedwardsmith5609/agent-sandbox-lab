package report_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/harness"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/report"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

func TestJSONAndMarkdownTable(t *testing.T) {
	t.Parallel()

	base := harness.Report{
		GeneratedAt: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
		WorkDir:     "/tmp/asl",
		Disclaimer:  "test disclaimer",
	}

	cases := []struct {
		name          string
		results       []runner.Result
		wantJSONKeys  []string
		wantMDSubstr  []string
		wantNotMD     []string
	}{
		{
			name: "completed backend",
			results: []runner.Result{{
				Backend:        "process",
				StartupLatency: 3 * time.Millisecond,
				Resources:      runner.ResourceUsage{PeakRSSBytes: 2048, MaxNProc: 32, Notes: "rlimit"},
				Probes: []runner.ProbeResult{
					{ID: runner.ProbeReadHostFile, Outcome: runner.OutcomeAllowed, Detail: "ok"},
					{ID: runner.ProbeWriteOutsideWorkdir, Outcome: runner.OutcomeBlocked, Detail: "erofs"},
				},
			}},
			wantJSONKeys: []string{"generated_at", "results", "disclaimer", "startup_latency_ns"},
			wantMDSubstr: []string{"# agent-sandbox-lab report", "`process`", "`blocked`", "Peak RSS"},
		},
		{
			name: "skipped backend",
			results: []runner.Result{{
				Backend:    "gvisor",
				Skipped:    true,
				SkipReason: "runsc not found on PATH",
			}},
			wantJSONKeys: []string{"skip_reason", "skipped"},
			wantMDSubstr: []string{"Skipped:", "runsc not found"},
		},
		{
			name: "error backend",
			results: []runner.Result{{
				Backend: "container",
				Error:   "boom",
			}},
			wantJSONKeys: []string{`"error": "boom"`},
			wantMDSubstr: []string{"Error: `boom`"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := base
			r.Results = tc.results

			raw, err := report.JSON(r)
			if err != nil {
				t.Fatal(err)
			}
			body := string(raw)
			for _, key := range tc.wantJSONKeys {
				if !strings.Contains(body, key) {
					t.Fatalf("json missing %q in %s", key, body)
				}
			}
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}

			md := report.Markdown(r)
			for _, s := range tc.wantMDSubstr {
				if !strings.Contains(md, s) {
					t.Fatalf("markdown missing %q in %s", s, md)
				}
			}
			for _, s := range tc.wantNotMD {
				if strings.Contains(md, s) {
					t.Fatalf("markdown unexpectedly contains %q", s)
				}
			}
		})
	}
}
