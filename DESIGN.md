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

Save versioned YAML profiles. File loading and dashboard edits use the same validation model, and dashboard edits require an explicit save. Exact schema, parser, defaults, and storage paths remain open; this is a format decision, not runnable configuration.

Save each run's raw observations as JSONL and its summary/settings snapshot as JSON, with CSV export for analysis. Preserve enough raw evidence to recompute summaries, including incomplete outcomes. Exact result schemas and file lifecycle remain open under T14; this does not adopt optional S05's future-proxy recommendation profile.

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

FPS quality favors consistent useful delivery, low tail latency/jitter, and few spikes/stalls. The proposed evaluation considers configurable timeliness criteria first, then latency/stability; a numerical score or weighting is not finalized. Keep thresholds, minimum sample evidence, and ranking policy transparent and configurable. Ties/inconclusive outcomes are valid. Failed, unsupported, canceled, insufficient, or incomplete profiles stay visible and cannot win.

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

## Validation before support/performance claims

The following are acceptance checks for later authorized implementation, not completed tests:

| Area | Required evidence |
| --- | --- |
| Authentic adapters | Real session-bound exchanges for each mode; actual WireGuard/Hysteria2 operation; QUIC stream/datagram and tunnel UDP/TCP distinctions |
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
