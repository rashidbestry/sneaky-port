# sneaky-port design

Status: design only. Implementation remains on hold until the user explicitly authorizes it. This is the single product reference; working instructions are in [AGENTS.md](AGENTS.md), backward tracking in [WORK_LOG.md](WORK_LOG.md), and forward feature/proposal tracking in [PROPOSALS.md](PROPOSALS.md).

## Confirmed intent and scope

Find which protocol, DNS mode, ports, and packet priority settings give the best connection to one owned server for FPS online games. A separate future project will use the findings to build a gaming proxy intended as an ExitLag alternative.

| Area | Confirmed requirement |
| --- | --- |
| Platforms | Windows 11 client only; Linux server only; 64-bit ARM64 and x86-64 on both; broad mainstream Linux coverage including CentOS, Ubuntu, Fedora, Arch, and other major distributions; exact version limits pending (T12) |
| Server count | One owned measurement server per comparison |
| Traffic | Real TCP, UDP, TLS, QUIC, WireGuard, Hysteria2, and DNS experiments |
| Protocol setup | The tool manages its own protocol endpoints, credentials, sessions, and cleanup |
| Protocol extensibility | Add new probe protocols in later releases through a common adapter contract, without broadly rewriting the measurement engine, dashboard, configuration, or reporting |
| DNS | Explore port controls, direct valid DNS, and resolver-mediated data tunnels, including DNS transport variants |
| Ports | Configurable destination/listening ports, optional fixed client source ports, selected-port comparisons |
| QoS | Independent packet priority markings for client-to-server and server-to-client traffic |
| Workload | Small, frequent, sustained, bidirectional FPS-like traffic |
| Result | Evidence-based comparison of timely delivery, latency tails, jitter, loss/late data, spikes, and stalls |

This repository does not implement a gaming proxy, capture/forward real games' traffic, select relay routes, switch routes automatically, or provide multipath/session migration. Traffic shaping, bandwidth control, and queue management belong to the future proxy. Scheduling configurable test traffic and validating under externally generated load are measurement functions, not shaping features.

The intended workflow does not require separately installed and manually configured WireGuard/Hysteria2 services. Supported libraries or managed bundled components may be used; the tool owns their lifecycle. Self-contained handling does not promise privilege-free operation or eliminate external inputs: real operation needs access to the server, suitable port/network permissions, and, for recursive DNS experiments, a domain/zone delegated to the measurement server and selected resolvers. The tool cannot control independent resolver policy.

## Application foundation

Confirmed on 2026-10-11 (T03, T04, T06-T09, T12/T13): one executable per OS/CPU with client/server modes, with supported deployments limited to Windows 11 clients and Linux servers on ARM64/x86-64. Target broad major Linux distributions, including CentOS, Ubuntu, Fedora, Arch, and others; exact version/support boundaries need integration review. The client hosts a local browser dashboard using embedded plain HTML/CSS/JavaScript and locally bundled assets with no runtime CDN dependency. Packaging and intended targets are design decisions, not verified integration or runtime support.

Use one shared measurement engine with separate authentic protocol adapters, platform port/priority support, and dashboard/API modules. Keep scheduling and delivery accounting consistent across modes; dashboard rendering or updates must not drive probe scheduling. This module separation does not itself adopt the optional separate coordination-path proposal S03.

