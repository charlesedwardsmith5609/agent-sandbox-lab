# Threat model and non-goals

## Purpose

`agent-sandbox-lab` measures how different isolation boundaries respond to a
fixed set of **harmless capability probes**. It exists to make trade-offs
visible (latency, resource cost, what gets blocked), not to certify sandboxes.

## Assets under consideration

| Asset | Why it matters |
|-------|----------------|
| Host filesystem paths outside the workdir | Secrets, configs, other tenants' data |
| Host network stack | Data exfiltration, lateral movement |
| Host process table / CPU / memory | Denial of service via fork/memory bombs |
| Credentials mounted into agent environments | Session tokens, cloud keys |

## Adversary sketch (lab scope)

We model an untrusted "tool call" that can execute local code inside the
backend's worker. The adversary wants to:

1. Read host files outside the assigned workdir.
2. Write outside the workdir (persistence / tampering).
3. Open network sockets (exfiltration).
4. Exhaust process slots under a declared limit.

We do **not** model kernel exploits, speculative-execution attacks, side
channels, malicious images, or compromised runtimes.

## What each boundary does / does not protect against

### Process (`process`)

**Does (best effort):**

- Apply `RLIMIT_NPROC`, `RLIMIT_NOFILE`, `RLIMIT_AS`, `RLIMIT_FSIZE` to the child.
- Optionally enter user / mount / network / PID namespaces when the kernel and
  permissions allow (Linux).
- Constrain the working directory for probe evaluation.

**Does not:**

- Provide a hard filesystem jail on all platforms.
- Block all syscalls (no full custom seccomp profile in v1).
- Isolate from the host kernel; a process escape is a kernel compromise.
- Claim defense against privileged helpers or ambient credentials in the parent env.

### Rootless container (`container`)

**Does (when runtime available):**

- Separate mount namespace with only the workdir bind-mounted.
- Drop many capabilities; prefer read-only root filesystem.
- Disable networking (`--network=none`) for the probe suite.
- Limit PIDs via container runtime flags / cgroups when supported.

**Does not:**

- Protect against container runtime or kernel vulnerabilities.
- Guarantee rootless mode on every host (may skip).
- Isolate a compromised container engine / dockerd.
- Make image supply-chain safe.

### gVisor (`gvisor`)

**Does (when `runsc` available):**

- Interpose guest syscalls through the Sentry user-space kernel.
- Narrow the host attack surface relative to a bare process or thin container.

**Does not:**

- Eliminate all escape risk; gVisor bugs and host misconfiguration remain.
- Match every container feature (compatibility gaps may force skips).
- Replace a threat model that includes malicious host admins.

## Probe ethics

Probes must be:

- **Harmless** — no privilege escalation payloads, no CVE reproduction.
- **Reversible** — temporary files cleaned up; no persistent host changes intended.
- **Explicit** — outcomes labeled `blocked` / `allowed` / `error` / `skipped`.

## Explicit non-goals

- Not a vulnerability exploitation toolkit.
- Not a replacement for production sandbox policy review.
- Not a guarantee that "blocked" means "impossible under all conditions."
- Not a claim that higher isolation is always better for every agent workload.

## Residual risk statement

Even if every probe reports `blocked`, residual risk remains: kernel bugs,
runtime bugs, misconfiguration, side channels, and future probe classes not
covered here. Treat reports as comparative lab output only.
