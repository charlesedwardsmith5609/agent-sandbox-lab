// Package harness orchestrates backends and aggregates results.
package harness

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

// Report is the top-level lab output.
type Report struct {
	GeneratedAt time.Time       `json:"generated_at"`
	WorkDir     string          `json:"work_dir"`
	Disclaimer  string          `json:"disclaimer"`
	Results     []runner.Result `json:"results"`
}

const disclaimer = "Comparative lab output only. No isolation level is claimed safe. Probes are harmless capability checks, not exploits."

// Options configures a harness run.
type Options struct {
	Backends []runner.Backend
	Workload runner.Workload
}

// Run executes all backends against the shared workload.
func Run(ctx context.Context, opt Options) (Report, error) {
	if len(opt.Backends) == 0 {
		return Report{}, fmt.Errorf("no backends configured")
	}
	workDir := opt.Workload.WorkDir
	if workDir == "" {
		var err error
		workDir, err = os.MkdirTemp("", "asl-work-*")
		if err != nil {
			return Report{}, err
		}
		opt.Workload.WorkDir = workDir
	}
	if err := os.MkdirAll(opt.Workload.WorkDir, 0o755); err != nil {
		return Report{}, err
	}
	if len(opt.Workload.Probes) == 0 {
		opt.Workload.Probes = append([]runner.ProbeID(nil), runner.AllProbes...)
	}

	rep := Report{
		GeneratedAt: time.Now().UTC(),
		WorkDir:     opt.Workload.WorkDir,
		Disclaimer:  disclaimer,
	}

	for _, b := range opt.Backends {
		ok, reason := b.Available(ctx)
		if !ok {
			rep.Results = append(rep.Results, runner.Result{
				Backend:    b.Name(),
				Skipped:    true,
				SkipReason: reason,
			})
			continue
		}
		res, err := b.Run(ctx, opt.Workload)
		if err != nil {
			res.Backend = b.Name()
			res.Error = err.Error()
		}
		if res.Backend == "" {
			res.Backend = b.Name()
		}
		rep.Results = append(rep.Results, res)
	}
	return rep, nil
}
