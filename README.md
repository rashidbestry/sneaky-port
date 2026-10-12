# sneaky-port

A client/server measurement tool under development for comparing FPS connection quality on your network.

It will compare real TCP, UDP, TLS, QUIC, WireGuard, Hysteria2, and DNS data paths against one Linux server, using Windows 11 clients. Both operating systems target 64-bit ARM64 and x86-64, with broad Linux server coverage including CentOS, Ubuntu, Fedora, Arch, and other major distributions; exact version limits remain open. Selected ports and independent packet priority markings in both directions are part of the comparison. Protocol sessions are handled by the tool itself.

Connection quality means timely delivery, low p95/p99 latency, low jitter, and few spikes or stalls under sustained game-like traffic. Preferential treatment of DNS is a hypothesis to measure, not an assumed advantage.

The protocol architecture must be extensible: later releases should add probe adapters through shared measurement, configuration, dashboard, and reporting contracts.

**Current status: finite authenticated UDP/TCP runs, evidence/replay/export, reconciliation, one-way estimates, protected local API, dashboard diagnostics and matched saved-run comparisons implemented within recorded scopes. Live counters, verified clock bounds, rendered-browser QA and full acceptance remain pending.** A future gaming proxy, traffic shaping, and automatic route switching are separate from this repository's scope.

