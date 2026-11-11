//go:build unix

package process

import (
	"os"
	"runtime"
	"syscall"
)

func peakRSS(state *os.ProcessState) uint64 {
	if state == nil {
		return 0
	}
	ru, ok := state.SysUsage().(*syscall.Rusage)
	if !ok || ru == nil || ru.Maxrss < 0 {
		return 0
	}
	// Linux reports Maxrss in kilobytes; Darwin in bytes.
	if runtime.GOOS == "darwin" {
		return uint64(ru.Maxrss)
	}
	return uint64(ru.Maxrss) * 1024
}
