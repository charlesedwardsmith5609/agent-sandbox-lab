// Package probe implements harmless capability checks for sandbox comparison.
//
// These are not exploits: each probe attempts a narrowly scoped operation and
// reports whether the environment allowed or blocked it.
package probe

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

// Env knobs passed into worker subprocesses.
const (
	EnvProbeList   = "ASL_PROBES"
	EnvWorkDir     = "ASL_WORKDIR"
	EnvHostFile    = "ASL_HOST_FILE"
	EnvMaxNProc    = "ASL_MAX_NPROC"
	EnvWorkerMode  = "ASL_WORKER"
	WorkerModeFlag = "1"
)

// RunAll executes the requested probes and returns structured results.
func RunAll(probes []runner.ProbeID, workDir, hostFile string, maxNProc uint64) []runner.ProbeResult {
	out := make([]runner.ProbeResult, 0, len(probes))
	for _, id := range probes {
		out = append(out, runOne(id, workDir, hostFile, maxNProc))
	}
	return out
}

func runOne(id runner.ProbeID, workDir, hostFile string, maxNProc uint64) runner.ProbeResult {
	switch id {
	case runner.ProbeReadHostFile:
		return readHostFile(hostFile)
	case runner.ProbeWriteOutsideWorkdir:
		return writeOutside(workDir)
	case runner.ProbeOpenNetworkSocket:
		return openNetwork()
	case runner.ProbeForkUnderLimit:
		return forkUnderLimit(maxNProc)
	default:
		return runner.ProbeResult{ID: id, Outcome: runner.OutcomeSkipped, Detail: "unknown probe"}
	}
}

func readHostFile(hostFile string) runner.ProbeResult {
	if hostFile == "" {
		hostFile = "/etc/hosts"
	}
	data, err := os.ReadFile(hostFile)
	if err != nil {
		return runner.ProbeResult{
			ID:      runner.ProbeReadHostFile,
			Outcome: runner.OutcomeBlocked,
			Detail:  err.Error(),
		}
	}
	return runner.ProbeResult{
		ID:      runner.ProbeReadHostFile,
		Outcome: runner.OutcomeAllowed,
		Detail:  fmt.Sprintf("read %d bytes from %s", len(data), hostFile),
	}
}

func writeOutside(workDir string) runner.ProbeResult {
	absWork, err := filepath.Abs(workDir)
	if err != nil {
		return runner.ProbeResult{ID: runner.ProbeWriteOutsideWorkdir, Outcome: runner.OutcomeError, Detail: err.Error()}
	}
	// Intentionally outside workdir: sibling path under /tmp.
	target := filepath.Join(os.TempDir(), "asl-write-outside-probe-"+strconv.Itoa(os.Getpid()))
	if strings.HasPrefix(target, absWork+string(os.PathSeparator)) || target == absWork {
		target = filepath.Join("/var/tmp", "asl-write-outside-probe-"+strconv.Itoa(os.Getpid()))
	}
	err = os.WriteFile(target, []byte("asl-probe"), 0o600)
	if err != nil {
		return runner.ProbeResult{
			ID:      runner.ProbeWriteOutsideWorkdir,
			Outcome: runner.OutcomeBlocked,
			Detail:  err.Error(),
		}
	}
	_ = os.Remove(target)
	return runner.ProbeResult{
		ID:      runner.ProbeWriteOutsideWorkdir,
		Outcome: runner.OutcomeAllowed,
		Detail:  "wrote " + target,
	}
}

func openNetwork() runner.ProbeResult {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		// Also try dial as a second signal; still blocked if both fail.
		conn, dialErr := net.DialTimeout("tcp", "127.0.0.1:1", 200*time.Millisecond)
		if dialErr != nil {
			return runner.ProbeResult{
				ID:      runner.ProbeOpenNetworkSocket,
				Outcome: runner.OutcomeBlocked,
				Detail:  fmt.Sprintf("listen: %v; dial: %v", err, dialErr),
			}
		}
		_ = conn.Close()
		return runner.ProbeResult{
			ID:      runner.ProbeOpenNetworkSocket,
			Outcome: runner.OutcomeAllowed,
			Detail:  "dial succeeded after listen failure: " + err.Error(),
		}
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return runner.ProbeResult{
		ID:      runner.ProbeOpenNetworkSocket,
		Outcome: runner.OutcomeAllowed,
		Detail:  "listened on " + addr,
	}
}

func forkUnderLimit(maxNProc uint64) runner.ProbeResult {
	if maxNProc == 0 {
		maxNProc = 32
	}
	// Spawn short-lived helper processes via the same binary until we hit a limit
	// or a soft ceiling. This is a resource-limit check, not a fork bomb.
	limit := int(maxNProc) + 8
	if limit > 128 {
		limit = 128
	}
	started := 0
	blocked := false
	var lastErr error
	helper, helperArgs := forkHelper()
	children := make([]*exec.Cmd, 0, limit)
	for i := 0; i < limit; i++ {
		cmd := exec.Command(helper, helperArgs...)
		if err := cmd.Start(); err != nil {
			blocked = true
			lastErr = err
			break
		}
		children = append(children, cmd)
		started++
	}
	// Give children a moment, then reap.
	runtime.Gosched()
	time.Sleep(20 * time.Millisecond)
	for _, cmd := range children {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}

	if blocked {
		return runner.ProbeResult{
			ID:      runner.ProbeForkUnderLimit,
			Outcome: runner.OutcomeBlocked,
			Detail:  fmt.Sprintf("started %d then blocked: %v (max_nproc hint %d)", started, lastErr, maxNProc),
		}
	}
	if uint64(started) > maxNProc {
		// Limit did not constrain us — still an observation, not an exploit.
		return runner.ProbeResult{
			ID:      runner.ProbeForkUnderLimit,
			Outcome: runner.OutcomeAllowed,
			Detail:  fmt.Sprintf("started %d processes without hitting limit hint %d", started, maxNProc),
		}
	}
	return runner.ProbeResult{
		ID:      runner.ProbeForkUnderLimit,
		Outcome: runner.OutcomeAllowed,
		Detail:  fmt.Sprintf("started %d processes (under hint %d)", started, maxNProc),
	}
}

func forkHelper() (string, []string) {
	// Prefer a tiny external binary so this stays a process-slot check, not a memory bomb.
	for _, cand := range []string{"/bin/sleep", "/usr/bin/sleep"} {
		if _, err := os.Stat(cand); err == nil {
			return cand, []string{"1"}
		}
	}
	self, err := os.Executable()
	if err != nil {
		return os.Args[0], []string{"worker-sleep", "200ms"}
	}
	return self, []string{"worker-sleep", "200ms"}
}

// ParseProbeList parses a comma-separated probe id list.
func ParseProbeList(s string) []runner.ProbeID {
	if strings.TrimSpace(s) == "" {
		return append([]runner.ProbeID(nil), runner.AllProbes...)
	}
	parts := strings.Split(s, ",")
	out := make([]runner.ProbeID, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, runner.ProbeID(p))
	}
	return out
}