Implementation was authorized on 2026-10-11. `client-run` connects shared UDP/TCP phases, reverse receipts, deadline/cutoff accounting, RTT/variation/spike/stall analysis, physical-phase marking and bounded cancellation. Both endpoints save raw evidence; replay/export, authenticated reconciliation and uncertainty-gated one-way estimates are available. `client-api` protects local run controls, saved profiles, public pairing/key operations and report replay/import/export. Saved dashboard diagnostics and matched comparisons are implemented; independently verified production clock bounds, live counters, rendered-browser QA and full acceptance remain pending. [Code flow](FLOWCHART.md), [contract](DESIGN.md#first-slice-implementation-contract), [roadmap](ROADMAP.md#first-slice-task-index).

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

`file_bytes` must leave room for reserved summaries, manifests and clock evidence as well as raw records. Increasing `ledger_messages` increases that reservation even in a short run. The example uses 1,024 ledger entries within its 8 MiB file ceiling; older copies with 10,000 entries must reduce that cap or increase the file budget and matching server ceiling. Readiness-only checks do not exercise evidence storage.

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

## Protected local run API

### Dashboard setup and run controls

After launching `client-api` below, open its exact `api_url` in your browser and paste the startup `session_token` into **Local session → Connect**. The embedded shell has no credentials/settings; all setup reads and actions use the protected API. The token stays in page memory, never in a URL, cookie or browser storage. Disconnect forgets it and stops polling; an admitted run continues until completion, explicit cancellation or API shutdown.

1. Review the public server bundle/address/fingerprint from your trusted transfer, then import under a pairing ID. Existing trust requires the explicit replacement checkbox. Select its reference and review the independently editable data address/ports. Generate a managed client key only if needed; transfer public enrollment JSON to the server operator using the existing [pairing workflow](#pair-endpoints-and-check-readiness).
2. Edit the launch or loaded saved profile, including registered adapter/options, ports, ordered phases, independent directional payload/rates/bursts/DSCP, timing, limits and calibration inputs. Durations/64-bit quantities remain exact decimal strings. Phase deadline left blank inherits the run deadline. Duplicate/move/remove phases explicitly; the editor supports up to 64 phases and rejects larger profiles visibly (the CLI retains its existing limits). Complete JSON is strictly validated before application; loading a profile or applying JSON explicitly replaces editor contents.
3. **Validate** leaves disk unchanged. **Save profile explicitly** uses the loaded revision; a changed profile ID creates only. Stale saves conflict and keep edits visible; load the saved version to resolve them. **Start selected revision** requires clean saved settings. Frozen settings for that start remain visible while later edits/saves affect future runs.
4. **Read status** or **Cancel this run** uses the displayed control handle. Keep that ID to recover status after reload. Polls coalesce to at most one per second while executing/cancelling; errors stop polling visibly. Lost start/cancel acknowledgments retain the exact command ID/target for explicit retry in the same API process. Reload loses page-local retry state; a restarted API has new credentials/history. Unknown/readiness/completeness states stay explicit; live phase/counters remain unavailable; saved diagnostics and matched comparison views are implemented below.

Adapter options come from registry descriptions and require explicit JSON values (for example a boolean); selecting another carrier keeps existing options visible for correction rather than silently rewriting them. Requested markings are not captured evidence. Richer fragmentation/header controls remain open under T25. [Dashboard source flow](FLOWCHART.md#dashboard-setup-and-run-controls).

Launch with an explicit application-owned client store and profile:

```powershell
.\build\sneaky-port.exe client-api -data-dir .\client-data -profile .\examples\readiness-udp.yaml
```

The example profile needs your explicit server/workload settings. Startup JSON returns `api_url`, a fresh `session_token`, `profile_id` and `profile_revision` (canonical selected plan hash). A saved profile with that ID takes precedence; otherwise the launch file is a transient fallback. Startup does not save profiles or generate keys. Keep the token local. Each request needs `Authorization: Bearer <session_token>` and a safe `X-Request-ID` (1-64 letters, digits, `_` or `-`). Browser requests must use the exact returned origin; no CORS access or token-in-URL bootstrap is provided. `-listen` accepts a literal loopback IP only, defaulting to `127.0.0.1:0`; `[::1]:0` supports IPv6. Ctrl+C cancels and joins owned work within `-shutdown-timeout` (default 10s), reporting expiration explicitly.

| Route | Request / result |
| --- | --- |
| `GET /api/v1/setup` | Selected normalized launch/saved profile and revision, persisted flag and optional public client enrollment; no key references |
| `GET /api/v1/adapters?cursor=0&limit=16` | Registry descriptions, at most 16 items; `next_cursor` is empty at the end |
| `POST /api/v1/runs` | JSON `{ "schema_version": 1, "request_id": "start-1", "profile_id": "<selected ID>", "profile_revision": "<selected hash>" }`; freezes that validated revision and returns `control_run_id` |
| `GET /api/v1/runs/<control_run_id>` | Bounded lifecycle/identity/completeness status |
| `POST /api/v1/runs/<control_run_id>/cancel` | JSON `{ "schema_version": 1, "request_id": "cancel-1" }`; cancels that handle only |
| `GET /api/v1/profiles`, `/pairings`, `/reports` | `cursor`/`limit` pages (maximum 16); profile/report items are saved IDs, pairing items include public bundles |
| `POST /api/v1/profiles/validate` | Body adds `profile` (complete normalized object) and `profile_revision: ""`; returns normalized profile/hash without saving |
| `GET /api/v1/profiles/<id>`, `PUT /api/v1/profiles/<id>` | Read saved profile/hash; PUT uses the validation body with the current hash, or `""` for create only. Stale edits return 409 |
| `POST /api/v1/pairings/import`, `GET /api/v1/pairings/<id>` | Import body adds `pairing_id`, public `bundle` and explicit `replace` boolean; existing trust requires `replace: true` |
| `POST /api/v1/client-keys`, `GET /api/v1/client-keys/<id>/public` | Create body adds `client_id`; generates the single managed client identity only when absent. Read exports enrollment public key only, for trusted server enrollment |
| `GET /api/v1/reports/<id>`, `GET /api/v1/reports/<id>/export?format=json\|jsonl\|csv` | Recompute saved evidence; export an attachment in one explicitly selected format |
| `POST /api/v1/reports/import` | Body adds `report_id` and `files` mapping fixed filenames to base64 bytes; validates/replays before publishing a new report; existing reports cannot be replaced |
| `POST /api/v1/reports/compare` | Explicit A/B `pairs`, predeclared `axes`, `endpoint` and originating `scope`; [workflow and limits](#compare-matched-saved-runs) |

Body/header request IDs must agree. Repeating an accepted mutation ID returns its original acknowledgment; reusing it for another operation conflicts. One run executes at a time, independent of HTTP connection/request cancellation. `executing` covers setup/data/finalization; `cancelling` lasts until cleanup returns. Measurement ID/readiness and local completeness become known at finalization; unknown booleans remain `null`, and no live phase/counters are fabricated. Failure reasons direct you to local evidence without exposing file/key paths. Existing replay/export commands remain available.

Start/cancel bodies are capped at 4 KiB; saved-operation bodies at 1 MiB including base64 overhead. Every mutation body carries `schema_version: 1` and matching `request_id`. The process retains at most 32 run statuses and 128 accepted start/cancel commands without evicting retries. Saved catalogs inspect at most 256 directory entries; report replay charges at most 8 MiB of file reads (including remote verification/replay), and serialized export is capped at 16 MiB. One saved operation executes at a time; busy requests return 503. Larger reports remain inspectable through the CLI with its explicit byte budget. These are engineering limits, not workload recommendations.

Report imports accept `manifest.json`, `events.jsonl`, optional referenced `summary.json` and `server-transfer.json`, and the complete transferred `remote-server/{manifest.json,events.jsonl,summary.json}` group. Import uses existing checksum/schema/accounting validators; cached reconciliation is excluded and recomputed. Offline integrity does not authenticate imported evidence. Imports never connect to a server, enroll keys remotely or resume runs. Explicit profile saves affect future starts; active plans and retry acknowledgments stay frozen. Validation/saves also reject unsupported registered-adapter options. Saved-operation request IDs provide correlation; automatic retry deduplication applies to start/cancel only. Saved diagnostic views are implemented; live counters remain unavailable and matched saved comparison is implemented below. [API flow](FLOWCHART.md#protected-local-run-api), [saved operations](FLOWCHART.md#saved-configuration-and-report-api).

### Diagnostic and distribution views

In **Inspect saved evidence**, refresh the report list and load an explicit local or transferred-server origin. Select `main`, `warmup`, or a zero-based phase index; the phase table supplies indices and exact half-open boundaries. The displayed report ID, endpoint, scope and partial/completeness/integrity state belong to the last successful load. Missing transferred evidence rejects visibly. Endpoint-only views retain receiver delivery/packet loss as unavailable; reconciled exports remain separate.

Counts/deadline misses precede tails. Elapsed RTT and scheduled-origin completion have separate native SVG time/ECDF plots. Quantiles use every eligible completed raw sample, including late receipts strictly before the observation cutoff. Missing/local groups remain marked; quiet phases invent no losses. Each plot uses at most 128 groups retaining exact extrema/counts and inclusive ECDF ranks; coordinates are approximate and hover text retains exact ns. Time groups never cross workload phases. Scalar/phase/wake tables paginate twenty rows; reports exceeding 64 phases reject instead of truncating. Replay uses the existing 8 MiB aggregate read budget and single saved-operation admission. No automatic replay during traffic or fabricated live progress.

**Optional local timer preflight** takes explicit positive decimal-string duration/interval ns: at most one second and 4,096 scheduled slots. It reports local wake lag/missed slots and exact nearest-rank tails without network probes or OS changes. Timer lag does not establish network delay or clock calibration; clock source/resolution remain unverified. Requests stop on disconnect/shutdown; interruption is explicit. Start/cancel retry deduplication does not apply to preflight.

Protected routes: `GET /api/v1/reports/<id>/diagnostics?endpoint=local|server&scope=main|warmup|<index>` and `POST /api/v1/preflight` with `schema_version`, `request_id`, `duration_ns`, `interval_ns`. The existing token/Host/Origin/peer guards cover both. [Source flow](FLOWCHART.md#diagnostic-and-distribution-views), [evidence](WORK_LOG.md#2026-10-12-diagnostic-and-distribution-views).

## Project documents

- [AGENTS.md](AGENTS.md): working instructions and current authorization.
- [DESIGN.md](DESIGN.md): confirmed requirements, protocol/DNS coverage, ports, packet priority, measurement rules, and validation.
- [CHOICES.md](CHOICES.md): technical questions, options, recommendations, and confirmed decisions.
- [ROADMAP.md](ROADMAP.md): the first few actions, adjusted as we learn.
- [WORK_LOG.md](WORK_LOG.md): current implementation, recent changes, and validation.
- [ARCHIVE.md](ARCHIVE.md): older work history and choice discussions for targeted deeper review.
- [PROPOSALS.md](PROPOSALS.md): forward tracking of future features, optional ideas, and their decisions/status.
- [FLOWCHART.md](FLOWCHART.md): implemented function paths and remaining connections, linked to source.

## Compare matched saved runs

After finalization, use **Compare saved trials** with distinct A/B report IDs from the saved catalog, one pair per line (maximum eight pairs). Select local or transferred-server origin and `main`, `warmup` or a zero-based phase index. Select only predeclared axes: `adapter`, `port`, `client_dscp`, `server_dscp`; every frozen profile's `comparison_axes` must equal the selection (order does not matter). No axes means identical configurations. The port axis permits destination/source port changes; the adapter axis permits contract/options changes only when the adapter actually differs. Other workload, phase/content/rate/deadline/warmup/grace, resource, calibration, address/pairing, analysis and preflight settings must match exactly. Profile labels may differ; each repeated A or B configuration must remain fixed.

Before the runs, provide matching `matched_metadata`, including a nonblank `environment` description of the owned server, client/path, external load and other relevant conditions. These are operator declarations. Selection order and `created_utc` storage labels do not verify alternating/sequential execution, uncontended traffic or environment. Different retained endpoint/peer platform, build or clock provenance rejects matching where available; absent remote provenance remains a limit. Actual offered traffic, setup, framing/outer overhead, source/NAT bindings and requested/applied/observed markings remain in settings/raw exports for review. A selected axis never proves priority effects or wire equivalence.

The protected API uses the normal schema/request envelope, for example:

```json
{"schema_version":1,"request_id":"compare-1","pairs":[{"a":"trial-a1","b":"trial-b1"},{"a":"trial-a2","b":"trial-b2"}],"axes":["port"],"endpoint":"local","scope":"main"}
```

Every pair retains settings/provenance, completeness, exact populations and local faults. Tables show deadline misses and exact `misses/planned slots` fractions first, then completed RTT p95/p99 and variation tails/counts, reconciled application delivery where available, skips/writes and shared spike/stall episodes. Deltas are exact signed **B minus A**, including rational fractions such as `1/3`; missing values remain unavailable. Zero planned slots has no fraction. Main originating-phase rows remain separate even when inspecting warmup or another scope. Quiet phases invent no samples; completed tails never hide unresolved traffic. Existing strict cutoff and origin-phase rules apply.

Repeated views show the minimum/maximum of the individual trial deltas, never pooled p95/p99. At least two complete compatible pairs are required for a repeated description; absent trials, partial evidence and opposite-sign deltas remain inconclusive. Exact equal available values are ties. There is no material-difference threshold, significance test, confidence score, winner or combined score. One-way comparison stays unavailable while production clock bounds are unverified; packet-loss claims and ISP attribution are not derived. Offline consistency never authenticates evidence.

Admission rejects duplicate report IDs or duplicate measurement run IDs (imported copies are not repetitions), invalid/tampered evidence and excessive work. One serialized saved operation shares an **8 MiB aggregate file-read budget** across all selected runs and transferred-file validation passes, with at most 16 distinct runs, 64 phases per run and a 16 MiB serialized result. These are engineering caps, not heap or performance guarantees. Cancellation is checked between bounded replays. Nothing writes reports, starts traffic or changes settings. Choose a pair to inspect its full provenance; tables render twenty rows per page. Failed requests retain the labeled last successful selection; disconnect clears it. Rendered-browser QA and full network acceptance remain pending. [Source flow](FLOWCHART.md#matched-saved-run-comparisons).
