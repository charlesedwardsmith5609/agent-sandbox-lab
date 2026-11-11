# agent-sandbox-lab

Small Go CLI that runs the same untrusted "tool call" workload under different
isolation levels and reports **startup latency**, **resource cost**, and which
**escape-style probes were blocked**.

> Comparative lab only. **No isolation level is claimed "safe."**
> Probes are harmless capability checks — not exploits, CVE PoCs, or escape tooling.

## Isolation backends

| Backend | Description | CI |
|---------|-------------|----|
| `process` | Child process with `RLIMIT_*` (seccomp-style resource limits; namespaces best-effort on Linux) | **Yes** |
| `container` | Rootless `docker`/`podman` (`--network=none`, read-only root, workdir bind) | Skips if unavailable |
| `gvisor` | Same as container with `--runtime=runsc` when `runsc` is installed | Skips if unavailable |

## Probe suite

| Probe | What it checks |
|-------|----------------|
| `read_host_file` | Read a host path outside the workdir |
| `write_outside_workdir` | Write a temp file outside the workdir |
| `open_network_socket` | Listen/dial on loopback |
| `fork_under_limit` | Spawn helpers until an nproc/cgroup ceiling intervenes |

## Quick start

```bash
git clone https://github.com/charlesedwardsmith5609/agent-sandbox-lab.git
cd agent-sandbox-lab
CGO_ENABLED=0 go build -o asl ./cmd/asl
./asl -backends process,container,gvisor -out out
```

Reports:

- `out/report.json`
- `out/report.md`

Process-only (matches CI):

```bash
./asl -backends process -out out
```

## Tests / CI

```bash
CGO_ENABLED=0 go test ./...
```

GitHub Actions (`.github/workflows/ci.yml`) runs **only the portable path**:
`go test ./...`, builds the CLI, and smokes `./asl -backends process`.

Container and gVisor backends are exercised locally when their runtimes exist;
otherwise they are recorded as skipped.

## Design & threat model

- [docs/design.md](docs/design.md) — architecture, runner interface, probe model
- [docs/threat-model.md](docs/threat-model.md) — what each boundary does / does not protect against, non-goals

## Example findings (illustrative)

On a typical developer laptop without container/gVisor tooling:

- `process` usually **allows** host file reads and local sockets; may **block** or
  constrain fork storms via `RLIMIT_NPROC`.
- `container` / `gvisor` appear as **skipped** until Docker/Podman and (for gVisor)
  `runsc` are installed and usable.
- When container isolation is available, host path reads often flip to **blocked**
  (path not mounted) and network probes fail under `--network=none`.

Treat every row as a lab observation, not a security certification.

## Inspiration (not copied)

Public discussion of agent isolation (Agent Substrate, Docker agent isolation,
gVisor/CNCF, Firecracker-related escape research, per-session credential
sandbox notes) shaped the questions this lab asks. Implementation is original
measurement code.

## License

Apache-2.0
