# Operator guide

Current UDP/TCP commands and evidence limits. [README](../README.md) is the quick start; [DESIGN](DESIGN.md) is the contract; [ROADMAP](ROADMAP.md#task-35-current-acceptance-review) lists unqualified acceptance gates.

## Default configuration

Windows starts the dashboard without arguments. Its default private store is `%APPDATA%\sneaky-port\client`. Linux no-command startup selects the standalone server; explicit `client-api` starts a Linux dashboard with `$XDG_CONFIG_HOME/sneaky-port/client` or `~/.config/sneaky-port/client`.

Optional `-data-dir FOLDER` reuses an explicit store without migration. Optional `-profile FILE` selects a complete strict YAML/JSON plan for `client-api`, `validate`, `client-ready` or `client-run`. Missing/malformed explicit files fail; no local `readiness.yaml` is loaded implicitly. A matching saved dashboard profile ID takes precedence over the transient launch plan; startup does not save a profile or create keys.

The built-in `default-udp` starter is IPv4 UDP, `127.0.0.1:32000`, pairing `lab-server`, 10 s, independent 60 packets/s and 32-byte content each direction, DSCP 0, 100 ms deadline/grace. Dashboard SSH setup selects the real server. These are editable starter values, not FPS tuning or verified clock bounds. [All compiled values](../internal/profile/defaults.go).

<a id="run-the-foundation-tools"></a>

## Build and check

Go 1.27+ is needed for source builds, not packaged binaries. From this checkout:

```powershell
.\build-dashboard.ps1
.\build\sneaky-port-dashboard.exe
```

The builder creates a Windows x86-64 dashboard and embeds its matching Linux helper. `-Platform linux` builds the Linux dashboard. Untagged CLI builds do not contain the upload payload.

Use owned-source scopes because root application stores may be access-restricted:

```text
go test ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
go run ./cmd/sneaky-port adapters
go run ./cmd/sneaky-port validate
go run ./cmd/sneaky-port preflight -duration-ns DURATION -interval-ns INTERVAL
go run ./cmd/sneaky-port keygen -output NEW_PRIVATE_KEY_FILE
```

Use workspace-local `GOCACHE` if the global cache is denied. Windows race checks need CGO plus a compiler; the recorded setup uses MSYS2 UCRT64 GCC with session-local PATH/CC/CGO_ENABLED. A build is not runtime qualification. Linux execution is currently user-deferred. Remove owned test scratch after checks; preserve user state/current deliverables/dependencies.

### GitHub Actions

[CI](../.github/workflows/ci.yml) checks Windows Server 2025/Ubuntu 24.04 x86-64 source tests/race/vet/formatting, the Node harness and bundled-helper equality. LF [attributes](../.gitattributes) avoid Windows checkout/gofmt drift.

[Release](../.github/workflows/release.yml) validates an exact version tag/revision, requires checks, packages both archives and publishes a prerelease. Existing assets are not overwritten. Local package checks do not prove a tagged hosted release is repaired.

```powershell
.\.github\scripts\package-release.ps1 -Platform windows -Version v0.1.0-alpha.2
```

Choose a new version; `-Platform linux` targets Linux. The packager verifies actual build settings/helper equality, public inventory/hashes, extraction and available native command smoke. `BUILD_INFO.json` records dirty state/revision/toolchain; archives contain binaries, README, metadata and licenses, not test fixtures/private state. [Script](../.github/scripts/package-release.ps1).

## Dashboard setup and run controls

Open the exact printed `api_url`. Enter normal Linux SSH IP/port (usually 22), username (editable `sneak`) and password, then Connect. The account needs command execution/forwarding plus `mktemp`, `head`, `sha256sum`, `chmod`, `rm` and `rmdir`. The chosen UDP/TCP data port must be reachable directly.

First SSH fingerprints are trusted after authentication; changed pins require explicit replacement before password delivery. Passwords are transient and cleared. The bundled helper is uploaded/checksummed into owned private temporary storage, started and removed on disconnect/shutdown; existing services/installs and system SSH settings remain untouched. Startup is bounded to 120 s and upload to 64 MiB.

Review conditional protocol, ports, phases/rates/content, duration/deadline, DSCP and resource settings. Run test validates/saves the edited revision and freezes exactly those settings. Stop cancels only that run; Ctrl+C joins owned work. One measured run executes at a time. Invalid/unsupported required settings fail visibly. Resource autofill does not change offered traffic.

The current UI is Test-only: no report-history or Compare sections. Receipt outcomes are numeric cards; one RTT chart shows milliseconds over elapsed seconds. Live client-local snapshots are provisional; terminal replay supplies complete/partial results. Rebuild/restart/reload to use changed embedded assets.

## Start the standalone Linux server

```sh
chmod +x ./sneaky-port-server-linux-amd64
./sneaky-port-server-linux-amd64
```

This starts app-owned key-only SSH on **TCP 22022**, using built-in ceilings and automatic private state under `$XDG_CONFIG_HOME/sneaky-port/server` or `~/.config/sneaky-port/server`. Clients supply the test plan after trusted enrollment. Normal OS SSH 22/dashboard helper control are separate. Ctrl+C stops the owned endpoint.

Fresh-state optional `server -listen IP:PORT -advertise REACHABLE_IP:PORT` selects explicit bindings. Existing identities/authorizations/bindings survive restart. Legacy `server-init -policy FILE` remains a complete strict override; invalid input never falls back. [Policy schema](../internal/control/store.go), [compiled limits](../internal/control/default_policy.go). Latest Linux startup runtime checks are deferred.

## Pair endpoints and check readiness

For a new standalone CLI pairing, use a new private client store; initialization refuses existing identities. The packaged dashboard executable also provides CLI commands.

1. On Windows:

```powershell
$clientState = '.\profiles-local\standalone-client'
.\build\sneaky-port-dashboard.exe client-init -data-dir $clientState -client-id my-client |
    Set-Content -Encoding utf8 .\client-public.json
```

2. Transfer only `client-public.json` to the owned Linux server through a trusted channel. Start its standalone binary, then in another terminal:

```sh
server_state="${XDG_CONFIG_HOME:-$HOME/.config}/sneaky-port/server"
./sneaky-port-server-linux-amd64 server-enroll -data-dir "$server_state" -enrollment ./client-public.json
./sneaky-port-server-linux-amd64 server-export -data-dir "$server_state" > server-public.json
```

3. Transfer only `server-public.json` back. Import trust and create an optional local plan from compiled settings:

```powershell
.\build\sneaky-port-dashboard.exe client-import -data-dir $clientState -pairing-id lab-server -bundle .\server-public.json
New-Item -ItemType Directory -Path .\profiles-local -Force | Out-Null
$plan = (.\build\sneaky-port-dashboard.exe validate | ConvertFrom-Json).plan
$plan.server_address = 'YOUR_SERVER_IPV4'
$plan | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 .\profiles-local\run.json
notepad .\profiles-local\run.json
.\build\sneaky-port-dashboard.exe validate -profile .\profiles-local\run.json
.\build\sneaky-port-dashboard.exe client-ready -data-dir $clientState -profile .\profiles-local\run.json -hold-ns 1000000000
```

Readiness authenticates/binds the plan but sends no scheduled measurement workload. `-hold-ns 0` ends immediately; positive holds use heartbeats within the control budget. For TCP set `"adapter_id": "tcp"` and `"adapter_options": {"tcp_no_delay": true}`, then validate again.

Private keys stay on their originating endpoints with restricted permissions/ACLs. `-key FILE` references a compatible existing key unchanged; encrypted keys prompt interactively. Host replacement needs trusted `client-import ... -replace`. Re-export server public trust with `server-export`; revoke using `server-disable -data-dir STORE -client-id ID`. Edit/re-enroll public authorization for explicit expiry/re-enable; revocation/expiry also affects active sessions. A stale store lock requires explicit inspection, not automatic theft.

Local pairing/profile/report files can expose identities/addresses even without secrets. Keep them private/out of Git; [.gitignore](../.gitignore) covers common local paths but cannot remove committed history.

## Run a finite measurement

After pairing and reviewing the complete plan:

```powershell
.\build\sneaky-port-dashboard.exe client-run -data-dir $clientState -profile .\profiles-local\run.json
```

Specialized CLI commands retain explicit identity/store/hold inputs even though dashboard startup needs none. The server persists between runs. Endpoints use independent local monotonic anchors; cancellation/control/lease/storage failure preserves actual cutoffs and partial/unknown outcomes. Product reports remain until explicitly removed.

`file_bytes` must reserve manifests/summaries/clock evidence as well as raw records. The starter uses 4,096 ledger entries/64 MiB; the smaller 1,024-entry/8 MiB reservation regression is tested. Readiness alone cannot validate disk budgets. Every planned slot remains accounted; queues/skips/stalls are not silently called packet loss.

## Replay and export local reports

Use `local_report_path` returned by the run and choose an explicit aggregate read budget:

```powershell
.\build\sneaky-port-dashboard.exe report-replay -report RUN_DIRECTORY -max-bytes 67108864
.\build\sneaky-port-dashboard.exe report-export -report RUN_DIRECTORY -max-bytes 67108864 -format csv
```

Formats are `json`, raw-inclusive `jsonl` and exact long-form `csv`. The CLI cap is at most 1 GiB of aggregate file reads, not heap usage; transferred hashes/replay passes also consume it. Replay reads only the expected run files.

Replay checks schema/identities/plan hash/raw checksums/cutoffs and recomputes summaries/reconciliation/clock models. It does not authenticate offline imports. Interrupted/torn/failed files remain partial or inconsistent; missing remote records remain unknown. Receiver-reconciled delivery is separate from endpoint-only receipt outcomes.

## Protected local run API

`client-api` accepts optional `-data-dir`, `-profile` and literal-loopback `-listen` (default `127.0.0.1:0`; IPv6 `[::1]:0`). Default shutdown budget is 10 s. Startup prints URL, bearer token and selected profile/revision. Dashboard same-origin bootstrap acquires access automatically; external callers need `Authorization: Bearer TOKEN` and safe `X-Request-ID` (1-64 letters/digits/underscore/hyphen). No wildcard CORS, cookie/URL-secret access or remote bind.

All mutations use `schema_version: 1` and matching body/header `request_id`. Example start body:

```json
{"schema_version":1,"request_id":"start-1","profile_id":"default-udp","profile_revision":"SELECTED_HASH"}
```

| Route under `/api/v1` | Purpose |
| --- | --- |
| `GET /setup`, `GET /adapters` | Selected plan/revision, public enrollment/active recovery; registry capabilities/options |
| `POST /runs`, `GET /runs/ID`, `POST /runs/ID/cancel` | Frozen start, bounded status/progress, owned cancellation |
| `GET /profiles`, `GET /profiles/ID`, `POST /profiles/validate`, `PUT /profiles/ID` | Saved configuration; PUT needs current revision or empty create-only revision |
| `GET /pairings`, `GET /pairings/ID`, `POST /pairings/import` | Public trust; replacement must be explicit |
| `POST /client-keys`, `GET /client-keys/ID/public` | Generate managed identity if absent; export public enrollment only |
| `GET /reports`, `GET /reports/ID`, `GET /reports/ID/export?format=json\|jsonl\|csv` | Saved replay and exact exports |
| `POST /reports/import`, `POST /reports/compare` | Validated fixed-file import; descriptive selected-run comparison |
| `GET /runs/ID/diagnostics`, `POST /preflight` | Terminal owned-report diagnostics; bounded local timing preflight |

Accepted start/cancel IDs return their original acknowledgment on retry; reuse for another operation conflicts. Saved-operation IDs are correlation, not automatic retry deduplication. Stale profile revisions return 409. One run persists independently of the initiating HTTP request; setup/readiness/completeness remain unknown until actually observed.

| Engineering limit | Cap |
| --- | --- |
| Start/cancel body; other saved bodies | 4 KiB; 1 MiB including base64 |
| Run statuses; accepted start/cancel IDs | 32; 128 without retry eviction |
| List page; catalog inspection | 16; 256 directory entries |
| Saved disk work | One operation; busy returns 503 |
| General saved replay/import/compare reads; serialized export | 8 MiB aggregate; 16 MiB output |
| Terminal owned-run diagnostics | 64 MiB, exact admitted identity; 30 s HTTP/35 s browser |
| Preflight | Explicit positive ns inputs, at most 1 s and 4,096 slots |

Import only allowed manifest/events/optional summary/transfer files and a complete transferred remote-server group. Cached reconciliation is excluded/recomputed. No import connects/enrolls/resumes a remote session. Source routing and body fields: [API](../internal/localapi/server.go), [saved operations](../internal/localapi/saved.go), [diagnostics](../internal/localapi/diagnostics.go).

## Diagnostic and distribution views

Terminal Test results load client-origin main evidence automatically. Protected APIs retain local/transferred-server and main/warmup/phase views; missing remote evidence rejects. Endpoint-only diagnostics cannot assert packet loss or complete receiver delivery.

Quantiles use all eligible raw completions. Time/ECDF projections use at most 128 groups with exact extrema/counts/ranks and explicit gaps; coordinates are approximate. Quiet periods do not invent loss. Timer preflight reports wake lag/missed slots/source provenance without network probes or OS changes; QPC precision does not prove clock accuracy.

## Compare matched saved runs

Use explicit A/B report IDs, permitted axes, endpoint and originating scope; no traffic/settings writes occur. Retain matching frozen workloads/resources/clock/marking/environment context, actual offered traffic and independent cutoffs. Selected axes do not themselves prove wire equivalence or priority effects.

Outputs are exact signed **B minus A**, including rational fractions; zero planned slots has no fraction. Deadline outcomes lead, then completed tails/variation/populations, reconciled delivery and local limits/spikes/stalls. Missing evidence remains unavailable. Repeated ranges use individual deltas, not pooled quantiles; at least two complete compatible pairs are required, while S06's advisory default is three. Opposite-sign/partial/missing trials remain inconclusive; equal available values are ties. No score, automatic winner, significance/material threshold, packet-loss attribution or unverified one-way comparison.

Reject duplicate report/measurement IDs, tampered/incompatible evidence and resource exhaustion. One operation shares 8 MiB aggregate reads, at most 16 runs/64 phases each and 16 MiB output, with 25 s cooperative work/30 s response limits. Cancellation cannot forcibly interrupt a filesystem call. Legacy comparison readers are outside the Test UI. [Comparison source](../internal/control/compare.go).

<a id="evidence-presentation"></a>
<a id="live-run-progress"></a>

## Evidence presentation and live run progress

S06 adds version-1 per-metric coverage/basis/count/reason, preserving measured values, plan/raw hashes and mandatory receiver/clock gates. Complete observations can show poor quality. Default advisory counts are median 20/p95 200/p99 1,000 and three compatible A/B pairs. Require `1 <= median <= p95 <= p99 <= 1,000,000` and repeat target 1..8. Preferences affect derived views only, not traffic/saved policy; variation counts adjacent completed pairs, not individual RTTs.

Diagnostic/export queries accept canonical `median_target`, `p95_target`, `p99_target` and `repeat_target`. Compare POST accepts optional version-1 `evidence_policy` with all four decimal-string targets. CLI uses defaults; full selected-report downloads include all scopes/available transferred evidence.

Run status snapshots identify `as_of_ns`, strict counts cutoff, local stage/phase, separate warmup/main outcomes and up to 128 recent one-second RTT ranges with exact ns min/max/count. Valid first receipts, including late ones before cutoff, contribute; duplicates/warmup/missing/local outcomes do not. Publication is at most four/sec and polling one/sec. Pending receipts/peer arrivals do not prove network loss or full remote delivery; no live quantile, synchronized phase or calibrated one-way claim. Terminal replay replaces provisional data.

## Adapter extension gate

Compile-time registration supplies one registry through profile/API/SSH, workers and replay/import/compare. Descriptors include fixed `framing_bytes` (UDP 0/TCP 4); factories implement real transport. Unsupported names/versions/options/families/frame sizes reject. Match current client/server builds because negotiation requires this descriptor field. The test-only adapter's distinct identity/options/1,024-byte cap exercise shared contracts without establishing another production carrier or broader platform acceptance.

## Cross-endpoint CLI validation

The opt-in `TestWindowsLinuxCLI` fixture builds isolated Windows/Linux x86-64 binaries and checks keyed UDP/TCP readiness/completion/cancellation/replay/export against an explicitly selected existing WSL distro. It is not native minimum/live acceptance. Linux runtime checks are user-deferred; do not execute this fixture merely for documentation verification. Current Test dashboard Chrome QA and older named VMware/kernel cases are recorded in [WORK_LOG.md](WORK_LOG.md); their separate limits remain.
