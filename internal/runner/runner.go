// Package runner defines the isolation backend contract used by the harness.
package runner

import (
	"context"
	"time"
)

// ProbeID identifies a capability check.
type ProbeID string

const (
	ProbeReadHostFile        ProbeID = "read_host_file"
	ProbeWriteOutsideWorkdir ProbeID = "write_outside_workdir"
	ProbeOpenNetworkSocket   ProbeID = "open_network_socket"
	ProbeForkUnderLimit      ProbeID = "fork_under_limit"
)

// AllProbes is the default suite order.
var AllProbes = []ProbeID{
	ProbeReadHostFile,
	ProbeWriteOutsideWorkdir,
	ProbeOpenNetworkSocket,
	ProbeForkUnderLimit,
}

// Outcome is the observed result of a probe.
type Outcome string

const (
	OutcomeBlocked Outcome = "blocked"
	OutcomeAllowed Outcome = "allowed"
	OutcomeError   Outcome = "error"
	OutcomeSkipped Outcome = "skipped"
)

// ProbeResult is one probe's observation inside a backend run.
type ProbeResult struct {
	ID      ProbeID `json:"id"`
	Outcome Outcome `json:"outcome"`
	Detail  string  `json:"detail,omitempty"`
}

// ResourceUsage is a best-effort cost snapshot for a backend run.
type ResourceUsage struct {
	PeakRSSBytes uint64        `json:"peak_rss_bytes,omitempty"`
	CPUTime      time.Duration `json:"cpu_time,omitempty"`
	MaxNProc     uint64        `json:"max_nproc,omitempty"`
	Notes        string        `json:"notes,omitempty"`
}

// Workload is the shared job handed to every backend.
type Workload struct {
	WorkDir     string
	Probes      []ProbeID
	HostFile    string // path used by read_host_file
	MaxNProc    uint64 // soft ceiling for fork probe / rlimit
	MaxMemoryMB uint64
	Timeout     time.Duration
}

// Result is the full observation for one backend.
type Result struct {
	Backend        string         `json:"backend"`
	Skipped        bool           `json:"skipped"`
	SkipReason     string         `json:"skip_reason,omitempty"`
	StartupLatency time.Duration  `json:"startup_latency"`
	Resources      ResourceUsage  `json:"resources"`
	Probes         []ProbeResult  `json:"probes"`
	Error          string         `json:"error,omitempty"`
}

// Backend isolates and executes a workload.
type Backend interface {
	Name() string
	Available(ctx context.Context) (bool, string)
	Run(ctx context.Context, w Workload) (Result, error)
}
