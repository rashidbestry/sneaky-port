# sneaky-port

A client/server measurement tool under development for comparing FPS connection quality on your network.

It will compare real TCP, UDP, TLS, QUIC, WireGuard, Hysteria2, and DNS data paths against one Linux server, using Windows 11 clients. Both operating systems target 64-bit ARM64 and x86-64, with broad Linux server coverage including CentOS, Ubuntu, Fedora, Arch, and other major distributions; exact version limits remain open. Selected ports and independent packet priority markings in both directions are part of the comparison. Protocol sessions are handled by the tool itself.

Connection quality means timely delivery, low p95/p99 latency, low jitter, and few spikes or stalls under sustained game-like traffic. Preferential treatment of DNS is a hypothesis to measure, not an assumed advantage.

The protocol architecture must be extensible: later releases should add probe adapters through shared measurement, configuration, dashboard, and reporting contracts.

**Current status: finite authenticated UDP/TCP CLI runs implemented; durable reports, dashboard and full acceptance remain pending.** A future gaming proxy, traffic shaping, and automatic route switching are separate from this repository's scope.

Implementation was authorized on 2026-10-11. Current code includes strict profiles/hashes, bounded scheduling/queues, local preflight, restricted pairing, key-only SSH readiness/leases and shared UDP/TCP warmup/main/burst/quiet accounting with reverse receipts, deadlines/cutoffs and RTT/variation/spike/stall analysis. TCP includes complete-frame arrivals, prefix/partial-byte evidence and stream stalls; finite start, writer-owned physical-phase marking, grace and bounded cancellation now connect that accounting to `client-run`. Live calibration, durable reports/API/dashboard and full acceptance remain pending. [Code flow](FLOWCHART.md), [contract](DESIGN.md#first-slice-implementation-contract), [TCP evidence](WORK_LOG.md#2026-10-11-tcp-integration).

## Run the foundation tools

Requires Go 1.27 or later. Commands below run only the currently implemented operations:

```text
go test ./...
go run ./cmd/sneaky-port adapters
go run ./cmd/sneaky-port validate -profile YOUR_PROFILE.yaml
go run ./cmd/sneaky-port preflight -duration-ns YOUR_DURATION -interval-ns YOUR_INTERVAL
go run ./cmd/sneaky-port keygen -output NEW_PRIVATE_KEY_FILE
```

All workload/timeout/budget settings are explicit; there is no automatically selected FPS profile. Preflight is local timer evidence. Key generation writes a new restricted file and prints only public enrollment material. Dependencies and [license notices](THIRD_PARTY_NOTICES.md) are pinned; native ARM64/live-network support remains unvalidated.

## Pair endpoints and check readiness

`client-ready` verifies the exact plan, owned sockets and authentication, holds an optional lease, then stops. Success prints `"status": "readiness_checked"` and `"measurement_started": false`, actual endpoint bindings, marking evidence and client application-control byte counts. It sends no scheduled workload and produces no latency/loss report.

1. **Build on Windows PowerShell**, from this checkout. These commands target an x86-64 Linux server; use `arm64` for an ARM64 server.

   ```powershell
   go build -o .\build\sneaky-port.exe ./cmd/sneaky-port
   $env:GOOS = "linux"
   $env:GOARCH = "amd64"
   $env:CGO_ENABLED = "0"
   go build -o .\build\sneaky-port-linux-amd64 ./cmd/sneaky-port
   Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
   .\build\sneaky-port.exe client-init -data-dir .\client-data -client-id rashid | Set-Content -Encoding utf8 client-public.json
   ```

2. **Transfer through a trusted channel**: copy the Linux binary, [readiness-policy.json](examples/readiness-policy.json) and `client-public.json` into your working directory on the owned Linux server. Review the policy's explicit bindings, ports, timeouts and resource ceilings; its numbers are examples. The server must be reachable on TCP 22220 for SSH and on the selected data port/protocol.

3. **Initialize and start on Linux**, replacing `YOUR_SERVER_IPV4` with the client-reachable IPv4 address:

   ```sh
   chmod +x ./sneaky-port-linux-amd64
   ./sneaky-port-linux-amd64 server-init -data-dir ./server-data -server-id lab-server -advertise YOUR_SERVER_IPV4:22220 -policy ./readiness-policy.json > server-public.json
   ./sneaky-port-linux-amd64 server-enroll -data-dir ./server-data -enrollment ./client-public.json
   ./sneaky-port-linux-amd64 server -data-dir ./server-data
   ```

   The final command remains in the foreground; Ctrl+C stops the owned endpoint. Existing system SSH settings are not used. Transfer only `server-public.json` back to Windows through your trusted channel.

4. **Import and check on Windows**, with `server-public.json` in this checkout:

   ```powershell
   .\build\sneaky-port.exe client-import -data-dir .\client-data -pairing-id lab-server -bundle .\server-public.json
   Copy-Item .\examples\readiness-udp.yaml .\readiness.yaml
   notepad .\readiness.yaml
   ```

   Set `server_address` to the same reachable server IPv4 address. Save, then run:

   ```powershell
   .\build\sneaky-port.exe validate -profile .\readiness.yaml
   .\build\sneaky-port.exe client-ready -data-dir .\client-data -profile .\readiness.yaml -hold-ns 1000000000
   ```

   For TCP, change `adapter_id: tcp` and `adapter_options: {tcp_no_delay: true}`, then rerun validation/readiness. All other requested inputs stay explicit. A hold of `0` stops immediately after readiness; a positive hold sends heartbeats within the profile's control-request budget. Failed readiness exits nonzero.

Initialization creates each private key locally and refuses existing identities/key destinations. Add `-key EXISTING_PRIVATE_KEY` to `client-init` or `server-init` to reference an existing compatible key unchanged; encrypted keys prompt in an interactive terminal without echo. Public imports accept UTF-8 with an optional BOM. The app directories are restricted; keep private keys on their originating endpoints.

`server-export -data-dir ./server-data` reprints the public bundle. `server-disable -data-dir ./server-data -client-id rashid` revokes access, including authenticated connections on the next policy authorization poll. To set expiry or re-enable, explicitly edit the public enrollment's `enabled` / optional RFC3339 `expires_at` and run `server-enroll` again. Replacing imported server trust requires trusted re-import with `client-import ... -replace`. Updates serialize with an exclusive store lock and synced same-directory file replacement; a stale lock is left for explicit operator inspection, never stolen automatically. Power-loss durability and nonlocal filesystem behavior are unvalidated.

## Run a finite measurement

After pairing and readiness, review every workload, deadline, grace and resource value in the explicit profile, then run:

```powershell
.\build\sneaky-port.exe client-run -data-dir .\client-data -profile .\readiness.yaml
```

The server stays running between finite trials. Both endpoints process an authenticated frozen-plan start and use independent local monotonic anchors; they do not claim simultaneous starts. Warmup, phases and grace come from the profile. Each FIFO writer applies the current physical phase's directional DSCP before outgoing data or receipts; queued identities remain attributed to their original phase, and grace retains the last phase setting. A frame already being written cannot be remarked mid-frame. API application is separate from unobserved outer packets.

Ctrl+C, control/lease loss and endpoint failures stop generation and allow at most the configured grace/shutdown drain. Output is a transient, secret-free local snapshot with actual cutoffs, phase summaries and a server lifecycle status. Interrupted/failed runs exit nonzero and remain partial. Remote delivery, underlying TCP packet loss and one-way delay remain unknown; a completed lifecycle does not prove successful timely delivery. Durable files, replay/export and remote reconciliation are tasks 12-14. Standalone `preflight` remains a separate local check.

Only one `client-run` can reserve a client data directory. Normal exit removes `active-run.lock`; after a crash, inspect it and verify no process still owns that run before removing the stale file. The server's own lease bounds cleanup when a client disappears. [Finite lifecycle evidence](WORK_LOG.md#2026-10-11-finite-run-lifecycle), [implemented flow](FLOWCHART.md#finite-run-lifecycle).

## Project documents

- [AGENTS.md](AGENTS.md): working instructions and current authorization.
- [DESIGN.md](DESIGN.md): confirmed requirements, protocol/DNS coverage, ports, packet priority, measurement rules, and validation.
- [CHOICES.md](CHOICES.md): technical questions, options, recommendations, and confirmed decisions.
- [ROADMAP.md](ROADMAP.md): the first few actions, adjusted as we learn.
- [WORK_LOG.md](WORK_LOG.md): current implementation, recent changes, and validation.
- [ARCHIVE.md](ARCHIVE.md): older work history and choice discussions for targeted deeper review.
- [PROPOSALS.md](PROPOSALS.md): forward tracking of future features, optional ideas, and their decisions/status.
- [FLOWCHART.md](FLOWCHART.md): implemented function paths and remaining connections, linked to source.
