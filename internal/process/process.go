// Package process implements the portable process isolation backend.
package process

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/probe"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

// Backend runs the workload in a child process with resource limits.
type Backend struct {
	// Self is the path to this binary; empty means os.Executable().
	Self string
}

func (b *Backend) Name() string { return "process" }

func (b *Backend) Available(context.Context) (bool, string) {
	return true, ""
}

func (b *Backend) Run(ctx context.Context, w runner.Workload) (runner.Result, error) {
	self := b.Self
	if self == "" {
		var err error
		self, err = os.Executable()
		if err != nil {
			return runner.Result{Backend: b.Name()}, err
		}
	}
	if w.Timeout <= 0 {
		w.Timeout = 30 * time.Second
	}
	if w.MaxNProc == 0 {
		w.MaxNProc = 32
	}
	if w.MaxMemoryMB == 0 {
		w.MaxMemoryMB = 256
	}
	if w.HostFile == "" {
		w.HostFile = "/etc/hosts"
	}
	probes := w.Probes
	if len(probes) == 0 {
		probes = runner.AllProbes
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, self, "worker-probes")
	cmd.Dir = w.WorkDir
	cmd.Env = append(os.Environ(),
		probe.EnvWorkerMode+"="+probe.WorkerModeFlag,
		probe.EnvWorkDir+"="+w.WorkDir,
		probe.EnvHostFile+"="+w.HostFile,
		probe.EnvMaxNProc+"="+strconv.FormatUint(w.MaxNProc, 10),
		probe.EnvProbeList+"="+joinProbes(probes),
	)
	applySysProcAttr(cmd, w)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Start()
	startup := time.Since(start)
	if err != nil {
		return runner.Result{
			Backend:        b.Name(),
			StartupLatency: startup,
			Error:          err.Error(),
		}, err
	}

	waitErr := cmd.Wait()
	res := runner.Result{
		Backend:        b.Name(),
		StartupLatency: startup,
		Resources: runner.ResourceUsage{
			MaxNProc: w.MaxNProc,
			Notes:    platformNotes(),
		},
	}
	if cmd.ProcessState != nil {
		res.Resources.CPUTime = cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()
		res.Resources.PeakRSSBytes = peakRSS(cmd.ProcessState)
	}

	if waitErr != nil && stdout.Len() == 0 {
		res.Error = fmt.Sprintf("worker failed: %v; stderr=%s", waitErr, stderr.String())
		return res, nil
	}

	var probeResults []runner.ProbeResult
	if err := json.Unmarshal(stdout.Bytes(), &probeResults); err != nil {
		res.Error = fmt.Sprintf("decode worker output: %v; stderr=%s; stdout=%s", err, stderr.String(), stdout.String())
		return res, nil
	}
	res.Probes = probeResults
	return res, nil
}

func joinProbes(ids []runner.ProbeID) string {
	if len(ids) == 0 {
		return ""
	}
	s := string(ids[0])
	for i := 1; i < len(ids); i++ {
		s += "," + string(ids[i])
	}
	return s
}

func platformNotes() string {
	return fmt.Sprintf("os=%s arch=%s rlimit_applied=true", runtime.GOOS, runtime.GOARCH)
}
