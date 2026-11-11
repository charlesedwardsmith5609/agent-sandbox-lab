# Design: agent-sandbox-lab

Small Go CLI that runs the same untrusted "tool call" workload under different
isolation levels and reports startup latency, resource cost, and which
escape-style probes were blocked.

**This is a measurement and education tool, not a security product.** No backend
is claimed to be "safe." Probes are harmless capability checks, not escape
exploits.

## Goals

- Compare isolation backends with a shared runner interface.
- Emit machine-readable (JSON) and human-readable (Markdown) reports.
- Skip backends cleanly when their runtime is unavailable.
- Keep CI portable: only the process backend runs in GitHub Actions.

## Non-goals

- No real sandbox escape code, CVE reproduction, or exploit PoCs.
- No claim that any isolation level is production-safe or complete.
- No multi-tenant orchestration, policy engine, or agent framework.
- No Firecracker / VM backends in v1 (inspiration only).
- No copying proprietary isolation code from other products.

## Architecture

```
CLI (cmd/asl)
  └── harness.Run(backends, probes, workdir)
        ├── runner.Backend.Available()
        ├── runner.Backend.Run(ctx, Workload) → Result
        └── report.{JSON,Markdown}
```

### Runner interface

Each backend implements `runner.Backend`:

| Method | Purpose |
|--------|---------|
| `Name()` | Stable identifier (`process`, `container`, `gvisor`) |
| `Available(ctx)` | Probe host for runtime support; return skip reason |
| `Run(ctx, Workload)` | Start isolated worker, execute probes, collect metrics |

`Workload` carries workdir, probe list, and resource budget hints.
`Result` carries startup latency, peak RSS / CPU time where available,
per-probe outcomes (`blocked` / `allowed` / `error` / `skipped`), and notes.

### Probe model

Probes are **capability assertions**, not attacks. Each probe:

1. Attempts a narrowly scoped, reversible operation.
2. Records whether the boundary prevented it (`blocked`) or permitted it (`allowed`).
3. Never embeds shellcode, kernel exploits, or privilege-escalation payloads.

| Probe ID | Check | Typical expectation |
|----------|-------|---------------------|
| `read_host_file` | Read a known host path outside the workdir | Process: often allowed; container/gVisor: often blocked if not mounted |
| `write_outside_workdir` | Create a file outside the workdir | Stronger backends should block |
| `open_network_socket` | Dial or listen on loopback | Process: usually allowed; network-namespaced backends may block |
| `fork_under_limit` | Spawn processes until limited | Should hit nproc/cgroup ceiling without hanging the host |

### Backends

1. **process** (portable / CI default) — child process with `RLIMIT_*` and optional
   Linux user-namespace / mount / network unshare when permitted. "Seccomp-style"
   here means syscall-adjacent resource and namespace limits feasible from pure Go
   without a custom seccomp BPF compiler in CI.
2. **container** — rootless `docker` or `podman` run with a minimal image, read-only
   root where possible, no network (`--network=none`), bind-mount of workdir only.
3. **gvisor** — `runsc` (or docker runtime `runsc`) when present; otherwise skipped.

Unavailable backends return `Available() = false` with a reason; the harness
records them as skipped, not failed.

## Threat model

See [threat-model.md](./threat-model.md).

## Inspiration (not copied)

Public discussion around agent isolation (Agent Substrate, Docker agent
isolation, gVisor/CNCF, Firecracker-related escape research, per-session
credential sandboxing) informs the *questions* this lab asks. Implementation
is original measurement code only.

## Testing strategy

- Unit tests with a `fake` backend covering harness aggregation.
- Table tests for JSON/Markdown report generation.
- Integration-ish tests for the process backend (always).
- Container / gVisor tests gated on `Available()` (skipped in CI).
