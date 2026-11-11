//go:build !unix

package process

import "os"

func peakRSS(*os.ProcessState) uint64 { return 0 }
