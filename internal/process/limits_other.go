//go:build !unix

package process

import (
	"os/exec"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

func applySysProcAttr(cmd *exec.Cmd, w runner.Workload) {
	_ = cmd
	_ = w
}

// ApplyLimits is a no-op on platforms without rlimit.
func ApplyLimits(maxNProc, maxMemoryMB uint64) error {
	_, _ = maxNProc, maxMemoryMB
	return nil
}
