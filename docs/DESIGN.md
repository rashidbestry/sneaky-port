# sneaky-port design

Current UDP/TCP contract and technical decisions. [ROADMAP.md](ROADMAP.md) owns task/feature status, [USAGE.md](USAGE.md) commands, and [WORK_LOG.md](WORK_LOG.md) evidence. Full earlier wording remains in the [pinned historical record](ARCHIVE.md#full-historical-record).

<a id="first-implementation-slice-scope"></a>

## Confirmed intent and scope

Measure application connection quality between Windows 11 clients and one Linux server. Shipping is x86-64 only; ARM was removed on 2026-10-13. The gaming proxy is separate. No shaping, interception, automatic routing/switching, multipath or automatic winner is part of this product.

The user authorized the first UDP/TCP slice on 2026-10-11 and limited release decisions to it on 2026-10-12. Implemented behavior does not establish full acceptance. Authentic future TLS, QUIC, WireGuard, Hysteria2 and DNS coverage remains accepted scope, with its own unresolved mechanics and gates.

<a id="protocol-extensibility"></a>
<a id="components-and-adapter-boundaries"></a>
<a id="dashboard-normal-operation"></a>

## Application foundation

Go backend; embedded plain HTML/CSS/JavaScript and native SVG, without runtime CDN/chart dependencies. A shared engine owns timing/accounting; compile-time adapters own actual carriers/framing/options, platform code owns sockets/marking, and API/UI/reporting share validated contracts. UI/replay/export never schedule probes.

Adapters expose ID/version, families/platforms, framing/max-message costs and strict options. Unknown IDs/versions/options or unsupported required capabilities reject; no silent replacement. The test-only TCP-backed extension passes the shared lifecycle/configuration/report workflow but is absent from production. There is no runtime plugin loader. [Registry](../internal/adapter/adapter.go).

## Authentic protocol coverage

| Carrier | Current disposition |
| --- | --- |
| UDP/TCP | Implemented genuine datagrams/streams; common measurements and accounting |
| TLS/QUIC | Future authentic TLS and QUIC stream/datagram adapters; credentials, versions and platform evidence remain open |
| WireGuard/Hysteria2 | Future tool-managed authentic tunnels with inner UDP/TCP; outer costs/priority require their own contracts |
| DNS | Future authentic data carriers and direct/resolver-mediated experiments; transport/encoding/polling/framing remain open |

### DNS experiments

Preserve configurable resolver/server/source/destination ports and direct versus resolver-mediated paths, including intended UDP/TCP/DoT/DoH/DoQ modes where genuinely supported. Record caching/interception, authoritative/session identity, encoding/name/TXT/EDNS limits, fragmentation/reassembly, polling and requested/actual outer transport. Do not substitute ordinary lookup latency for data transport. DNS prioritization is an ISP-specific hypothesis; required zone/delegation/resolver prerequisites must be explicit.

<a id="protocol-traffic-tweak-choices"></a>

### Protocol-specific traffic controls

Current controls are payload length/repeated pattern, explicit TCP_NODELAY, ordered traffic phases, ports, directional rates and DSCP. Version-1 framing stays fixed. F10's richer header/padding/fragmentation/reassembly controls remain accepted future intent; forced logical/IP fragmentation and new version mechanics are deferred. Native IP fragmentation/TCP segmentation can occur without proving any particular MTU or wire behavior.

<a id="port-selection"></a>
<a id="independent-packet-priority"></a>

## Port selection and independent packet priority

Destination ports are explicit 1-65535; source is explicit or `auto`. Busy/unsupported choices fail rather than moving ports. Record requested, bound and observed tuples, including NAT/measurement location when known.

DSCP is independent numeric 0-63 per physical direction, including reverse receipts. Linux uses IP_TOS/IPV6_TCLASS; Windows uses qWave subject to capability/privilege. Preserve ECN and distinguish requested/applied/readback/on-wire observations. SO_MARK is a routing mark, not DSCP. Nonzero requirements must not silently fall back to unmarked traffic. Capture loss/offload/location and independent endpoint evidence limit priority claims.

<a id="measurement-contract-draft"></a>
<a id="provisional-example-workload-and-report"></a>

## First-slice implementation contract

<a id="run-lifecycle-and-identities"></a>
<a id="pairing-and-user-provided-certificate-flow"></a>

### Pairing, credentials and SSH coordination

- Linux no-command startup opens app SSH **22022** with compiled limits and generated private identity/authorization state. Clients supply complete test plans after authentication. No shared hard-coded keys/passwords or anonymous enrollment. Existing stores/bindings/overrides survive restart.
- The standalone app endpoint is key-only SSH with application channels, not a shell/SFTP/forwarding server. Public pairing/enrollment travels through a trusted channel; revocation/expiry cancels active work as partial. New host identity needs explicit trusted re-import.
- The dashboard instead logs into normal OS SSH (usually **22**) using transient password credentials and editable default username `sneak`. It generates a managed local identity when needed, uploads the matching bundled Linux helper into owned private temporary storage, verifies bytes/SHA-256, and forwards only helper control; measured data remains direct. It does not modify existing services or system SSH settings.
- Normal-SSH first pins are accepted after successful authentication; changed pins require explicit replacement before password delivery. Helper startup is bounded to 120 s; upload is capped at 64 MiB. SSH EOF/shutdown joins and removes owned helper/state. Original explicit keyed CLI pairing remains supported.
- Private keys stay restricted on their originating hosts; compatible existing keys may be referenced unchanged. Encrypted keys use an in-memory prompt. Profiles/reports contain no private keys, passphrases or session MAC keys.

Lifecycle: authenticate -> negotiate -> validate/freeze plan -> prepare/bind/authenticate sockets -> accept effective settings -> start/warmup/measure/grace -> reconcile/finalize/close. One measured run per endpoint. Start anchors and cutoffs are host-local; simultaneous phase starts are not claimed.

Control/lease/storage failures stop new generation and preserve partial evidence within bounded drain/shutdown. No reconnect/resume or endpoint migration. Missing authenticated receiver evidence leaves remote delivery unknown. [Control](../internal/control/session.go), [managed SSH](../internal/control/ssh_dashboard.go).

<a id="profile-result-and-dashboard-fields"></a>

### Versioned schemas and local API

All current profile/control/API/probe/result contracts are version 1. Strict YAML/JSON rejects unknown/duplicate fields, tags/aliases/merge keys, invalid options, nonfinite values, overflow and infeasible resource demand. A SHA-256 digest of RFC 8785 integer/string-domain canonical JSON binds the frozen secret-free plan.

64-bit times/counts/sequences are exact decimal strings; signed derived differences remain signed. Small ports/versions/DSCP are validated numbers. Unknown values are `null` with status/reason, never a fabricated zero; fractions retain numerator/denominator and exports retain units.

| Plan/evidence group | Required content |
| --- | --- |
| Identity/path | Schema/profile/pairing/adapter IDs, contract, family, server, destination/source ports, options |
| Workload | Warmup, ordered unique phases, grace, receipt deadline and optional phase overrides |
| Direction | Steady/burst/quiet; rational rate, content length/base64 pattern, active/gap duration, DSCP |
| Limits | Queue/message/byte/reserve/outstanding/ledger/event/file caps; lag, write/setup/shutdown, heartbeat/lease, control and calibration/preflight bounds |
| Analysis | Explicit spike threshold, comparison axes and matched metadata |
| Raw evidence | Event/run/session/endpoint/epoch identities; local ns, phase/direction/sequence, attempts/arrivals/tickets/faults/cutoffs |
| Summary | Requested/effective settings, hashes/build/platform/adapter IDs, completeness, exact populations/distributions, delivery/marking/clock provenance |

Profile/control documents are capped at 1 MiB. UDP's complete authenticated message cap is 65,507 bytes; TCP's logical frame cap is 1 MiB. Report smaller carrier/platform constraints explicitly; baseline UDP has no application fragmentation or fallback.

The local API is loopback-only with peer/Host/Origin guards, per-process bearer token and bounded IDs/bodies. Explicit saves use revision checks; admitted plans and accepted start/cancel acknowledgments stay frozen. The current dashboard polls bounded status, rather than driving measurements. See the [implemented API and limits](USAGE.md#protected-local-run-api); an earlier SSE-event design is not a claim that an event-stream route is implemented.

<a id="independent-traffic-and-timing-boundaries"></a>

### UDP/TCP payload and timing boundaries

Each message has a **92-byte** big-endian header, selected content and **32-byte HMAC-SHA-256**: fixed overhead **124 bytes**. UDP carries one message per datagram. TCP adds a **4-byte** big-endian length. Receipts have zero content; binding carries a 32-byte server challenge outside workload counts.

| Header field | Size/meaning |
| --- | --- |
| Magic/version | `SPV1`: 4 bytes; unsigned 16-bit version 1 |
| Kind/direction | 1 byte each; bind request/ack=1/2, data/receipt=3/4; origin client/server=1/2 |
| Lifecycle/flags/reserved | 1/1/2 bytes; bind=0, warmup=1, main=2; flags/reserved zero |
| Run/session | 16 bytes each |
| Phase/sequence/content length | Unsigned 32/64/32 bits; phase index and original sequence |
| Origin/peer epoch | Unsigned 32 bits each |
| Origin attempt/peer arrival/peer receipt attempt | Unsigned 64-bit host-local ns each; peer fields zero in data |

Separate random 32-byte keys authenticate each physical direction. HMAC input is `sneaky-port/probe/v1`, a zero byte, 32-byte plan digest, two-byte adapter-ID length, UTF-8 adapter ID, then header/content; compare tags in constant time. Raw UDP/TCP payloads are authenticated, not encrypted TLS/tunnel traffic.

The challenge binds the observed UDP peer or TCP connection before downstream work. Wrong identity/tuple/version/tag/content or impossible replay rejects within budgets. Valid duplicates are separate counts, never extra RTT samples or repeated receipts.

Attempt time is after queue waiting, immediately before encode/authenticate/write. First socket call and complete/partial write are separate evidence. Arrival is complete datagram/frame read, captured before validation; only subsequently valid arrivals qualify. RTT is origin attempt to first valid receipt arrival; scheduled-origin completion also includes sender lag/queue delay. These are application boundaries, not wire timestamps. Partial frames never count as delivery. [Codec](../internal/probe/wire.go), [workers](../internal/worker/endpoint.go).

<a id="timing-and-scheduling-correctness"></a>

### Phase, scheduling and resource policy

- Each host uses independent monotonic half-open phases: start included, end excluded. Rate `n/d` schedules `floor(k*d*1e9/n)` for rational offsets inside the phase; reject unrealizable/overflowing schedules. Bursts reset slots within active intervals. Quiet/burst gaps plan no new data but may receive receipts.
- Warmup uses the first phase's directional pattern with separate identities. Main restarts the pattern; sequences are never reused. Data, receipt and resolved deadline belong to the originating phase.
- Completion exactly at deadline is on time; after it is late. Only receipts strictly before cutoff affect finalized metrics. No cross-host anchor subtraction.
- At a late wake, account all due slots, skip earlier overdue slots and admit at most the latest within explicit lag bounds. Never catch up with a burst. Local admission/scheduler skips stay separate from carrier outcomes.
- One bounded FIFO writer per physical direction includes active work in message/byte accounting and reserves receipt capacity. Frame bytes are 124+content, excluding TCP/outer framing. Data ceilings are `queue_messages-receipt_reserve` and `queue_bytes-124*receipt_reserve`. Receipt exhaustion is a local receipt failure, not data loss.
- Outstanding expiry occurs strictly after deadline and before new admission; late identities remain recognizable. Partial/failed writes terminate the segment with their byte/ticket evidence.
- DSCP is selected for the actual outgoing phase before encoding, independently of the message's origin phase. Quiet updates on the next outgoing frame; grace retains the last setting. A frame spanning a boundary gives no exact on-wire transition claim.
- Cancellation truncates actual observation/planned populations; it does not invent later slots or completed-run percentages. Server cutoff interrupts data I/O without forcing early peer EOF before control cleanup.

[Scheduler](../internal/schedule/schedule.go), [FIFO](../internal/writer/queue.go), [engine](../internal/engine/finite.go).

<a id="reproducible-evidence"></a>

### Persistence, replay and reconciliation

Persist bounded append-only endpoint-local JSONL plus manifests/settings before traffic; finalize summaries/manifests atomically with cutoffs and completeness. A 128-8192-record evidence FIFO is derived from ledger capacity; active queued bytes are capped. File budgets reserve summary/manifest/staging room and charge exact raw bytes. Exhaustion or write failure stops visibly; an expired disk join leaves incomplete evidence. Filesystem calls are not portably interruptible; power-loss/nonlocal durability remains unvalidated.

Product reports are not automatically deleted. Agent-owned test scratch is cleaned after checks. Offline replay uses fixed filenames, schemas/identities/hashes/cutoffs and an explicit aggregate input-byte budget, maximum 1 GiB, not a heap cap. Torn trailing bytes, failed storage and summary mismatches remain visible. JSON/JSONL/CSV retain exact units, empty unknown values and reasons; offline integrity does not authenticate evidence.

Post-stop transfer uses numbered **32 KiB** chunks, checksums, contiguous offsets and a final aggregate hash within control/file/setup limits. Exclusive `remote-server` files and sidecars preserve originals. Interrupted transfer leaves endpoint-only evidence, not remote loss.

Reconcile only compatible finalized replayed windows with matching plan/settings/run/session/epochs and original attempt/direction/phase/sequence identities. Separate missing application data, data arriving despite missing receipts and ambiguous writes. Independent cutoffs remain independent; unmatched receiver identities invalidate the affected direction. Replay rederives reconciliation, not cached JSON. [Evidence](../internal/control/evidence.go), [replay](../internal/control/replay.go), [transfer](../internal/control/transfer.go).

<a id="control-connection-and-clock-calibration"></a>

### Calibration and one-way evidence policy

Use one elapsed epoch for data, scheduling, calibration and cutoffs: Windows checked QPC/QPF, otherwise Go monotonic. Read/backward/overflow failures stop partially without source fallback. Source/frequency/tick provenance is not an accuracy bound. Valid RTT populations can be empty or quantized to zero; host-dependent tests must not invent positive completions.

Authenticated four-timestamp exchanges capture client t1, server arrival t2, server attempt t3 and client arrival t4. Preserve epochs, invalid attempts, serialization/socket-call costs, cadence/window/age/expiry and all valid constraints. Drift needs at least two time-separated valid exchanges; absolute cadence skips overdue slots. Raw capture demand reserves control/file/event capacity, including 4 KiB per capture.

For `S(c)=a+b*c` with positive b, nonnegative transit constrains `a+b*t1<=t2` and `a+b*t4>=t3`, widened by justified timestamp/model error. At b=1 the ideal offset interval is `[t3-t4,t2-t1]`; its midpoint does not prove symmetry. Feasible offset/drift envelopes need justified rate-change bounds, not only a fitted line.

Use only the model revision available at client send time upstream or client arrival downstream; later captures cannot upgrade old messages. Both captures must fit its validity lifetime and independent reconciliation gates. Output signed lower/upper/midpoint/uncertainty, revision/evidence IDs, age and units: server-local ns upstream, client-local ns downstream. Wholly negative/impossible intervals invalidate; crossing-zero bounds are not silently clamped. Derived detail is capped at min(event budget, 256 KiB), with omission counts.

| State | Gate |
| --- | --- |
| Calibrated | Independent timestamp/model/drift bounds, valid captures, allowed uncertainty and validated method |
| Estimate | Unverified assumptions or over-policy uncertainty; excluded from directional winner/timeliness claims |
| Unavailable | Missing/expired/inconsistent epochs or constraints; no directional value |

Production bounds remain unverified, so results stay estimated/unavailable. Clock/suspend/rate changes can invalidate mapping; calibration failure alone need not stop usable RTT/delivery. [Elapsed source](../internal/elapsed/epoch.go), [clock capture](../internal/control/clock.go), [one-way derivation](../internal/control/one_way.go).

<a id="delivery-accounting-and-statistics"></a>
<a id="fair-comparisons-and-fps-suitability"></a>
<a id="accounting-and-summary-concepts"></a>

### Presentation, comparison and acceptance handoff

Nearest-rank quantiles use all eligible completed samples, including late receipts strictly before cutoff; unresolved outcomes stay beside tails. RTT variation is the absolute difference between adjacent original-sequence completions in the same session/lifecycle/phase; gaps break pairs. Arrival gaps are local evidence, not transit jitter.

Spikes are completions strictly above the explicit threshold, grouped consecutively within phase and broken by gaps/non-spikes. Receipt-completion stalls require accepted overdue unresolved work and no valid completion; end at completion/cutoff and exclude intentional quiet periods without overdue work. Neither proves TCP packet loss or an Internet outage.

S06 labels each metric with coverage, basis, count and reason without changing values/mandatory gates. Advisory completed-sample targets are median 20, p95 200, p99 1,000 and three compatible A/B pairs; no confidence score or winner. Plots may use at most 128 bounded extrema/count groups; final quantiles remain raw-sample based. Live client-local updates are at most four/sec, browser polls one/sec, and terminal replay is authoritative.

Compare sequential repeated/alternating trials, varying only explicit adapter/port/DSCP axes while matching workload, phase/deadline/resource/analysis settings and retaining achieved traffic/environment/overhead/outer evidence. Show deadline outcomes first, then tails/variation and delivery/local limits. Ties/incompatible/partial/inconclusive results remain explicit. One-way gates apply independently to each run; priority-effect claims require relevant outer observations.

<a id="technical-decisions"></a>
<a id="first-slice-final-decisions"></a>
<a id="additional-open-choices"></a>
<a id="maintaining-choices"></a>

## Technical choices

Confirmed user choices and engineering selections retain their provenance; decisions are not runtime evidence.

| ID | Current decision / unresolved part |
| --- | --- |
| T01 | Go; user confirmed 2026-10-11 |
| T02 | Local browser dashboard; user confirmed 2026-10-11 |
| T03 | One executable per supported OS/CPU with roles; Windows client/Linux server; zero-argument startup selected 2026-10-15 |
| T04 | Embedded plain HTML/CSS/JS, no CDN; native SVG selected |
| T05 | Standard Go UDP/TCP now; authentic future carrier integrations/pins remain open |
| T06 | Shared engine, separate adapters/platform/API/UI; UI never schedules probes |
| T07 | Versioned strict YAML/JSON, shared validation and explicit save; built-in starter/optional profile selected 2026-10-15 |
| T08 | JSONL raw observations, JSON summary/settings and exact CSV; bounded replay/export/reconciliation implemented |
| T09 | Endpoint-local keys/public enrollment, explicit revocation/trust; future TLS certificate lifecycle open |
| T10 | One measured run per endpoint; sequential matched repeated/alternating comparisons; no automatic screening |
| T11 | Owned Linux/qWave marking; capability and independent outer observations remain required |
| T12 | Windows 11/Linux x86-64 only; ARM removed 2026-10-13; Ubuntu 22.04.5 baseline/minimum claims unqualified |
| T13 | Test-only conditional dashboard/current results, native SVG; no sidebar/history/Compare sections or chart build pipeline |
| T14 | Version-1 profile/control/API/probe/result and data-plus-receipt contracts; other-adapter framing open |
| T15 | Editable ordered steady/burst/quiet phases, payload/rates/timing/priority; defaults are starters, not FPS tuning |
| T16 | DNS records/encoding/framing/polling/fragmentation and transport limits remain open |
| T17 | Independent DSCP 0-63, no automatic presets; required unsupported marking rejects |
| T18 | Deadline-first descriptive comparisons and accepted S06 labels/reasons; no score/winner; future recommendation thresholds open |
| T19 | IRTT/iperf2/Crusader are external design/validation references, not embedded/bundled components |
| T20 | Pinned reference versions and baseline/held-out method selected; valid measured bands remain pending |
| T21 | Compile-time adapter registration confirmed 2026-10-11; test extension gate implemented; no runtime loader |
| T22 | Keyed app SSH for CLI; normal-SSH password dashboard/helper selected 2026-10-12, editable username `sneak` |
| T23 | Causal clock envelopes with operator quality inputs; independent production bounds remain open |
| T24 | Existing gap-aware RTT variation/spike/stall semantics; monitoring/jitter/MOS/availability formulas remain future choices |
| T25 | Current payload/TCP_NODELAY/version-1 framing; richer header/padding/fragmentation/reassembly controls deferred |

### Current UDP/TCP release decisions

Task-4 engineering policies were selected under the 2026-10-11 completion instruction; the user limited further resolution to UDP/TCP on 2026-10-12. Dashboard simplifications were selected on 2026-10-12/13, ARM removal on 2026-10-13, startup simplification on 2026-10-15. No later carrier or monitoring policy is accepted by implication. [Dated summaries](ARCHIVE.md).

<a id="reference-and-tolerance-selections"></a>
<a id="primary-references"></a>

### Reference comparison acceptance matrix

| Reference | Pinned baseline | Compatible overlap / evidence limits |
| --- | --- | --- |
| IRTT | v0.9.2, commit `0d612bb790034e5257a4979bcf655609f644d08a` | Local elapsed UDP RTT only where workload/boundaries match |
| iperf2 | 2.2.1 archive SHA-256 `754ab0a7e28033dbea81308ef424bc7df4d6e2fe31b60cc536b61b51fefbd8fb` | Compatible receiver-count/rate and TCP responsiveness; pacing warnings retained |
| Crusader | v0.3.2, commit `ac0c5b67040fb5cf07c1a97205b26c9af701ebc5` | Idle/upload/download/both-load responsiveness, not identical FPS traffic or calibrated one-way |

Before comparison verify source/binary hashes, licenses, native platform/compiler/options and feature/boundary compatibility. Deterministic identities/counts/content have zero tolerance. For each compatible quantity/environment collect **10 separate baseline pairs and two held-out pairs**; freeze the largest baseline absolute delta plus independently documented boundary uncertainty before target trials. A held-out failure invalidates calibration; do not widen bands afterward. These are engineering reproducibility bands, not statistical confidence/clock accuracy. Actual valid bands and task-33 acceptance remain pending.

<a id="integration-review-status"></a>
<a id="contract-acceptance-examples"></a>

## Validation before support/performance claims

Required gates cover trust/tamper/replay rejection; schema/length/HMAC precision; phase/deadline/fault accounting; bounded queues/storage/UI/cleanup; extension registration; clock uncertainty; actual ports/outer DSCP/ECN; independent compatible references; named native platforms and live paths. Build, loopback, WSL, impairment, rendered Chrome, native deployment and live ISP evidence are distinct. Do not infer a Linux minimum, calibrated one-way status, zero network loss or broader protocol support from local success. [Remaining gates](ROADMAP.md#task-35-current-acceptance-review).
