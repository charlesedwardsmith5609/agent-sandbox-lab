//go:build unix && !linux

package process

import (
	"os/exec"
	"syscall"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

func applySysProcAttr(cmd *exec.Cmd, w runner.Workload) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_ = w
}

// ApplyLimits sets portable resource ceilings inside the worker process.
func ApplyLimits(maxNProc, maxMemoryMB uint64) error {
	if maxNProc == 0 {
		maxNProc = 32
	}
	if maxMemoryMB == 0 {
		maxMemoryMB = 256
	}
	_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Cur: 64, Max: 64})
	_ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 8 * 1024 * 1024, Max: 8 * 1024 * 1024})
	// RLIMIT_NPROC exists on BSD/Darwin.
	_ = syscall.Setrlimit(syscall.RLIMIT_NPROC, &syscall.Rlimit{Cur: maxNProc, Max: maxNProc})
	_ = maxMemoryMB
	return nil
}
