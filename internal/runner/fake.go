package runner

import (
	"context"
	"fmt"
	"time"
)

// Fake is a deterministic in-memory backend for unit tests.
type Fake struct {
	BackendName string
	Avail       bool
	Reason      string
	Latency     time.Duration
	Resources   ResourceUsage
	Outcomes    map[ProbeID]ProbeResult
	RunErr      error
}

func (f *Fake) Name() string {
	if f.BackendName == "" {
		return "fake"
	}
	return f.BackendName
}

func (f *Fake) Available(context.Context) (bool, string) {
	return f.Avail, f.Reason
}

func (f *Fake) Run(ctx context.Context, w Workload) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !f.Avail {
		return Result{
			Backend:    f.Name(),
			Skipped:    true,
			SkipReason: f.Reason,
		}, nil
	}
	if f.RunErr != nil {
		return Result{Backend: f.Name()}, f.RunErr
	}

	res := Result{
		Backend:        f.Name(),
		StartupLatency: f.Latency,
		Resources:      f.Resources,
	}
	for _, id := range w.Probes {
		if pr, ok := f.Outcomes[id]; ok {
			res.Probes = append(res.Probes, pr)
			continue
		}
		res.Probes = append(res.Probes, ProbeResult{
			ID:      id,
			Outcome: OutcomeSkipped,
			Detail:  fmt.Sprintf("fake has no scripted outcome for %s", id),
		})
	}
	return res, nil
}
