// Package gvisor implements a runsc-backed isolation backend when available.
package gvisor

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

// Backend uses docker/podman with the runsc runtime, or skips cleanly.
type Backend struct {
	Runtime  string // container CLI: docker/podman
	Image    string
	OCIRunsc string // path to runsc; empty = look up
}

func (b *Backend) Name() string { return "gvisor" }

func (b *Backend) Available(ctx context.Context) (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "gvisor backend supported on linux hosts only"
	}
	if _, err := exec.LookPath("runsc"); err != nil {
		return false, "runsc not found on PATH"
	}
	rt, err := b.resolveRuntime(ctx)
	if err != nil {
		return false, err.Error()
	}
	// Confirm the container engine knows about runsc, or that we can pass --runtime=runsc.
	cmd := exec.CommandContext(ctx, rt, "info")
	out, _ := cmd.CombinedOutput()
	info := string(out)
	if !strings.Contains(info, "runsc") && !runtimeConfigured(ctx, rt) {
		// Still attempt --runtime=runsc; many installs work without advertising.
		if err := exec.CommandContext(ctx, "runsc", "--version").Run(); err != nil {
			return false, "runsc present but not usable"
		}
	}
	return true, ""
}

func runtimeConfigured(ctx context.Context, rt string) bool {
	cmd := exec.CommandContext(ctx, rt, "info", "--format", "{{json .Runtimes}}")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "runsc")
}

func (b *Backend) resolveRuntime(ctx context.Context) (string, error) {
	if b.Runtime != "" {
		if _, err := exec.LookPath(b.Runtime); err != nil {
			return "", fmt.Errorf("runtime %s not found: %w", b.Runtime, err)
		}
		return b.Runtime, nil
	}
	for _, cand := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(cand); err != nil {
			continue
		}
		if err := exec.CommandContext(ctx, cand, "version").Run(); err == nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no container runtime available to launch runsc")
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
		"--runtime=runsc",
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
			Notes:    fmt.Sprintf("runtime=%s oci=runsc image=%s", rt, image),
		},
	}
	if err != nil && stdout.Len() == 0 {
		msg := strings.TrimSpace(stderr.String())
		res.Skipped = true
		res.SkipReason = fmt.Sprintf("runsc launch failed: %v (%s)", err, msg)
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