Planning direction confirmed on 2026-10-11: selectively learn from existing measurement/tunnel tools and improve suitable functionality while preserving this contract. [Reference candidates](PROPOSALS.md#reference-tool-candidates) and [reuse choices](CHOICES.md#reference-tool-reuse-and-validation) track potential components and review needs. This does not select copied code, dependencies, external runtime tools or optional S07-S11 behavior; implementation remains on hold.

Save versioned YAML profiles. File loading and dashboard edits use the same validation model, and dashboard edits require an explicit save. Exact schema, parser, defaults, and storage paths remain open; this is a format decision, not runnable configuration.

Save each run's raw observations as JSONL and its summary/settings snapshot as JSON, with CSV export for analysis. Preserve enough raw evidence to recompute summaries, including incomplete outcomes. Exact result schemas and file lifecycle remain open under T14; this does not adopt optional S05's future-proxy recommendation profile.

Confirmed on 2026-10-11 (T15): traffic must be configurable, including packet/payload size, payload content, burst timing, directional rates, and run timing, with an ordered sequence of different traffic phases. Each phase supports independent directional steady/repeating-burst settings, burst duration/gap/rate, and payload size/content. Detailed boundary rules and example values are discussed in the contract below; numerical defaults have not been selected. Configurable workloads do not adopt S04's automated size/rate sweeps.

Support tool-generated pairing setup and user-provided TLS certificates. Pairing information is transferred through an already trusted channel and imported into the client dashboard; verify the owned server's identity explicitly and authenticate authorized clients. Keep the server's private identity key on the server and exclude reusable credentials/private keys from reports. User-provided certificates require explicit identity/trust validation; native protocol credentials and recursive-resolver authentication remain distinct. Detailed enrollment, storage, rotation/revocation, and certificate lifecycle remain open under T09/T14. Chart library/layout and exact Linux version targets also remain open.

## Authentic protocol coverage

| Mode | Required experiment |
| --- | --- |
| UDP | Sequence-tagged application data; delivery, loss, late arrival, reordering, duplicates |
| TCP | Actual connection and framed application exchange; connect time, steady RTT, stalls/deadline misses |
| TLS | Authenticated TLS over TCP; distinct TCP/TLS setup and established delivery |
| QUIC | Authenticated QUIC; reliable streams and unreliable datagrams as separate modes |
| WireGuard | Real peer handshake and encrypted tunnel to the owned server; inner UDP/TCP tested separately |
| Hysteria2 | Real authenticated Hysteria2 session; UDP and TCP forwarding tested separately |
| DNS | Valid DNS carrying test data, with direct/recursive transport and downlink strategies identified |

Plain UDP sent to a WireGuard/Hysteria2 port is only a UDP test. Generic QUIC is not Hysteria2. Using port 443 does not make arbitrary traffic HTTPS. Failed or unsupported modes remain visible; never silently bypass the tunnel, downgrade a protocol, or substitute another echo endpoint.

## Protocol extensibility

Confirmed on 2026-10-11: the protocol range must be easy to extend across releases. The current required protocols are initial coverage, not a closed list. This is an architecture requirement (F08), not a request to add particular protocols now.

- Keep scheduling, timing, delivery accounting, comparison rules, and common report fields in the shared engine. A new adapter maps authentic protocol exchanges to that contract rather than creating another measurement engine or changing metric definitions.
- Define an adapter contract for protocol identity/version, managed endpoint/session lifecycle, probe and receipt framing, capabilities, validated protocol-specific options, and cancellation/cleanup. Exact Go interfaces and registration mechanics remain open under T21.
- Make protocol selection, dashboard controls, profile validation, and result metadata use registered adapter descriptions and capabilities. Routine additions should need an adapter, its option descriptions, registration, and relevant checks; avoid repeated protocol-name switches throughout common modules.
- Declare real limitations: supported directions, workload modes, payload limits, ports, trust requirements, and outer-priority/observation support. Preserve requested versus effective behavior and explicit unsupported outcomes. A carrier that cannot express part of the shared contract must report that limitation rather than simulate support.
- Preserve stable adapter identifiers and versioned configuration/results. New adapters must not change the meaning of existing profiles or reports; previously saved results must remain readable or have an explicit migration path.

Treat extensibility as a first-slice design constraint. Later authorized acceptance should demonstrate that a small additional adapter can be registered and used through the shared workflow without a broad core/UI/report rewrite, while existing adapters retain their accounting and behavior. Actual new-carrier support still requires authentic protocol operation and applicable validation. This requirement does not mandate runtime plugins or hot loading; that mechanism is undecided.

## DNS experiments

| Case | What to compare |
| --- | --- |
| Ordinary-port UDP | Direct non-DNS baseline to the owned server |
| Raw UDP on port 53 | Same workload/server to investigate port-related treatment; label as non-DNS |
| Direct DNS over UDP | Valid data-bearing queries/replies on port 53 and selected alternatives |
| Direct DNS over TCP | Valid framed DNS exchange; distinguish stream behavior and setup |
| Direct encrypted DNS | DoT, DoH, and DoQ; identify actual transport/version and authenticate the endpoint |
| Resolver-mediated DNS tunnel | Query unique names in the delegated zone through a chosen recursive resolver; correlate receipt/replies at the owned authoritative server |
| Resolver-access transports | Supported UDP, TCP, DoT, DoH, and DoQ access; keep unavailable capabilities explicit |
| Downlink strategies | Independently ready server data retrieved by client queries; include queue residence, polling cadence, empty polls, batching, and backlog |

Preferential DNS treatment by the ISP is a hypothesis. The ISP might classify by port, packet contents, destination, or combinations; an RTT difference alone does not establish which mechanism it uses. The recursive resolver introduces hops but is not another owned measurement relay. Its onward transport, source port, and markings are outside the tool's control.

Validate unique run/message identities and intended server receipt. Ordinary lookup latency, cached answers, redirected traffic, and unrelated DNS services do not prove end-to-end data-tunnel performance. Exercise caching/TTL, repeated queries, interception, truncation, fallback, encoding expansion, query/record size limits, fragmentation/reassembly, rate limits, and timeouts. Record actual fallback/retry costs and transport changes. Do not assume the resolver's authoritative hop uses the client's access transport.

Report usable payload and wire overhead. Test sustained bidirectional traffic, not isolated queries alone. Server-originated data must become ready independently of client queries so measurements expose polling delay. Calibrated timing or a valid local observation method is necessary before calling this true cross-host one-way latency.

## Port selection

Configure server/destination ports per profile and optionally bind a client source port where supported. Record requested and effective local bindings and remotely observed endpoints when available. NAT may rewrite source ports; local binding success does not establish the public value. Recursive DNS access-port selection does not control the resolver's onward authoritative port.

Coordinate listeners sharing address/transport/port through sequential trials or verified multiplexing. Occupied/privileged ports, unsupported bindings, fixed-source collisions, and blocked traffic produce explicit outcomes. Do not silently substitute ports or take over existing listeners. Baseline and default ports are not finalized.

## Independent packet priority

The proposed control is DSCP, with named presets and explicit numeric values in IPv4 DS/TOS and IPv6 Traffic Class. Preserve the separate ECN bits and the transport's congestion behavior; ECN is not an arbitrary priority knob.

Set each direction independently. Compare unmarked traffic, upstream-only marking, downstream-only marking, equal markings in both directions, and asymmetric combinations. Changing the client's socket does not configure server-originated packets.

For tunneled/encapsulated modes, apply the setting to the actual outer IP traffic visible to routers; keep inner markings separate. Managed integrations must expose this capability or report the limitation. Applying only an inner marking is not a successful outer-marking test.

| Evidence | Meaning |
| --- | --- |
| Requested | Profile's selected value, independently for each direction |
| Applied | OS/component accepted, rejected, unsupported, or unknown; API success alone is not wire proof |
| Observed at sender | Actual emitted marking where capture/telemetry permits; otherwise unknown |
| Observed at receiver | Arriving marking where supported; otherwise unknown; identify the observed DNS hop |
| Measured effect | Repeated matched comparison of actual delivery quality |

Preservation does not prove routers granted priority; rewriting does not reveal where it occurred. Neither endpoint controls ISP queues or independent recursive-resolver outbound markings. Do not silently apply global network/QoS policies to make a test work.

## Integration review status

Roadmap action 3 completed as a documentation/source-review milestone on 2026-10-11. [Integration constraints](CHOICES.md#integration-constraints-review) record candidate managed APIs, outer socket ownership, port controls, directional DSCP/observation limits, DNS delegation/framing/polling, Linux toolchain/userspace boundaries, and checks required after implementation authorization. No dependency, priority backend, DNS framing default, Linux release minimum, or implementation slice was selected; no integration/platform capability is validated. Existing F01-F07 coverage and the implementation hold remain intact.

The source review distinguishes WireGuard's Linux routing mark from DSCP, identifies Windows arbitrary-DSCP privileges, and separates recursive DNS access and authoritative hops. These findings constrain later capability reporting. Detailed source evidence and unresolved alternatives belong in `CHOICES.md`; product requirements above and acceptance checks below remain authoritative.

## Timing and scheduling correctness

- Use host-local monotonic clocks for durations and wall time for labels. Different hosts' monotonic timestamps cannot be subtracted.
- Record application RTT from local send/receive boundaries. Record server receive-to-send processing duration separately; report processing-corrected RTT only when both intervals are valid. Preserve both values and flag impossible or missing timing rather than clamping it.
- Corrected RTT is not pure network delay: scheduling, socket/transport queues, and tunnel processing may remain. State timing boundaries and precision; keep submission and scheduling delays visible.
- True one-way delay is optional and needs validated calibration, drift tracking, and uncertainty. RTT/2 is only a symmetry-based estimate.
- Schedule sends independently of replies. Record planned/actual sends, blocked/skipped submissions, queue growth, and local failures. A stalled stream must not lower the reported offered rate invisibly; bound outstanding work and queues.
- Configure payload size, bidirectional/asymmetric rates, run length, warmup, deadline, and final observation grace period. No universal FPS values are selected.
- Separate setup, cold/resumed handshakes, established-session delivery, reconnect, and optional idle/resume phases. Test immediate RTT exchanges and independently generated downstream data.
- State the origin of deadlines. Local RTT deadlines can start at scheduled send time; cross-host directional deadlines need calibrated clocks or a defined valid observation method. Otherwise report directional counts/gaps/order without invented one-way timeliness.

## Delivery accounting and statistics

Use run/session/message identities and reconcile sender/receiver evidence in both directions. Distinguish planned sends, attempts, accepted submissions, local failures, skipped sends, unique arrivals, duplicates, reordering, late arrivals, missing messages, stalls, disconnects, and reconnects. Keep original observations when a later arrival changes classification.

State denominators and collection windows. An accepted submission is not proof of on-wire transmission; local scheduling failure is not network loss. Reconciled unique counts support loss estimates for unreliable delivery. Complete stream delivery does not prove zero underlying packet loss; report late delivery and stalls, and identify supplemental retransmission counters by source.

Stop new sends, then collect delayed traffic/final server evidence for the documented grace period. Keep late records outside the main window. Canceled runs, missing remote evidence, or unfinished observations are incomplete/unknown, not fabricated clean loss rates.

Report min/max, median, p95/p99, sample counts, and any additional justified metrics with percentile method and rounding. Completed-response percentiles must appear beside timeout/deadline outcomes so missing slow traffic stays visible. Small samples do not justify precise tails.

Define jitter and ordering explicitly: RTT variation and receiver inter-arrival variation are different observations. Define spike baseline, threshold, grouping, duration, peak, and associated failures. Proposed additional burst-pattern metrics remain in the suggestions backlog.

## Fair comparisons and FPS suitability

Repeat and alternate matched profiles against the same owned server. Match application payload, offered rates, deadlines, duration, warmup, and environment; record unavoidable differences in wire overhead, reliability, DNS polling, cache/path, fragmentation, and connection reuse. Avoid accidental contention between concurrent benchmark modes.

Compare idle and externally generated busy-network conditions, with direction and load recorded. Record CPU pressure, scheduling lag, interfaces, MTU assumptions, local errors, and network changes. A host timing problem must not automatically be diagnosed as ISP jitter.

FPS quality favors consistent useful delivery, low tail latency/jitter, and few spikes/stalls. Confirmed on 2026-10-11 (T18): present deadline misses first, then p95/p99 RTT and variation; show ties/inconclusive results and no combined score. Keep thresholds, minimum sample evidence, and ranking policy transparent and configurable; those details remain open. Failed, unsupported, canceled, insufficient, or incomplete profiles stay visible and cannot win.

A winner applies to the measured server/path/workload/time. It is not a universally perfect protocol or proof of improvement on the unmeasured server-to-game-server leg.

## Reproducible evidence

Retain JSONL raw observations, a JSON summary/settings snapshot, and CSV export (T08); exact schemas remain undecided. Raw records must support recomputation of summaries. Include:

- Run identity, tool/protocol versions, platforms, start/end labels, timing method, and calibration uncertainty if used.
- Requested/effective workload, phases, duration, deadline/grace/warmup policy, payload/rate, and execution order.
- Actual protocol/DNS path, endpoint validation, addresses/ports, resolver identity, relevant options, and fallbacks.
- Directional requested/applied/sender-observed/receiver-observed markings, explicit unavailable states, and inner/outer/hop observation locations.
- Per-message identity/direction, planned and actual local times, submission outcomes, server processing, classifications, and errors.
- Reconciled totals, denominators, distribution sample counts, spikes/stalls, setup/recovery outcomes, environmental limits, and incomplete evidence.

Validate intended server identity and session binding appropriately for each mode. Do not log credentials, private keys, or reusable authentication material. A machine-readable winning profile for the future proxy remains an optional backlog proposal.

## Measurement contract draft

Roadmap action 2's documentation milestone completed on 2026-10-11. T14's independent bidirectional data plus correlated receipts, carrying information in the protocol payload, T15's configurable steady/burst traffic and ordered phases, and T18's presentation order are confirmed. This section documents the conceptual contract, examples, and acceptance checks; proposed detailed mechanics and numerical examples remain explicitly open in [CHOICES.md](CHOICES.md#measurement-contract-discussion). No numerical defaults were accepted. Nothing here is a runnable profile, final wire/API schema, or implementation authorization. All authentic protocol/DNS modes remain in scope; the eventual first implementation slice is still undecided.

### Run lifecycle and identities

Proposed lifecycle: validate profile and pairing, prepare managed endpoints, authenticate/verify the intended server, record setup/capabilities, warm up, measure, stop generating new data, drain during grace, reconcile/finalize, and clean up owned resources. Keep setup/handshake and warmup samples separate from established-session statistics. Display errors at the phase where they occur. Cancellation stops new work and triggers bounded draining/cleanup; preserve a partial report with its actual observation window.

Freeze a validated, secret-free settings snapshot for each run. Edits affect a later run; saving a profile is explicit. Both endpoints agree on workload and phase definitions before sending. Each host uses its own monotonic phase anchor; do not claim synchronized phase starts or subtract the two anchors. Record actual phase durations and start uncertainty. A reconnect creates a new session/connection segment rather than concealing setup time inside steady-delivery metrics; retry policy remains open.

Proposed logical identity: schema version, run ID, session/connection-segment ID, lifecycle phase, workload-phase ID, origin direction, message kind, and per-origin sequence number. A receipt references the original data identity. Sequence numbers cannot be reused within a session; fragments, retransmissions, and duplicate receipts retain the original logical identity. Transport-specific framing and DNS fragmentation/encoding belong to later adapter design. Reject malformed, foreign-session, and unauthenticated data as appropriate to the real protocol; a matching run ID alone is not authentication.

### Independent traffic and timing boundaries

Each endpoint schedules its own data messages at the configured rate, independent of received data or receipts. The receiver returns a correlated receipt for each valid data message; receipt traffic is additional overhead and cannot replace independently ready downstream data. Record receipt sizes/counts and adapter polling/retry traffic separately from offered application payload. A protocol unable to meet a requested workload reports that limitation rather than silently lowering it.

Confirmed on 2026-10-11 (T14): use independently scheduled data in both directions plus correlated application receipts, with information carried in the real protocol payload. The logical data payload contains measurement metadata and configurable test content; the receipt payload identifies the received message and carries the receiver-local processing interval when available. Proposed metadata concepts are version, run/session/workload-phase identity, origin direction, message kind, sequence, and content length; exact names, encoding, and widths remain open. Carry these through the authentic adapter, including valid DNS encoding for DNS modes. Native transport acknowledgments alone do not establish receipt of a complete logical application message. Payload identities do not replace protocol-appropriate authentication.

Keep configured test-content bytes, encoded measurement-metadata bytes, total logical message bytes, receipt bytes, and outer transport overhead distinct. In the illustrative 128-byte workload, 128 bytes means test content, with metadata added separately; the dashboard must label that boundary clearly. Configurable content must not overwrite mandatory identities/framing. Sender-local timing records remain available for RTT recomputation; any transmitted timing values have an explicit host-local origin/units and cannot be subtracted across hosts without calibration. Do not put reusable credentials/private keys into measurement payloads or reports.

T14 explanation: client data message C1 goes to the server, which replies with a small receipt saying it received C1. The client measures elapsed time from sending C1 to receiving that receipt using its own clock. Separately, the server generates data message S1 according to its own workload schedule, and the client sends a receipt for S1. S1 is not merely the reply to C1. Both schedules continue while earlier receipts are outstanding. These application receipts are additional traffic, including over transports that already have their own acknowledgments. Missing data and missing receipts have different meanings: receiver records can show C1 arrived even if its receipt never returned. Receipt size, adapter mapping, and overhead limits remain open for integration review.

For each scheduled slot, record planned local time, submission attempt/completion, outcome, and queue/scheduling delay. Record receiver-local arrival and receipt submission times. Proposed elapsed RTT boundaries are original submission attempt to receipt arrival at the origin; also retain scheduled-slot-to-receipt duration for the scheduled-origin deadline. Submission acceptance time and duration remain separate evidence: an attempt or accepted write does not prove on-wire transmission. Receipts include the receiver's local arrival-to-receipt-submission interval for optional processing correction; invalid/missing correction stays explicit.

Use bounded outstanding work and queues; exact limits and skip/block behavior await T14/T15 decisions. Scheduling continues to account for every planned slot under backpressure. No unbounded catch-up burst or hidden reduction of offered rate. Warmup and main-phase messages have distinct phase identities; a warmup response arriving in the main phase does not enter main-phase RTT statistics.

Without calibrated cross-host clocks, report directional send/arrival counts, receiver inter-arrival gaps, ordering, and local queue durations. Local receipt RTT is round-trip evidence from that direction's origin, not a one-way delay. A DNS downstream datum can become ready at the server before any poll; record server-local ready-to-dispatch queue residence and polling costs separately, without inventing ready-to-client one-way latency. The exact receipt mapping for every adapter remains an integration question.

### Accounting and summary concepts

Maintain separate data-delivery and receipt/RTT ledgers in each origin direction. Preserve events so later arrivals can revise classifications without erasing history.

| Ledger / concept | Required meaning |
| --- | --- |
| Planned slots | Every scheduled main-phase datum, including locally failed or skipped slots |
| Submission | Attempted, accepted, locally failed, skipped, or pending/unknown; retain partial-stream-write evidence and uncertainty |
| Data arrivals | Unique accepted identities seen at the receiver; duplicate arrivals separate; identify late/out-of-window arrivals |
| Reconciled missing data | Accepted identities absent from complete receiver evidence at the observation cutoff; report missing application data, not proven on-wire packet loss |
| Receipt completion | First valid receipt at the origin; duplicates do not add samples; receiver data receipt and origin receipt completion can differ |
| Deadline outcome | On-time, completed late, unresolved at cutoff, locally failed/skipped, or unknown; a late completion remains a deadline miss |
| Ordering | Within one origin/session/phase, first-arrival sequence below the highest previously seen is reordered; duplicates separate. This is application ordering, not underlying packet ordering. |
| RTT variation | Proposed absolute differences between adjacent completed RTT samples ordered by original sequence; report pair count and gaps/missing sequences. Distinct from receiver inter-arrival variation. |

Proposed scheduled-slot miss fraction: (late completions + unresolved receipts + locally failed/skipped slots) / planned slots, only when the origin ledger is complete through the declared cutoff. Show the components, plus accepted-submission miss fraction separately. Pending/uncertain writes stay explicit; do not force them into a definitive category. If a run is canceled or evidence is unfinished, report the known counts/window and an incomplete outcome rather than presenting the intended full-run fraction.

Proposed unreliable data-missing fraction: missing accepted identities / accepted submissions, only with complete compatible sender/receiver records and observation windows; show arrivals, duplicates, and cutoff alongside it. Unmatched identities or ambiguous writes require reconciliation warnings. Without remote evidence, missing directional delivery is unknown even when local RTT outcomes are available. Eventual reliable-stream delivery never becomes a zero network-packet-loss claim.

Keep elapsed RTT and scheduled-origin completion distributions separate, with on-time/late/unresolved counts adjacent. Proposed percentile method: nearest rank on sorted completed samples, rank ceil(p * n), with sample count and unrounded values retained for recomputation. Small samples and missing slow responses limit tail conclusions. Spike baseline/threshold/grouping and stall start/end rules remain unresolved; do not produce arbitrarily defined spike scores or imply S02 is adopted.

### Profile, result, and dashboard fields

Confirmed T15 controls: an ordered list of traffic phases, each with its duration and independent steady or repeating-burst settings per direction, application payload byte length/content, intra-burst message rate, burst duration, and gap between bursts. Warmup, RTT deadline, and final grace are also configurable. Keep size/content choices reproducible; literal bytes or deterministic patterns with a recorded seed are candidate content options, not a selected generator API. Record active-message rate and whole-phase average separately for bursts; intentional gaps are not skipped probes. Exact burst boundary/rounding rules, phase-transition behavior, and deadline inheritance/overrides remain open.

Illustrative phase sequence, with no defaults selected: 20 s steady traffic with 128-byte payloads at 60 messages/sec each way; 5 s repeating bursts with 256-byte payloads, 200 ms active/800 ms gap and 120 messages/sec while active in each direction; then 10 s with no new scheduled data. Values and directional patterns are editable. Proposed accounting attributes a datum and its receipt/deadline to the originating workload phase even if they arrive in a later phase; a quiet phase can still receive outstanding traffic. Show per-phase counts/distributions and the complete sequence in reports, alongside whole-run totals. Match phase sequences when comparing profiles; aggregate statistics cannot conceal a phase-specific failure. Schedule boundaries use each host's local monotonic timeline; phase support does not establish cross-host synchronization.

Distinguish configured application payload length, total logical message length including measurement framing, and observed outer packet sizes when available. Protocol headers, encryption, segmentation, MTU, and DNS encoding can change outer sizes. A request for an exact outer packet size requires adapter capability/validation; report unsupported or observed differences rather than claiming the payload length forces that wire size. Control limits must be configurable and explicit, with invalid or infeasible combinations rejected/reported instead of silently rewritten.

Proposed profile concepts: schema/profile identity; server/pairing reference; authentic adapter and inner mode; direct/recursive DNS path and resolver/zone inputs when relevant; requested listener/destination/source ports; independent upstream/downstream outer DSCP; payload/rates; warmup/measurement/deadline/grace; requested resource limits and reconnect policy. Validate units, bounds, compatibility, and unsupported requests through the same model in the dashboard and profile loader. Do not place reusable credentials in a results snapshot.

Proposed raw-event concepts: event/schema version, run/session/phase/message identity, endpoint role, host-local time origin and monotonic offset, event kind, submission/arrival/receipt outcome, bytes, durations, error, and evidence source. Wall-clock labels are separate. Preserve units, explicit unavailable/unknown values, observation cutoffs, and remote-record completeness. Final encoding, exact field names, API operations, and version-migration policy await T14.

The JSON summary/settings snapshot must identify tool/adapter/platform versions, workload requested/observed, protocol/path, ports requested/bound/remotely observed, setup durations, phase/cancellation outcome, directional accounting, distributions/denominators, queue/scheduling limits, and environmental changes. For priority, retain requested/applied/sender-observed/receiver-observed values separately for each direction, with outer/inner and DNS hop locations. Summary metrics must link to the raw records from which they were derived. CSV export retains units, counts, missing values, and run identity; it is not S05's future-proxy configuration.

Proposed dashboard controls: import/view pairing; choose protocol/DNS mode, ports and both directional markings; add/reorder/remove traffic phases and edit their durations, payload size/content, directional rates, steady/burst timing and run/deadline/grace timing, and explicitly save workload; validate and show capability limitations; start/cancel; display lifecycle/current workload phase, progress and live provisional counters; inspect/export finalized or partial results including per-phase outcomes; compare selected matched runs. Show unsaved edits and frozen running settings distinctly. Charts cannot drive scheduling; chart library/layout and live-update cadence remain open under T13/T14. API transport/endpoints and local request protection still need selection.

### Pairing and user-provided certificate flow

Generated setup: server produces protocol-appropriate public identity/trust information and client-specific authorization setup; user transfers a versioned pairing bundle through an already trusted channel, then imports it in the local client dashboard. Preview intended server address/identity and supported modes before connecting. Validate bundle format and endpoint identity, then establish client authorization. Retain native protocol credentials; do not substitute a TLS certificate for WireGuard keys or imply raw UDP is TLS. No server private identity key belongs in the transferred bundle.

User-provided TLS setup: user supplies certificate chain/private key to the server and configures expected endpoint name plus trusted CA or explicitly selected identity mechanism on the client. Validate endpoint name/trust and report certificate errors; never bypass verification after failure. Client authorization remains a separate step. Do not export keys or reusable enrollment/authentication material in logs/reports; use non-secret references/identifiers there. Encrypted recursive-resolver trust remains independent of owned-server pairing.

Remaining decisions: exact bundle format and enrollment proof, authorization for each non-TLS mode, secret storage and file access, expiry, rotation/revocation, renewal/reload, and rollback on failed replacement. Wrong identity, malformed setup, expired credentials, or missing authorization prevents the affected session, with explicit errors. Protocol mapping and credential mechanics require later integration review; this flow is a design draft rather than a validated security implementation.

### Provisional example workload and report

Illustrative T15 profile in prose, with no numerical defaults selected: one owned server, real UDP data mode, explicitly chosen available destination port and automatic client source port, requested outer DSCP 0 in both directions; 128-byte test content excluding measurement metadata/framing, with content recorded; one steady workload phase at 60 independently generated data messages/sec per direction; 5 s warmup, 60 s measurement, 100 ms scheduled-origin receipt RTT deadline, 5 s final grace. Receipt traffic adds overhead under the confirmed T14 pattern. All workload values and phase sequences are editable. This example chooses neither a default port nor the first implementation slice; stream/tunnel/DNS modes use the same accounting concepts with their actual overhead and limits.

The 60 s main phase schedules 3,600 slots per direction, at local offsets k/60 seconds for k = 0 through 3,599. Warmup is excluded. Stop generating data at each endpoint's main-phase end, continue bounded receipt/observation work through grace, and record actual cutoffs; never assume the two local cutoff timestamps are identical.

The following is a synthetic accounting example, not observed performance or predicted success:

| Main-phase field | Client-origin data | Server-origin data |
| --- | --- | --- |
| Planned / accepted / local failed / skipped | 3,600 / 3,588 / 4 / 8 | 3,600 / 3,600 / 0 / 0 |
| Unique receiver arrivals / extra duplicates | 3,570 / 3 | 3,600 / 0 |
| Missing accepted data at cutoff | 18 / 3,588, about 0.50% | 0 / 3,600 |
| Receipts on time / completed late / unresolved | 3,500 / 40 / 48 | 3,600 / 0 / 0 |
| Scheduled-slot deadline misses | 100 / 3,600, about 2.78% (12 local + 40 late + 48 unresolved) | 0 / 3,600 |
| Accepted-submission deadline misses | 88 / 3,588, about 2.45% | 0 / 3,600 |
| Completed elapsed RTT sample count | 3,540, including 40 scheduled-deadline-late completions | 3,600 |

Assume complete reconciled ledgers through grace only for this example. Some data arrived even though its receipt did not; 18 missing data messages and 48 unresolved receipts describe different outcomes. Directional missing-data counts do not establish physical packet-loss counts. RTT values, p95/p99, variation, setup/overhead, ports/priority observations, and environment are absent from this synthetic example and must appear as unavailable rather than fabricated values. It cannot support a winner.

Confirmed T18 presentation: display scheduled-deadline misses and their causes first, then p95/p99 completed RTT and variation with counts, data delivery, scheduling limits, and actual settings; show ties/inconclusive results and no combined score. Use repeated matched measurements before conclusions. Automatic screening, repetition defaults, material-difference/evidence thresholds, and an additional confidence model remain unselected; the presentation order alone does not establish an automatic winner.

### Contract acceptance examples

These are required checks for a later authorized implementation, not tests performed now:

- A blocked stream still accounts for all planned slots and reports local backpressure/skips; its eventual delivery does not prove zero network loss.
- A received datum with a missing receipt contributes to receiver delivery but not completed RTT; duplicate data/receipts do not inflate unique/sample counts.
- Real data/receipt payloads carry their logical identities through each adapter. Edited test content preserves metadata/framing; reports separately account for content, metadata, receipts, and observable transport bytes. Invalid identity/content-length records cannot become valid delivery samples.
- A receipt at 120 ms after its scheduled slot with a 100 ms deadline is late, remains a deadline miss, and enters completed distributions; an arrival after final grace retains its event/window identity without silently rewriting the finalized cutoff.
- Warmup/foreign-session traffic cannot contaminate main-phase samples; both origins generate data without waiting for incoming probes.
- Reordering/editing traffic phases changes the saved next-run schedule; actual planned slots match the selected sequence. Late receipts retain the originating workload-phase identity, intentional burst/quiet gaps are not skipped probes, and phase summaries reconcile with whole-run counts.
- Missing remote evidence makes directional loss unknown; cancellation yields a partial report; raw replay recomputes the declared summary and denominator.
- Host wall-clock changes do not change elapsed timing; the two hosts' monotonic values never produce an uncalibrated one-way claim; invalid correction remains visible.
- Busy/unsupported ports, unavailable outer markings, invalid pairing/certificates, and cleanup failures have explicit outcomes with no silent substitutions; cleanup preserves user-owned resources.
- The synthetic report balances planned/submission and receipt counts; dashboard saving/editing does not alter the active snapshot; exports preserve unknowns and exclude secrets.

## Validation before support/performance claims

The following are acceptance checks for later authorized implementation, not completed tests:

| Area | Required evidence |
| --- | --- |
| Authentic adapters | Real session-bound exchanges for each mode; actual WireGuard/Hysteria2 operation; QUIC stream/datagram and tunnel UDP/TCP distinctions |
| Extensibility | Add/register a small additional adapter through the shared workflow; check capability/options discovery, profile/result compatibility, lifecycle cleanup, common accounting, and unchanged existing-adapter behavior |
| Basic timing | Loopback overhead floor plus real-adapter tests; wall-clock adjustments; valid processing correction; no invented cross-host delay |
| Scheduling/accounting | Stalled streams with independently planned sends; delayed arrivals after deadline/end; duplicates, reordering, malformed/foreign/replayed data; incomplete evidence and summary recomputation |
| Controlled impairment | Known delay, jitter, periodic spikes, random/burst loss, duplication, reordering, directional impairment, rate limits, restrictive MTU, and brief interruptions |
| DNS | Intended authoritative receipt; cache/interception tests; downlink queue/polling effects; framing/encoding/limits/truncation/fallback/reassembly; resolver restrictions and encrypted endpoint trust |
| Ports/priority | Actual bindings and NAT observations; collisions/privileges; directional DSCP with ECN preserved; outer packets verified where possible; unsupported/unknown states |
| Fair ranking | Repeated matched profiles, observed offered workload, justified sample sizes/uncertainty, missing samples visible, and no winners from incomplete evidence |
| Trust/lifecycle | Wrong identities/keys/certificates, tampering and replay rejected; bounded reflection/resource work; setup/probing/polling/teardown cancellation; managed-resource cleanup |
| Sustained operation | Bounded CPU/memory/outstanding queues, counter rollover, report flushing, and graceful shutdown |
| Platforms | Actual Windows 11 client and Linux server paths on ARM64/x86-64; supported OS versions, privileges/components explicitly recorded |

External fixtures such as Linux `tc netem` belong in an isolated authorized validation environment; record their location/settings and restore temporary changes. This does not add shaping to the product. Preserve user-owned services, keys, files, routes, firewall, resolver, and QoS settings.

Distinguish documented intent, compilation, loopback validation, controlled impairment, and live ISP evidence. A successful build cannot prove every protocol/marking works on every platform. Record exact versions/configurations and remaining coverage gaps.

## Technical choices

Confirmed on 2026-10-11: **Go**, a **local browser dashboard**, and the [application foundation](#application-foundation). Technical questions, recommendations, research notes, and decision states are maintained in [CHOICES.md](CHOICES.md). Remaining choices are open; implementation remains on hold.

## Primary references

- [WireGuard protocol](https://www.wireguard.com/protocol/)
- [Hysteria2 protocol](https://v2.hysteria.network/docs/developers/Protocol/) and [forwarding modes](https://v2.hysteria.network/docs/advanced/Full-Client-Config/)
- [QUIC, RFC 9000](https://www.rfc-editor.org/rfc/rfc9000.html)
- [DS field, RFC 2474](https://www.rfc-editor.org/rfc/rfc2474.html), [tunnels, RFC 2983](https://datatracker.ietf.org/doc/html/rfc2983), and [DSCP re-marking, RFC 9435](https://www.rfc-editor.org/rfc/rfc9435.html)
- [Service-class guidelines, RFC 4594](https://www.rfc-editor.org/rfc/rfc4594.html#section-4.9): recommends standard best-effort for DNS; actual ISP treatment remains experimental.
