// Command asl compares isolation backends for an untrusted tool-call workload.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/container"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/gvisor"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/harness"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/probe"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/process"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/report"
	"github.com/charlesedwardsmith5609/agent-sandbox-lab/internal/runner"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "worker-probes":
			os.Exit(runWorkerProbes())
		case "worker-sleep":
			os.Exit(runWorkerSleep(os.Args[2:]))
		}
	}
	os.Exit(runCLI(os.Args[1:]))
}

func runCLI(args []string) int {
	fs := flag.NewFlagSet("asl", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outDir := fs.String("out", "out", "directory for report.json and report.md")
	backends := fs.String("backends", "process,container,gvisor", "comma-separated backends")
	workdir := fs.String("workdir", "", "workload directory (default: temp dir)")
	hostFile := fs.String("host-file", "/etc/hosts", "host path for read_host_file probe")
	maxNProc := fs.Uint64("max-nproc", 32, "process limit hint")
	maxMem := fs.Uint64("max-memory-mb", 256, "memory limit hint in MiB")
	timeout := fs.Duration("timeout", 45*time.Second, "per-backend timeout")
	format := fs.String("format", "both", "json|markdown|both")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	selected := parseBackends(*backends)
	var backs []runner.Backend
	for _, name := range selected {
		switch name {
		case "process":
			backs = append(backs, &process.Backend{})
		case "container":
			backs = append(backs, &container.Backend{})
		case "gvisor":
			backs = append(backs, &gvisor.Backend{})
		default:
			fmt.Fprintf(os.Stderr, "unknown backend %q\n", name)
			return 2
		}
	}

	wd := *workdir
	if wd == "" {
		tmp, err := os.MkdirTemp("", "asl-work-*")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		wd = tmp
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout*time.Duration(len(backs)+1))
	defer cancel()

	rep, err := harness.Run(ctx, harness.Options{
		Backends: backs,
		Workload: runner.Workload{
			WorkDir:     wd,
			Probes:      runner.AllProbes,
			HostFile:    *hostFile,
			MaxNProc:    *maxNProc,
			MaxMemoryMB: *maxMem,
			Timeout:     *timeout,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	jsonBytes, md, err := report.WriteBoth(rep)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	switch strings.ToLower(*format) {
	case "json":
		if err := os.WriteFile(filepath.Join(*outDir, "report.json"), jsonBytes, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(string(jsonBytes))
	case "markdown", "md":
		if err := os.WriteFile(filepath.Join(*outDir, "report.md"), []byte(md), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Print(md)
	default:
		if err := os.WriteFile(filepath.Join(*outDir, "report.json"), jsonBytes, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := os.WriteFile(filepath.Join(*outDir, "report.md"), []byte(md), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("Wrote %s and %s\n", filepath.Join(*outDir, "report.json"), filepath.Join(*outDir, "report.md"))
	}
	return 0
}

func parseBackends(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func runWorkerProbes() int {
	maxNProc, _ := strconv.ParseUint(os.Getenv(probe.EnvMaxNProc), 10, 64)
	_ = process.ApplyLimits(maxNProc, 256)
	results := probe.RunAll(
		probe.ParseProbeList(os.Getenv(probe.EnvProbeList)),
		os.Getenv(probe.EnvWorkDir),
		os.Getenv(probe.EnvHostFile),
		maxNProc,
	)
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(results); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runWorkerSleep(args []string) int {
	d := 50 * time.Millisecond
	if len(args) > 0 {
		if parsed, err := time.ParseDuration(args[0]); err == nil {
			d = parsed
		}
	}
	time.Sleep(d)
	return 0
}
