# sneaky-port

A client/server measurement tool under development for comparing FPS connection quality on your network.

It will compare real TCP, UDP, TLS, QUIC, WireGuard, Hysteria2, and DNS data paths against one Linux server, using Windows 11 clients. Both operating systems target 64-bit ARM64 and x86-64, with broad Linux server coverage including CentOS, Ubuntu, Fedora, Arch, and other major distributions; exact version limits remain open. Selected ports and independent packet priority markings in both directions are part of the comparison. Protocol sessions are handled by the tool itself.

Connection quality means timely delivery, low p95/p99 latency, low jitter, and few spikes or stalls under sustained game-like traffic. Preferential treatment of DNS is a hypothesis to measure, not an assumed advantage.

The protocol architecture must be extensible: later releases should add probe adapters through shared measurement, configuration, dashboard, and reporting contracts.

**Current status: finite authenticated UDP/TCP CLI runs, bounded local evidence and replay/export implemented; remote reconciliation, dashboard and full acceptance remain pending.** A future gaming proxy, traffic shaping, and automatic route switching are separate from this repository's scope.

Implementation was authorized on 2026-10-11. Current code supplies strict profiles/hashes, bounded scheduling/queues, local preflight, restricted pairing and key-only SSH readiness/leases. `client-run` connects shared UDP/TCP phase/accounting, reverse receipts, deadlines/cutoffs, RTT/variation/spike/stall analysis, physical-phase marking and bounded cancellation. Both endpoints save raw evidence; replay/export, authenticated server reconciliation and uncertainty-gated one-way estimates are available. Independently verified production clock bounds, API/dashboard and full acceptance remain pending. [Code flow](FLOWCHART.md), [contract](DESIGN.md#first-slice-implementation-contract), [TCP evidence](WORK_LOG.md#2026-10-11-tcp-integration).

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
   .\build\sneaky-port.exe client-init -data-dir .\client-data -client-id example-client | Set-Content -Encoding utf8 client-public.json
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

Keep local pairing exports and edited profiles out of Git. The root `client-public.json`, `server-public.json`, `readiness.yaml`, default data folders, `profiles-local/`, root report/export/capture folders, `.env` files and common private-key filenames are ignored by [.gitignore](.gitignore). Public keys are safe to transfer through the trusted setup channel, but pairing files and reports can reveal identities, addresses and ports. Review files before publishing, including exports saved under other names; ignore rules do not remove files from existing commits.

`server-export -data-dir ./server-data` reprints the public bundle. `server-disable -data-dir ./server-data -client-id example-client` revokes access, including authenticated connections on the next policy authorization poll. To set expiry or re-enable, explicitly edit the public enrollment's `enabled` / optional RFC3339 `expires_at` and run `server-enroll` again. Replacing imported server trust requires trusted re-import with `client-import ... -replace`. Updates serialize with an exclusive store lock and synced same-directory file replacement; a stale lock is left for explicit operator inspection, never stolen automatically. Power-loss durability and nonlocal filesystem behavior are unvalidated.

## Run a finite measurement

After pairing and readiness, review every workload, deadline, grace and resource value in the explicit profile, then run:

```powershell
.\build\sneaky-port.exe client-run -data-dir .\client-data -profile .\readiness.yaml
```

The server stays running between finite trials. Both endpoints process an authenticated frozen-plan start and use independent local monotonic anchors; they do not claim simultaneous starts. Warmup, phases and grace come from the profile. Each FIFO writer applies the current physical phase's directional DSCP before outgoing data or receipts; queued identities remain attributed to their original phase, and grace retains the last phase setting. A frame already being written cannot be remarked mid-frame. API application is separate from unobserved outer packets.

Ctrl+C, control/lease loss and endpoint failures stop generation and allow at most the configured grace/shutdown drain. Output includes a secret-free local snapshot with actual cutoffs, phase summaries, a server lifecycle status and `local_report_path`. Interrupted/failed runs exit nonzero and remain partial. After stop, a separate report channel on the same pinned SSH connection transfers server evidence. Output adds `server_evidence_transfer` and `reconciliation`; the `local` snapshot remains endpoint-only. Compatible complete records yield directional application delivery at each receiver’s own cutoff. Missing/partial/incompatible records keep delivery unknown; underlying packet loss stays unknown. `reconciliation.one_way` separately reports interval estimates or unavailable timing. A completed lifecycle does not prove timely delivery. Standalone `preflight` remains a separate local check.

Each endpoint saves `reports/<run-id>-client/` or `reports/<run-id>-server/` under its own data directory: `events.jsonl`, `manifest.json` and, after finalization, `summary.json`. Settings and an incomplete manifest are synced before measurement; raw records preserve host-local decimal nanoseconds and original identities. The final manifest governs storage completeness; a summary or final cutoff alone cannot prove successful publication. Existing run files are never overwritten or automatically deleted. The client stores transferred fixed files under `remote-server/`, a `server-transfer.json` receipt and a separately derived `reconciliation.json`; these never rewrite its endpoint-only manifest/raw summary.

Storage admission is nonblocking, with active-inclusive `queue_messages` and `event_bytes` queue caps. `file_bytes` covers raw envelopes, settings, summaries and simultaneous publication staging files; space is reserved before start, and infeasible budgets reject measurement. A queue/file/disk failure stops the local run as partial. `shutdown_timeout_ns` separately bounds the storage-worker join; a stalled filesystem can leave incomplete files until the OS call returns. File sync/atomic replacement is checked, but power-loss/nonlocal-filesystem durability remains unvalidated. [Persistence flow](FLOWCHART.md#local-evidence-persistence).

Transfer uses contiguous numbered 32 KiB chunks, SHA-256 checksums, acknowledged offsets and a final content hash. Server files share the explicit per-endpoint `file_bytes` ceiling (at most 1 GiB); report requests consume the remaining `control_requests` budget. `setup_timeout_ns` bounds the network transfer. Insufficient budget, interruption or bad content leaves the run partial and local evidence usable; no reconnect/resume or automatic deletion occurs. Both endpoints must run the current report-channel implementation. [Transfer flow and limits](FLOWCHART.md#authenticated-server-evidence-transfer-and-reconciliation).

Only one `client-run` can reserve a client data directory. Normal exit removes `active-run.lock`; after a crash, inspect it and verify no process still owns that run before removing the stale file. The server's own lease bounds cleanup when a client disappears. [Finite lifecycle evidence](WORK_LOG.md#2026-10-11-finite-run-lifecycle), [implemented flow](FLOWCHART.md#finite-run-lifecycle).

## Replay and export local reports

Use the run directory returned as `local_report_path`. Choose an explicit aggregate input-byte limit; these example limits are editable operator inputs, not a heap-memory cap:

```powershell
.\build\sneaky-port.exe report-replay -report .\client-data\reports\<run-id>-client -max-bytes 4194304
.\build\sneaky-port.exe report-export -report .\client-data\reports\<run-id>-client -max-bytes 4194304 -format csv
```

Both commands read local files and write to stdout, with no server connection or source-file changes. Replay validates schema/identities, frozen plan, raw checksum and declared cutoffs, then recomputes the saved summary through the shared ledger. A finalized `saved` summary mismatch rejects the report. Partial reports can be exported successfully; check `status`, `reason`, integrity and verification fields. Initial manifests and torn trailing writes remain partial, with discarded tail bytes recorded. Failed storage can expose a summary mismatch rather than inventing missing observations.

Choose `json` for settings and recomputed summary, `jsonl` for a typed `local_report` header plus raw endpoint envelopes, or `csv` for long-form field/unit/value/status rows. Exact integers, nulls and unknown reasons survive export. The explicit input cap (maximum 1 GiB) includes local files, the transfer receipt and both remote hash/replay passes. Replay rederives delivery and one-way intervals instead of trusting cached reconciliation JSON; checksums/provenance establish offline consistency, not transferable authentication. Missing remote evidence leaves delivery/timing unknown; packet loss remains unknown. [Replay flow](FLOWCHART.md#local-report-replay-and-export), [evidence](WORK_LOG.md#2026-10-11-local-report-replay-and-export).

`client-run` also returns `clock_sampling`; replay/export derives the same bounded window from raw `clock_capture` events. Explicit profile cadence, minimum recent valid samples, window and expiry control captures during warmup/main/grace. Four timestamps use the data epochs; serialization/send costs, invalid captures, skipped cadence slots and calibration application-byte counters remain visible. `samples_available` means enough recent raw constraints; `unavailable` means insufficient/expired evidence. Neither establishes calibrated one-way delay. Snapshot ages refer to the saved `as_of_ns`; replay does not advance a finished run's local clock. Readiness-only runs do not sample. [Sampling policy](DESIGN.md#calibration-and-one-way-evidence-policy), [code flow](FLOWCHART.md#live-ssh-clock-sampling).

`reconciliation.one_way` matches original attempts and complete receiver arrivals, then applies the clock revision available at client send time upstream or client arrival downstream. The report records raw-file hashes and retained results' identities, model/sample IDs, age, signed lower/upper/midpoint/uncertainty and clock-local units. Application serialization/submission/buffering are included. Production bounds are unverified, so values remain `estimate` or `unavailable`; source labels cannot enable `calibrated`. Later samples never upgrade old results. Detail bytes are capped at the smaller of the profile event budget and 256 KiB; exact status counts and omitted-model/result counts remain visible, with raw evidence retained. [One-way flow](FLOWCHART.md#uncertainty-gated-one-way-results).

## Project documents

- [AGENTS.md](AGENTS.md): working instructions and current authorization.
- [DESIGN.md](DESIGN.md): confirmed requirements, protocol/DNS coverage, ports, packet priority, measurement rules, and validation.
- [CHOICES.md](CHOICES.md): technical questions, options, recommendations, and confirmed decisions.
- [ROADMAP.md](ROADMAP.md): the first few actions, adjusted as we learn.
- [WORK_LOG.md](WORK_LOG.md): current implementation, recent changes, and validation.
- [ARCHIVE.md](ARCHIVE.md): older work history and choice discussions for targeted deeper review.
- [PROPOSALS.md](PROPOSALS.md): forward tracking of future features, optional ideas, and their decisions/status.
- [FLOWCHART.md](FLOWCHART.md): implemented function paths and remaining connections, linked to source.
