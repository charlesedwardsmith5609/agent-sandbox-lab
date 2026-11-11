// Package container implements a rootless container backend (docker/podman).
package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/probe"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

// Backend runs probes inside a rootless container when a runtime is available.
type Backend struct {
	Runtime string // "docker" or "podman"; empty = auto
	Image   string // default: alpine:3.20
}

func (b *Backend) Name() string { return "container" }

func (b *Backend) Available(ctx context.Context) (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "container backend supported on linux hosts only"
	}
	rt, err := b.resolveRuntime(ctx)
	if err != nil {
		return false, err.Error()
	}
	if err := exec.CommandContext(ctx, rt, "version").Run(); err != nil {
		return false, fmt.Sprintf("%s not usable: %v", rt, err)
	}
	return true, ""
}

func (b *Backend) resolveRuntime(ctx context.Context) (string, error) {
	if b.Runtime != "" {
		if _, err := exec.LookPath(b.Runtime); err != nil {
			return "", fmt.Errorf("runtime %s not found: %w", b.Runtime, err)
		}
		return b.Runtime, nil
	}
	for _, cand := range []string{"podman", "docker"} {
		if _, err := exec.LookPath(cand); err != nil {
			continue
		}
		if err := exec.CommandContext(ctx, cand, "version").Run(); err == nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no container runtime (podman/docker) available")
}

func (b *Backend) Run(ctx context.Context, w runner.Workload) (runner.Result, error) {
	ok, reason := b.Available(ctx)
	if !ok {
		return runner.Result{Backend: b.Name(), Skipped: true, SkipReason: reason}, nil
	}
	rt, err := b.resolveRuntime(ctx)
	if err != nil {
		return runner.Result{Backend: b.Name(), Skipped: true, SkipReason: err.Error()}, nil
	}
	image := b.Image
	if image == "" {
		image = "alpine:3.20"
	}
	if w.Timeout <= 0 {
		w.Timeout = 60 * time.Second
	}
	if w.MaxNProc == 0 {
		w.MaxNProc = 32
	}
	memMB := w.MaxMemoryMB
	if memMB == 0 {
		memMB = 256
	}
	probes := w.Probes
	if len(probes) == 0 {
		probes = runner.AllProbes
	}
	workDir, err := filepath.Abs(w.WorkDir)
	if err != nil {
		return runner.Result{Backend: b.Name()}, err
	}
	self, err := os.Executable()
	if err != nil {
		return runner.Result{Backend: b.Name()}, err
	}
	selfAbs, err := filepath.Abs(self)
	if err != nil {
		return runner.Result{Backend: b.Name()}, err
	}

	hostFile := w.HostFile
	if hostFile == "" {
		hostFile = "/etc/hosts"
	}

	args := []string{
		"run", "--rm",
		"--network=none",
		"--read-only",
		"--security-opt", "no-new-privileges",
		"--cap-drop", "ALL",
		"--tmpfs", "/tmp:rw,size=16m",
		"--pids-limit", fmt.Sprintf("%d", w.MaxNProc),
		"--memory", fmt.Sprintf("%dm", memMB),
		"-v", workDir + ":/work:rw",
		"-v", selfAbs + ":/asl:ro",
		"-w", "/work",
		"-e", probe.EnvWorkerMode + "=" + probe.WorkerModeFlag,
		"-e", probe.EnvWorkDir + "=/work",
		"-e", probe.EnvHostFile + "=" + hostFile,
		"-e", probe.EnvMaxNProc + "=" + fmt.Sprintf("%d", w.MaxNProc),
		"-e", probe.EnvProbeList + "=" + joinProbes(probes),
		image,
		"/asl", "worker-probes",
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, rt, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	startup := time.Since(start)

	res := runner.Result{
		Backend:        b.Name(),
		StartupLatency: startup,
		Resources: runner.ResourceUsage{
			MaxNProc: w.MaxNProc,
			Notes:    fmt.Sprintf("runtime=%s image=%s network=none read_only=true", rt, image),
		},
	}
	if err != nil && stdout.Len() == 0 {
		msg := strings.TrimSpace(stderr.String())
		if looksUnavailable(msg, err) {
			res.Skipped = true
			res.SkipReason = fmt.Sprintf("%s run failed: %v (%s)", rt, err, msg)
			return res, nil
		}
		res.Error = fmt.Sprintf("%v: %s", err, msg)
		return res, nil
	}
	var probeResults []runner.ProbeResult
	if decErr := json.Unmarshal(stdout.Bytes(), &probeResults); decErr != nil {
		res.Error = fmt.Sprintf("decode: %v; stderr=%s", decErr, stderr.String())
		return res, nil
	}
	res.Probes = probeResults
	return res, nil
}

func joinProbes(ids []runner.ProbeID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = string(id)
	}
	return strings.Join(parts, ",")
}

func looksUnavailable(msg string, err error) bool {
	low := strings.ToLower(msg + " " + err.Error())
	for _, s := range []string{
		"cannot connect", "permission denied", "no such file", "not found",
		"pull access", "rootless", "daemon", "executable file not found",
		"failed to resolve", "no match for platform",
	} {
		if strings.Contains(low, s) {
			return true
		}
	}
	return false
}
