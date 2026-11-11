// Package report renders harness output as JSON and Markdown.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/harness"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

// JSON returns indented JSON for a report.
func JSON(r harness.Report) ([]byte, error) {
	type wireProbe struct {
		ID      runner.ProbeID  `json:"id"`
		Outcome runner.Outcome  `json:"outcome"`
		Detail  string          `json:"detail,omitempty"`
	}
	type wireResources struct {
		PeakRSSBytes uint64 `json:"peak_rss_bytes,omitempty"`
		CPUTimeNS    int64  `json:"cpu_time_ns,omitempty"`
		MaxNProc     uint64 `json:"max_nproc,omitempty"`
		Notes        string `json:"notes,omitempty"`
	}
	type wireResult struct {
		Backend           string         `json:"backend"`
		Skipped           bool           `json:"skipped"`
		SkipReason        string         `json:"skip_reason,omitempty"`
		StartupLatencyNS  int64          `json:"startup_latency_ns"`
		StartupLatencyHuman string       `json:"startup_latency"`
		Resources         wireResources  `json:"resources"`
		Probes            []wireProbe    `json:"probes"`
		Error             string         `json:"error,omitempty"`
	}
	type wireReport struct {
		GeneratedAt string       `json:"generated_at"`
		WorkDir     string       `json:"work_dir"`
		Disclaimer  string       `json:"disclaimer"`
		Results     []wireResult `json:"results"`
	}

	out := wireReport{
		GeneratedAt: r.GeneratedAt.Format(time.RFC3339Nano),
		WorkDir:     r.WorkDir,
		Disclaimer:  r.Disclaimer,
	}
	for _, res := range r.Results {
		wr := wireResult{
			Backend:             res.Backend,
			Skipped:             res.Skipped,
			SkipReason:          res.SkipReason,
			StartupLatencyNS:    res.StartupLatency.Nanoseconds(),
			StartupLatencyHuman: res.StartupLatency.String(),
			Resources: wireResources{
				PeakRSSBytes: res.Resources.PeakRSSBytes,
				CPUTimeNS:    res.Resources.CPUTime.Nanoseconds(),
				MaxNProc:     res.Resources.MaxNProc,
				Notes:        res.Resources.Notes,
			},
			Error: res.Error,
		}
		for _, p := range res.Probes {
			wr.Probes = append(wr.Probes, wireProbe(p))
		}
		out.Results = append(out.Results, wr)
	}
	return json.MarshalIndent(out, "", "  ")
}

// Markdown returns a human-readable report.
func Markdown(r harness.Report) string {
	var b strings.Builder
	b.WriteString("# agent-sandbox-lab report\n\n")
	b.WriteString(fmt.Sprintf("Generated: `%s`\n\n", r.GeneratedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("Workdir: `%s`\n\n", r.WorkDir))
	b.WriteString("> " + r.Disclaimer + "\n\n")

	b.WriteString("## Summary\n\n")
	b.WriteString("| Backend | Status | Startup | Peak RSS | Blocked probes |\n")
	b.WriteString("|---------|--------|---------|----------|----------------|\n")
	for _, res := range r.Results {
		status := "ok"
		if res.Skipped {
			status = "skipped"
		} else if res.Error != "" {
			status = "error"
		}
		rss := "—"
		if res.Resources.PeakRSSBytes > 0 {
			rss = fmt.Sprintf("%d KiB", res.Resources.PeakRSSBytes/1024)
		}
		startup := "—"
		if !res.Skipped {
			startup = res.StartupLatency.String()
		}
		blocked := countOutcome(res.Probes, runner.OutcomeBlocked)
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %d |\n",
			res.Backend, status, startup, rss, blocked))
	}
	b.WriteString("\n")

	for _, res := range r.Results {
		b.WriteString(fmt.Sprintf("## Backend: `%s`\n\n", res.Backend))
		if res.Skipped {
			b.WriteString(fmt.Sprintf("Skipped: %s\n\n", res.SkipReason))
			continue
		}
		if res.Error != "" {
			b.WriteString(fmt.Sprintf("Error: `%s`\n\n", res.Error))
		}
		b.WriteString(fmt.Sprintf("- Startup latency: `%s`\n", res.StartupLatency))
		if res.Resources.CPUTime > 0 {
			b.WriteString(fmt.Sprintf("- CPU time: `%s`\n", res.Resources.CPUTime))
		}
		if res.Resources.PeakRSSBytes > 0 {
			b.WriteString(fmt.Sprintf("- Peak RSS: `%d` bytes\n", res.Resources.PeakRSSBytes))
		}
		if res.Resources.MaxNProc > 0 {
			b.WriteString(fmt.Sprintf("- Max nproc hint: `%d`\n", res.Resources.MaxNProc))
		}
		if res.Resources.Notes != "" {
			b.WriteString(fmt.Sprintf("- Notes: %s\n", res.Resources.Notes))
		}
		b.WriteString("\n| Probe | Outcome | Detail |\n|-------|---------|--------|\n")
		for _, p := range res.Probes {
			detail := strings.ReplaceAll(p.Detail, "|", "\\|")
			b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", p.ID, p.Outcome, detail))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Findings notes\n\n")
	b.WriteString("- `blocked` means the probe operation failed inside the backend — useful for comparison, not a security proof.\n")
	b.WriteString("- `allowed` means the operation succeeded; expected for weaker boundaries on some probes.\n")
	b.WriteString("- Skipped backends were unavailable on this host and are not failures.\n")
	return b.String()
}

func countOutcome(probes []runner.ProbeResult, want runner.Outcome) int {
	n := 0
	for _, p := range probes {
		if p.Outcome == want {
			n++
		}
	}
	return n
}

// WriteBoth returns JSON and Markdown bytes.
func WriteBoth(r harness.Report) (jsonBytes []byte, md string, err error) {
	jsonBytes, err = JSON(r)
	if err != nil {
		return nil, "", err
	}
	md = Markdown(r)
	if len(bytes.TrimSpace(jsonBytes)) == 0 {
		return nil, "", fmt.Errorf("empty json report")
	}
	return jsonBytes, md, nil
}
