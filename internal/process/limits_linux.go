//go:build linux

package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

func applySysProcAttr(cmd *exec.Cmd, w runner.Workload) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	_ = w
}

// ApplyLimits sets portable resource ceilings inside the worker process.
//
// RLIMIT_NPROC on Linux is user-wide (threads for the real UID). We raise a
// ceiling of (current user threads + maxNProc) so the fork probe can hit a
// limit without immediately failing against the host's existing process table.
// RLIMIT_AS is avoided: Go's virtual address space is large and a tight AS cap
// aborts the runtime before probes run.
func ApplyLimits(maxNProc, maxMemoryMB uint64) error {
	if maxNProc == 0 {
		maxNProc = 32
	}
	_ = maxMemoryMB

	// Ceiling = existing user threads + maxNProc. The fork probe asks for more
	// than maxNProc helpers so it should observe EAGAIN.
	nproc := uint64(userThreadCount()) + maxNProc
	if nproc < 16 {
		nproc = 16
	}
	_ = unix.Setrlimit(unix.RLIMIT_NPROC, &unix.Rlimit{Cur: nproc, Max: nproc})
	_ = unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{Cur: 256, Max: 256})
	_ = unix.Setrlimit(unix.RLIMIT_FSIZE, &unix.Rlimit{Cur: 32 * 1024 * 1024, Max: 32 * 1024 * 1024})
	_ = unix.Setrlimit(unix.RLIMIT_CPU, &unix.Rlimit{Cur: 60, Max: 60})
	return nil
}

func userThreadCount() int {
	uid := os.Getuid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return currentThreadCount()
	}
	total := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid := e.Name()
		if pid[0] < '0' || pid[0] > '9' {
			continue
		}
		status, err := os.ReadFile(filepath.Join("/proc", pid, "status"))
		if err != nil {
			continue
		}
		if procUID(string(status)) != uid {
			continue
		}
		total += threadsFromStatus(string(status))
	}
	if total <= 0 {
		return currentThreadCount()
	}
	return total
}

func procUID(status string) int {
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				n, _ := strconv.Atoi(fields[1])
				return n
			}
		}
	}
	return -1
}

func threadsFromStatus(status string) int {
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "Threads:") {
			n, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Threads:")))
			if n > 0 {
				return n
			}
		}
	}
	return 1
}

func currentThreadCount() int {
	matches, err := filepath.Glob(filepath.Join("/proc/self/task", "*"))
	if err == nil && len(matches) > 0 {
		return len(matches)
	}
	return 16
}
