# Operator guide

Detailed commands and measurement/reporting contracts for the current UDP/TCP implementation. See [README](../README.md) for the quick start and [ROADMAP](ROADMAP.md#task-35-current-acceptance-review) for acceptance status.

## GitHub Actions

[CI](../.github/workflows/ci.yml) runs on branch pushes, pull requests and manual dispatch: Windows Server 2025/Ubuntu 24.04 x86-64 owned-source tests, native race checks, vet, formatting, the Node dashboard harness and packaged-helper checks. Hosted checks are separate from Windows 11 deployment, rendered-browser QA and the remaining native/live acceptance gates.

[Repository attributes](../.gitattributes) keep Go source as LF on both platforms so Windows checkout conversion does not create false `gofmt` failures.

[Release preview](../.github/workflows/release.yml) runs on a new `v*` version tag or manual dispatch with an existing tag. It validates the exact tagged revision, requires CI to pass, builds/verifies both archives and publishes a prerelease with GitHub's built-in token. All releases remain prereleases while full acceptance is incomplete. Commit and push these workflow files before creating a new tag; the tagged commit must include the packaging script. Existing releases are not overwritten.

The shared [packager](../.github/scripts/package-release.ps1) also runs locally: `./.github/scripts/package-release.ps1 -Platform windows -Version v0.1.0-alpha.2` (use `linux` on Linux). It writes only public binaries, quickstart/build metadata and licenses to `build/release-assets/`, verifies extraction and refuses existing staging/output. [packaging source](../.github/scripts/package-release.ps1).

## Adapter extension gate

Adapters register in code before serving. `Store.Adapters` selects one startup registry for saved configuration, authenticated client/server coordination, finite workers and replay/import/comparison; nil retains the production UDP/TCP registry. Standalone workers use `Config.Registry`, and offline `ReplayReport` accepts the matching registry. Unknown IDs/versions/options and unsupported advertised families/platforms/frame sizes reject explicitly. This is compile-time integration, with no runtime plugin loader.

Descriptors now expose `framing_bytes`: the fixed transport bytes beyond each authenticated message (UDP 0, framed TCP 4). `Read` returns a complete message with framing removed; `Write` returns bytes including framing. Common workers use those limits/costs while the factory owns real framing/socket behavior. These fixed costs are not captured outer-packet sizes; variable overhead and later-carrier mechanics still need their own contracts. SSH discovery now requires this descriptor field: older descriptor shapes reject during contract negotiation before traffic. Use matching updated client/server builds; saved reports and probe framing remain compatible.

The TCP-backed `test-extension` fixture has a distinct authenticated identity, `enabled` option and 1,024-byte limit. It is imported only by tests and is absent from production commands. Its protected HTTP/SSH lifecycle, both-direction accounting, cancellation, saved replay/import/three-format exports/comparison and actual dashboard script pass the shared workflow. Directional delivery requires compatible identities within independent endpoint cutoffs; completed endpoints can still retain unknown delivery. This gate adds no production carrier or native/live support claim. [source](../internal/adapter/adapter.go), [latest accounting checks](WORK_LOG.md#2026-10-14-ci-localapi-accounting-fixture-repair).

## Run the foundation tools

Requires Go 1.27 or later. Commands below run only the currently implemented operations:

```text
go test ./...
go run ./cmd/sneaky-port adapters
go run ./cmd/sneaky-port validate -profile YOUR_PROFILE.yaml
go run ./cmd/sneaky-port preflight -duration-ns YOUR_DURATION -interval-ns YOUR_INTERVAL
go run ./cmd/sneaky-port keygen -output NEW_PRIVATE_KEY_FILE
```

Preflight is local timer evidence. Key generation writes a new restricted file and prints only public enrollment material. Dependencies and [license notices](THIRD_PARTY_NOTICES.md) are pinned; supported Linux versions and live-network acceptance require their own evidence.

## Default configuration

Windows startup requires no arguments: run or double-click the dashboard executable. It creates/reuses `%APPDATA%\sneaky-port\client` for private keys, saved settings and reports, independently of the current folder. Optional flags work directly, for example `sneaky-port-dashboard.exe -data-dir .\client-data` to reuse an existing store. Explicit `client-api` remains available on both platforms; its default store on Linux is `${XDG_CONFIG_HOME:-$HOME/.config}/sneaky-port/client`. Existing stores are not migrated automatically. Linux no-command startup retains the standalone server role.

`client-api`, `validate`, `client-ready` and `client-run` accept an optional `-profile FILE`. Without it, the binary supplies an editable `default-udp` plan: 10 seconds, UDP/IPv4, 60 packets/s and 32-byte payloads per direction, port 32000, DSCP 0, a 100-ms receipt deadline and 100-ms grace. Dashboard SSH setup selects the real server; direct CLI defaults use `127.0.0.1` and pairing `lab-server`. Use an explicit profile for a different CLI target/workload. Specialized CLI identity/readiness/run commands retain their explicit data-directory and applicable hold inputs.

No command automatically reads local `readiness.yaml`. Opening the dashboard or validating settings sends no measurement traffic; `client-run` explicitly starts a run after authentication. A saved dashboard profile with the selected ID still takes precedence; otherwise settings remain transient until explicitly saved. Supplied files must remain complete and valid: unreadable/malformed overrides reject. The frozen plan and reports contain every resolved value. No sample configuration folder is shipped; create an optional override from the resolved `validate` plan as shown below. Starter values are editable examples; production clock bounds remain unverified. [Compiled settings](../internal/profile/defaults.go).

## Start the standalone Linux server

Run the Linux server binary with no arguments:

```sh
chmod +x ./sneaky-port-server-linux-amd64
./sneaky-port-server-linux-amd64
```

It starts the app's embedded SSH listener on **TCP 22022**, using built-in service/resource limits. No server policy file or test profile is required. First start generates private identity and authorization state under `${XDG_CONFIG_HOME:-$HOME/.config}/sneaky-port/server`; restarts preserve existing identity, clients and bindings. Authorized clients send their complete test settings before data sockets are prepared. First-time key pairing remains the authentication step below.

Ubuntu's normal SSH service on port 22 is independent. The dashboard's password-based **SSH Connect** continues to target that normal Linux SSH service and manages its disposable helper; direct standalone app connections use the existing keyed CLI/pairing flow on 22022. For a fresh standalone state, optional `server -listen IP:PORT -advertise CLIENT_REACHABLE_IP:PORT` handles explicit bindings/public addresses; automatic discovery uses a local address of the selected family.

Implementation/build and Windows checks are recorded in [server startup evidence](WORK_LOG.md#2026-10-15-built-in-server-defaults-and-port-22022). Linux runtime validation for this change is deferred at the user's request.

## Pair endpoints and check readiness

`client-ready` verifies the exact plan, owned sockets and authentication, holds an optional lease, then stops. Success prints `"status": "readiness_checked"` and `"measurement_started": false`, actual endpoint bindings, marking evidence and client application-control byte counts. It sends no scheduled workload and produces no latency/loss report.

1. **Build on Windows PowerShell**, from this checkout, for the Windows 11 x86-64 client and Linux x86-64 server.

   ```powershell
   $env:GOOS = "windows"
   $env:GOARCH = "amd64"
   $env:GOAMD64 = "v1"
   $env:CGO_ENABLED = "0"
   go build -o .\build\sneaky-port.exe ./cmd/sneaky-port
   $env:GOOS = "linux"
   go build -o .\build\sneaky-port-linux-amd64 ./cmd/sneaky-port
   Remove-Item Env:GOOS, Env:GOARCH, Env:GOAMD64, Env:CGO_ENABLED
   .\build\sneaky-port.exe client-init -data-dir .\client-data -client-id example-client | Set-Content -Encoding utf8 client-public.json
   ```

2. **Transfer through a trusted channel**: copy the Linux binary and `client-public.json` into your working directory on the owned Linux server. The app must be reachable on TCP 22022 and on the client-selected data port/protocol. A server policy JSON is unnecessary.

3. **Start on Linux** in one terminal:

   ```sh
   chmod +x ./sneaky-port-linux-amd64
   ./sneaky-port-linux-amd64
   ```

   This remains in the foreground; Ctrl+C stops the owned endpoint. In a second terminal, authorize the client's public key and export the server's public identity:

   ```sh
   server_state="${XDG_CONFIG_HOME:-$HOME/.config}/sneaky-port/server"
   ./sneaky-port-linux-amd64 server-enroll -data-dir "$server_state" -enrollment ./client-public.json
   ./sneaky-port-linux-amd64 server-export -data-dir "$server_state" > server-public.json
   ```

   Transfer only `server-public.json` back to Windows through your trusted channel. Existing explicit `server-init` and `-data-dir` workflows remain supported; `server-init -policy` optionally accepts a complete legacy override matching the [server policy schema](../internal/control/store.go). An invalid explicit override rejects instead of falling back to defaults.

4. **Import and check on Windows**, with `server-public.json` in this checkout:

   ```powershell
   .\build\sneaky-port.exe client-import -data-dir .\client-data -pairing-id lab-server -bundle .\server-public.json
   New-Item -ItemType Directory -Path .\profiles-local -Force | Out-Null
   $plan = (.\build\sneaky-port.exe validate | ConvertFrom-Json).plan
   $plan.server_address = "YOUR_SERVER_IPV4"
   $plan | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 .\profiles-local\run.json
   notepad .\profiles-local\run.json
   ```

   Set `server_address` to the same reachable server IPv4 address. Save, then run:

   ```powershell
   .\build\sneaky-port.exe validate -profile .\profiles-local\run.json
   .\build\sneaky-port.exe client-ready -data-dir .\client-data -profile .\profiles-local\run.json -hold-ns 1000000000
   ```

   For TCP, change `"adapter_id": "tcp"` and `"adapter_options": {"tcp_no_delay": true}`, then rerun validation/readiness. All other requested inputs stay explicit. A hold of `0` stops immediately after readiness; a positive hold sends heartbeats within the profile's control-request budget. Failed readiness exits nonzero.

Initialization creates each private key locally and refuses existing identities/key destinations. Add `-key EXISTING_PRIVATE_KEY` to `client-init` or `server-init` to reference an existing compatible key unchanged; encrypted keys prompt in an interactive terminal without echo. Public imports accept UTF-8 with an optional BOM. The app directories are restricted; keep private keys on their originating endpoints.

Keep local pairing exports and edited profiles out of Git. The root `client-public.json`, `server-public.json`, `readiness.yaml`, default data folders, `profiles-local/`, root report/export/capture folders, `.env` files and common private-key filenames are ignored by [.gitignore](../.gitignore). Public keys are safe to transfer through the trusted setup channel, but pairing files and reports can reveal identities, addresses and ports. Review files before publishing, including exports saved under other names; ignore rules do not remove files from existing commits.

`server-export -data-dir ./server-data` reprints the public bundle. `server-disable -data-dir ./server-data -client-id example-client` revokes access, including authenticated connections on the next policy authorization poll. To set expiry or re-enable, explicitly edit the public enrollment's `enabled` / optional RFC3339 `expires_at` and run `server-enroll` again. Replacing imported server trust requires trusted re-import with `client-import ... -replace`. Updates serialize with an exclusive store lock and synced same-directory file replacement; a stale lock is left for explicit operator inspection, never stolen automatically. Power-loss durability and nonlocal filesystem behavior are unvalidated.

## Run a finite measurement

After pairing and readiness, review every workload, deadline, grace and resource value in the explicit profile, then run:

```powershell
.\build\sneaky-port.exe client-run -data-dir .\client-data -profile .\profiles-local\run.json
```

`file_bytes` must leave room for reserved summaries, manifests and clock evidence as well as raw records. Increasing `ledger_messages` increases that reservation even in a short run. The built-in plan uses 4,096 ledger entries and a 64 MiB file ceiling; smaller explicit budgets must retain sufficient reservation space. Readiness-only checks do not exercise evidence storage.

The server stays running between finite trials. Both endpoints process an authenticated frozen-plan start and use independent local monotonic anchors; they do not claim simultaneous starts. Warmup, phases and grace come from the profile. Each FIFO writer applies the current physical phase's directional DSCP before outgoing data or receipts; queued identities remain attributed to their original phase, and grace retains the last phase setting. A frame already being written cannot be remarked mid-frame. API application is separate from unobserved outer packets.

Ctrl+C, control/lease loss and endpoint failures stop generation and allow at most the configured grace/shutdown drain. Output includes a secret-free local snapshot with actual cutoffs, phase summaries, a server lifecycle status and `local_report_path`. Interrupted/failed runs exit nonzero and remain partial. After stop, a separate report channel on the same pinned SSH connection transfers server evidence. Output adds `server_evidence_transfer` and `reconciliation`; the `local` snapshot remains endpoint-only. Compatible complete records yield directional application delivery at each receiver’s own cutoff. Missing/partial/incompatible records keep delivery unknown; underlying packet loss stays unknown. `reconciliation.one_way` separately reports interval estimates or unavailable timing. A completed lifecycle does not prove timely delivery. Standalone `preflight` remains a separate local check.

Each endpoint saves `reports/<run-id>-client/` or `reports/<run-id>-server/` under its own data directory: `events.jsonl`, `manifest.json` and, after finalization, `summary.json`. Settings and an incomplete manifest are synced before measurement; raw records preserve host-local decimal nanoseconds and original identities. The final manifest governs storage completeness; a summary or final cutoff alone cannot prove successful publication. Existing run files are never overwritten or automatically deleted. The client stores transferred fixed files under `remote-server/`, a `server-transfer.json` receipt and a separately derived `reconciliation.json`; these never rewrite its endpoint-only manifest/raw summary.

Storage admission is nonblocking, with an independent 128–8192-record FIFO derived from `ledger_messages`, and an active-inclusive `event_bytes` byte cap. Physical `queue_messages` remains a send-queue setting. `file_bytes` covers raw envelopes, settings, summaries and simultaneous publication staging files; space is reserved before start, and infeasible budgets reject measurement. A queue/file/disk failure stops the local run as partial. `shutdown_timeout_ns` separately bounds the storage-worker join; a stalled filesystem can leave incomplete files until the OS call returns. File sync/atomic replacement is checked, but power-loss/nonlocal-filesystem durability remains unvalidated. [Persistence flow](../internal/control/evidence.go).

Transfer uses contiguous numbered 32 KiB chunks, SHA-256 checksums, acknowledged offsets and a final content hash. Server files share the explicit per-endpoint `file_bytes` ceiling (at most 1 GiB); report requests consume the remaining `control_requests` budget. `setup_timeout_ns` bounds the network transfer. Insufficient budget, interruption or bad content leaves the run partial and local evidence usable; no reconnect/resume or automatic deletion occurs. Both endpoints must run the current report-channel implementation. [Transfer flow and limits](../internal/control/transfer.go).

Only one `client-run` can reserve a client data directory. Normal exit removes `active-run.lock`; after a crash, inspect it and verify no process still owns that run before removing the stale file. The server's own lease bounds cleanup when a client disappears. [Finite lifecycle evidence](WORK_LOG.md#2026-10-11-finite-run-lifecycle), [implemented flow](../internal/control/run.go).

## Replay and export local reports

Use the run directory returned as `local_report_path`. Choose an explicit aggregate input-byte limit; these example limits are editable operator inputs, not a heap-memory cap:

```powershell
.\build\sneaky-port.exe report-replay -report .\client-data\reports\<run-id>-client -max-bytes 4194304
.\build\sneaky-port.exe report-export -report .\client-data\reports\<run-id>-client -max-bytes 4194304 -format csv
```

Both commands read local files and write to stdout, with no server connection or source-file changes. Replay validates schema/identities, frozen plan, raw checksum and declared cutoffs, then recomputes the saved summary through the shared ledger. A finalized `saved` summary mismatch rejects the report. Partial reports can be exported successfully; check `status`, `reason`, integrity and verification fields. Initial manifests and torn trailing writes remain partial, with discarded tail bytes recorded. Failed storage can expose a summary mismatch rather than inventing missing observations.

Choose `json` for settings and recomputed summary, `jsonl` for a typed `local_report` header plus raw endpoint envelopes, or `csv` for long-form field/unit/value/status rows. Exact integers, nulls and unknown reasons survive export. The explicit input cap (maximum 1 GiB) includes local files, the transfer receipt and both remote hash/replay passes. Replay rederives delivery and one-way intervals instead of trusting cached reconciliation JSON; checksums/provenance establish offline consistency, not transferable authentication. Missing remote evidence leaves delivery/timing unknown; packet loss remains unknown. [Replay flow](../internal/control/replay.go), [evidence](WORK_LOG.md#2026-10-11-local-report-replay-and-export).

`client-run` also returns `clock_sampling`; replay/export derives the same bounded window from raw `clock_capture` events. Explicit profile cadence, minimum recent valid samples, window and expiry control captures during warmup/main/grace. Four timestamps use the data epochs; serialization/send costs, invalid captures, skipped cadence slots and calibration application-byte counters remain visible. `samples_available` means enough recent raw constraints; `unavailable` means insufficient/expired evidence. Neither establishes calibrated one-way delay. Snapshot ages refer to the saved `as_of_ns`; replay does not advance a finished run's local clock. Readiness-only runs do not sample. [Sampling policy](DESIGN.md#calibration-and-one-way-evidence-policy), [source](../internal/control/clock.go).

`reconciliation.one_way` matches original attempts and complete receiver arrivals, then applies the clock revision available at client send time upstream or client arrival downstream. The report records raw-file hashes and retained results' identities, model/sample IDs, age, signed lower/upper/midpoint/uncertainty and clock-local units. Application serialization/submission/buffering are included. Production bounds are unverified, so values remain `estimate` or `unavailable`; source labels cannot enable `calibrated`. Later samples never upgrade old results. Detail bytes are capped at the smaller of the profile event budget and 256 KiB; exact status counts and omitted-model/result counts remain visible, with raw evidence retained. [One-way flow](../internal/control/one_way.go).

### Cross-endpoint CLI validation

Task 23 validates native Windows x86-64 CLI processes against an isolated Ubuntu x86-64 WSL2 server over its private virtual network. Pairing, UDP/TCP readiness/completion/console interruption, endpoint replay/exports and owned-resource cleanup pass. Bare-metal Linux and live ISP acceptance remain separate. [Recorded environment and results](WORK_LOG.md#2026-10-12-windows-to-linux-cli-validation).

To rerun the [opt-in fixture](../cmd/sneaky-port/cross_endpoint_windows_test.go), select an existing WSL distribution with `python3` and a reachable non-loopback IPv4 address:

```powershell
$env:GOCACHE = "$PWD\.cache\go-build"
$env:SNEAKY_PORT_TASK23_WSL = "Ubuntu"
go test ./cmd/sneaky-port -run '^TestWindowsLinuxCLI$' -count=1 -v -timeout 5m
Remove-Item Env:SNEAKY_PORT_TASK23_WSL
```

The fixture builds both current binaries and creates fresh stores/ports; it leaves existing services and identities intact. Local outputs stay under ignored `test-output/task23-*`; Linux keys/reports stay in its private `/var/tmp/sneaky-port-task23-*` directory, surviving idle WSL restarts. Cancellation exports local partial evidence without server transfer or delivery claims. Ordinary tests skip this fixture unless explicitly enabled.

## Protected local run API

### Dashboard setup and run controls

The dashboard contains **Test** only: setup, **Run test / Stop / New test**, current-test metric cards and charts. Settings expand through **More settings / Adjust**; TCP, Burst and custom port/content choices reveal applicable controls. Saved results and Compare sections are removed; stored reports and protected APIs remain. Inline groups retain unfinished edits without sending measurement commands. [Completed Chrome checks](ROADMAP.md#23c-verify-the-rendered-dashboard) record rendered layout, interaction, keyboard and responsive evidence.

After launching `client-api` below, open its exact `api_url`. The dashboard opens automatically, with no session key to copy or paste. An exact-origin POST grants the page-memory bearer; private API calls still require bearer/request identities and literal-loopback/Host/Origin/peer checks. No token enters a URL, cookie or browser storage.

1. In the top bar, enter server IP, normal SSH port, username (editable default `sneak`) and password, then **Connect**. The first fingerprint is automatically trusted and saved after successful authentication. A changed saved fingerprint stops before password delivery and requires the inline replacement checkbox. Public pins live only in the application store, separate from system SSH.
2. The packaged dashboard uploads its matching Linux x86-64 helper over SSH, verifies SHA-256 and starts it in an exclusively created owner-only temporary folder. There is no executable-path field or manual server copying. The SSH account must permit commands and forwarding to the private control listener; Linux needs standard `mktemp`, `head`, `sha256sum`, `chmod`, `rm` and `rmdir` tools. Only owned upload/helper folders are removed on disconnect; existing files/services/stores/system SSH remain intact. Startup has a 120-second budget, with fixed platform/storage/checksum/runtime failures. Passwords are cleared after authentication/failure and never saved in profiles/reports. **Refresh** recovers unknown acknowledgments or public setup progress; **Details** shows the helper digest and saved host fingerprint.
3. Name the test, choose UDP/TCP and edit duration in seconds, receipt deadline in milliseconds, payload bytes and independent packet rates. **More settings** opens traffic/timing, ports/priority, phases, saved configuration, limits, clocks, payload content and protocol options. Main inputs edit the selected phase independently. **Automatically size test resources** covers evidence/storage plus heartbeat/calibration and report-transfer capacity without changing traffic or clock/heartbeat cadence. Manual limits remain explicit; insufficient duration budgets block start with guidance. **Clock & local timing / Check local timer** unlocks bounded local timer results.
4. **Run test** validates/saves unsaved edits with revision checks, freezes that exact revision, and negotiates plan/effective settings/data binding with the server before traffic. Storage identities and run handles are managed internally; no UID or JSON is required. Test names derive safe saved names; an existing/stale name rejects without overwriting. Later edits affect the next test, while the running test retains its admitted settings. **Saved tests** loads/saves named configurations.
5. **Stop** cancels the managed current test. Unknown start acknowledgments expose **Retry same request**, preserving the original ID/hash; **Refresh results** reads status and retries only result loading. Reload automatically recovers the admitted active run through protected setup. Terminal reports load automatically into the cards/RTT plot when available. **New test** resets the editable draft and display while retaining saved evidence and SSH connection. Disconnect remains disabled until active work/retries are resolved; API shutdown joins owned work.

Protocol controls use actual registry contracts; unsupported complex extension values remain unchanged and require CLI editing. Receipt outcomes are numbers, including late receipts within missed deadlines. One latency chart shows client attempt-to-receipt RTT in **milliseconds** against elapsed **seconds**. Live ranges retain completed min/max/count in up to 128 one-second buckets; missing receipts create no latency sample. Final or partial replay replaces provisional ranges and supplies exact RTT/variation quantiles and coverage notes. Pending receipts/missed deadlines are not packet loss. Run failure reasons appear on the main page. Current results load through the admitted handle under a 64-MiB aggregate replay cap and 30-second HTTP / 35-second browser budgets. Ordinary saved/import/compare caps remain unchanged. Requested markings are not capture evidence; richer controls remain open under T25. [Current run flow](../internal/localapi/web/workspace.js), [latency flow](../internal/measure/progress.go).

Build a self-contained Windows dashboard from this checkout (PowerShell; use `-Platform linux` for a Linux x86-64 dashboard), then launch without arguments. Ordinary untagged CLI builds do not contain the upload payload:

```powershell
.\build-dashboard.ps1
.\build\sneaky-port-dashboard.exe
```

Review the built-in settings in the dashboard, or append `-profile FILE` for an explicit override. Startup JSON returns `api_url`, a fresh `session_token`, `profile_id` and `profile_revision` (canonical selected plan hash). A saved profile with that ID takes precedence; otherwise the compiled or file-loaded plan is a transient fallback. Startup does not save profiles or generate keys; dashboard SSH Connect generates a managed identity if absent. The token is retained for external API clients; the dashboard obtains it automatically. Each request needs `Authorization: Bearer <session_token>` and a safe `X-Request-ID` (1-64 letters, digits, `_` or `-`). Browser requests must use the exact returned origin; no CORS access or token-in-URL bootstrap is provided. `-listen` accepts a literal loopback IP only, defaulting to `127.0.0.1:0`; `[::1]:0` supports IPv6. Ctrl+C cancels and joins owned work within `-shutdown-timeout` (default 10s), reporting expiration explicitly.

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

Body/header request IDs must agree. Repeating an accepted mutation ID returns its original acknowledgment; reusing it for another operation conflicts. One run executes at a time, independent of HTTP connection/request cancellation. `executing` covers setup/data/finalization; `cancelling` lasts until cleanup returns. Measurement ID/readiness and local completeness become known at finalization; unknown booleans remain `null`, and progress carries bounded provisional client-local phase/counters. Failure reasons direct you to local evidence without exposing file/key paths. Existing replay/export commands remain available.

Start/cancel bodies are capped at 4 KiB; saved-operation bodies at 1 MiB including base64 overhead. Every mutation body carries `schema_version: 1` and matching `request_id`. The process retains at most 32 run statuses and 128 accepted start/cancel commands without evicting retries. Saved catalogs inspect at most 256 directory entries; report replay charges at most 8 MiB of file reads (including remote verification/replay), and serialized export is capped at 16 MiB. One saved operation executes at a time; busy requests return 503. Larger reports remain inspectable through the CLI with its explicit byte budget. These are engineering limits, not workload recommendations.

Report imports accept `manifest.json`, `events.jsonl`, optional referenced `summary.json` and `server-transfer.json`, and the complete transferred `remote-server/{manifest.json,events.jsonl,summary.json}` group. Import uses existing checksum/schema/accounting validators; cached reconciliation is excluded and recomputed. Offline integrity does not authenticate imported evidence. Imports never connect to a server, enroll keys remotely or resume runs. Explicit profile saves affect future starts; active plans and retry acknowledgments stay frozen. Validation/saves also reject unsupported registered-adapter options. Saved-operation request IDs provide correlation; automatic retry deduplication applies to start/cancel only. Saved diagnostic views are implemented; live counters are available through run status and matched saved comparison is implemented below. [API flow](../internal/localapi/server.go), [saved operations](../internal/localapi/saved.go).

### Diagnostic and distribution views

The Test dashboard automatically loads its terminal client-origin main results. Protected diagnostic APIs additionally support explicit local/transferred-server origins and main, warmup or phase scopes; the dashboard has no report-history section. Missing transferred evidence rejects. Endpoint-only views retain delivery/packet loss as unavailable; reconciled exports remain separate.

Quantiles use every eligible completed raw sample, including late receipts strictly before cutoff. Missing/local groups stay marked; quiet phases invent no losses. Diagnostic time/ECDF projections use at most 128 groups retaining exact extrema/counts/ranks and phase boundaries; coordinates are approximate. General report diagnostics keep the 8-MiB aggregate cap. `GET /api/v1/runs/<handle>/diagnostics` uses a 64-MiB cap only for a terminal run report, verifies admitted plan/measurement identity and shares single-operation disk admission. Its response gets 30-second HTTP / 35-second browser budgets; filesystem calls remain noninterruptible. No replay occurs during traffic.

**Optional local timer preflight** takes explicit positive decimal-string duration/interval ns: at most one second and 4,096 scheduled slots. It reports local wake lag/missed slots and exact nearest-rank tails without network probes or OS changes. Source/tick provenance is reported; accuracy/drift bounds remain unverified. Windows finite runs and preflight use QPC/QPF; other platforms use Go monotonic timing. Reports retain optional `clock_details`; comparisons distinguish known provenance from different or absent details. Timer lag does not establish network delay or clock calibration. Requests stop on disconnect/shutdown; interruption is explicit. Start/cancel retry deduplication does not apply to preflight. [Clock flow and limits](../internal/elapsed/epoch.go).

Protected routes: `GET /api/v1/reports/<id>/diagnostics?endpoint=local|server&scope=main|warmup|<index>` and `POST /api/v1/preflight` with `schema_version`, `request_id`, `duration_ns`, `interval_ns`. The existing token/Host/Origin/peer guards cover both. [source](../internal/localapi/diagnostics.go), [evidence](WORK_LOG.md#2026-10-12-diagnostic-and-distribution-views).

## Compare matched saved runs

Saved comparisons remain available through the protected API below; their dashboard section is removed. Submit distinct A/B report IDs (at most eight pairs), an explicit origin/scope and only predeclared adapter, port, client_dscp or server_dscp axes matching both frozen profiles. No axes requires identical configurations. The port axis permits data port differences; the adapter axis permits compatible contract/options differences. Other workload, environment and evidence compatibility rules below remain mandatory.

Before the runs, provide matching `matched_metadata`, including a nonblank `environment` description of the owned server, client/path, external load and other relevant conditions. These are operator declarations. Selection order and `created_utc` storage labels do not verify alternating/sequential execution, uncontended traffic or environment. Different retained endpoint/peer platform, build or clock provenance rejects matching where available; absent remote provenance remains a limit. Actual offered traffic, setup, framing/outer overhead, source/NAT bindings and requested/applied/observed markings remain in settings/raw exports for review. A selected axis never proves priority effects or wire equivalence.

The protected API uses the normal schema/request envelope, for example:

```json
{"schema_version":1,"request_id":"compare-1","pairs":[{"a":"trial-a1","b":"trial-b1"},{"a":"trial-a2","b":"trial-b2"}],"axes":["port"],"endpoint":"local","scope":"main"}
```

Every pair retains settings/provenance, completeness, exact populations and local faults. Tables show deadline misses and exact `misses/planned slots` fractions first, then completed RTT p95/p99 and variation tails/counts, reconciled application delivery where available, skips/writes and shared spike/stall episodes. Deltas are exact signed **B minus A**, including rational fractions such as `1/3`; missing values remain unavailable. Zero planned slots has no fraction. Main originating-phase rows remain separate even when inspecting warmup or another scope. Quiet phases invent no samples; completed tails never hide unresolved traffic. Existing strict cutoff and origin-phase rules apply.

Repeated views show the minimum/maximum of the individual trial deltas, never pooled p95/p99. At least two complete compatible pairs are required for a repeated description; absent trials, partial evidence and opposite-sign deltas remain inconclusive. Exact equal available values are ties. There is no material-difference threshold, significance test, confidence score, winner or combined score. One-way comparison stays unavailable while production clock bounds are unverified; packet-loss claims and ISP attribution are not derived. Offline consistency never authenticates evidence.

Admission rejects duplicate report IDs or duplicate measurement run IDs (imported copies are not repetitions), invalid/tampered evidence and excessive work. One serialized saved operation shares an **8 MiB aggregate file-read budget** across all selected runs and transferred-file validation passes, with at most 16 distinct runs, 64 phases per run and a 16 MiB serialized result. Valid comparisons have a 25-second cooperative work budget and a 30-second response window; earlier caller deadlines apply. Cancellation is checked between bounded replays and cannot forcibly interrupt a filesystem call. Budget exhaustion returns a fixed resource error when the response remains writable. These are engineering caps, not heap or performance guarantees. Nothing writes reports, starts traffic or changes settings. Comparison responses retain per-trial settings and metric evidence. Legacy comparison readers remain outside the current Test dashboard; their rendered QA is unqualified. Full network acceptance remains pending. [source](../internal/control/compare.go).

## Evidence presentation

Saved replay/API reports and JSON/JSONL/CSV exports include `evidence_presentation` with policy version 1, independent main/warmup/phase labels and clock/marking/environment context. Diagnostics remain endpoint-only; comparison delivery may use existing reconciled receiver gates. Coverage is complete/partial/unavailable/incompatible; basis distinguishes locally measured, receiver reconciled, estimated, operator declared and unverified evidence. Complete observations may show poor quality or losses. Counts, quantiles, ties and inconclusive decisions remain unchanged; labels are descriptive, with no score or winner.

**Evidence presentation** fields select page-local advisory targets: median 20, p95 200, p99 1,000 completed observations and three compatible A/B pairs by default. Require 1 ≤ median ≤ p95 ≤ p99 ≤ 1,000,000 and repeat target 1..8. Targets below/above defaults never change traffic, plan hashes, raw files or mandatory two-pair/clock/receiver gates. Existing views retain the policy loaded with them; re-read to apply edits. Variation uses actual adjacent completed pairs; gaps remain separate. Eligible duration counts describe reached nonlocal slots, not an assertion of delivery. Reasons use a fixed public vocabulary and preserve missing/partial/fault evidence independently.

Diagnostic/export GET queries accept `median_target`, `p95_target`, `p99_target`, `repeat_target` as single canonical integer strings; comparison POST accepts optional `evidence_policy` with `policy_version: 1` and all four decimal-string targets. Omitted preferences use defaults. Overrides are derived output preferences and are not persisted. CLI replay/export uses defaults. **Download selected report** exports the full selected report in JSON, raw-inclusive JSONL or CSV, including all scopes and available transferred evidence; it is not filtered by the diagnostic scope. Existing authentication and input/output budgets apply.

[Accepted S06 policy](ROADMAP.md#s06-separate-connection-quality-from-measurement-confidence), [source](../internal/measure/evidence.go).

## Live run progress

Run status optionally includes `progress`: publication `as_of_ns`, strict `counts_cutoff_ns`, elapsed/total nanoseconds, client-local stage/phase, accounting validity, separate main/warmup counts, incoming peer data and bounded `rtt_time`. Each main-only RTT range has elapsed `start_ns`/`end_ns`, completed `count` and exact `min_ns`/`max_ns`; all integers remain decimal strings. At most 128 recent scheduled-time one-second buckets retain eligible first receipts, including late ones. Warmup, duplicates, missing/local outcomes and cutoff-equal receipts supply no RTT sample. Pending/missing receipts are not packet loss; peer arrivals do not prove complete remote delivery. No cross-host synchronization, live quantile or calibrated one-way claim is made.

The accounting owner incrementally maintains presentation counts/ranges and publishes copied atomic snapshots at most four times per second, plus a final update. Actual stopped cutoffs seal/rebase both counts and RTT; readers cannot mutate them or drive the engine. The browser coalesces one status poll per second, stops visibly on errors and recovers the active run through setup. Terminal replay loads automatically into the current cards/chart; partial or unavailable evidence stays explicit. Cancellation does not claim 100 percent completion. Rebuild/restart the owned API process and reload the page to load new embedded assets.

Rendered Chrome QA is complete for the current Test dashboard: conditional settings, exact edits, keyboard controls, narrow layouts, live/final UDP/TCP results, active recovery, partial cancellation, timer preflight, saved-name conflicts and cleanup pass. Fixtures use real loopback SSH/carriers/reporting with modeled Linux upload/shell. Shipping scope is Windows 11 x86-64 clients and Linux x86-64 servers. Native baseline, clock accuracy, outer marking and live acceptance retain their own gates. [Browser/reference evidence](WORK_LOG.md#2026-10-13-reference-runtime-and-rendered-browser-progress), [remaining acceptance](ROADMAP.md#task-35-current-acceptance-review).
