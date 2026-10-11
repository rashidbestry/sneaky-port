# Future features and proposals

Recorded: 2026-10-10.

Use this file to track forward: upcoming features, optional ideas, their decisions, and progress. [WORK_LOG.md](WORK_LOG.md) tracks backward: changes actually made, validation, and current implemented behavior. Product details remain in [DESIGN.md](DESIGN.md).

Accepted features reflect confirmed scope. S07-S11 were accepted for future scope on 2026-10-11; S01-S06/S12 remain proposals. User explicitly authorized the agreed first slice on 2026-10-11; unrelated acceptance/plans do not expand execution scope.

First-slice foundations and authenticated UDP/TCP readiness now exist, with measurement workflow/acceptance pending. [Readiness evidence](WORK_LOG.md#2026-10-11-ssh-pairing-and-readiness), [code flow](FLOWCHART.md).

## Confirmed future features

This index tracks scope and partial progress, not a release schedule. Next actions are in [ROADMAP.md](ROADMAP.md); no F item is complete.

| ID | Feature | Status | Design reference |
| --- | --- | --- | --- |
| F01 | Windows 11 clients, one Linux server across major distributions including CentOS/Ubuntu/Fedora/Arch, ARM64/x86-64 on both, managed endpoint/session lifecycle | Partial: persistent pairing, key-only embedded SSH, bounded readiness/lease/revocation and four builds; full measurement workflow, native ARM64/Linux minima pending | [Scope](DESIGN.md#confirmed-intent-and-scope) |
| F02 | Real TCP, UDP, TLS, and QUIC stream/datagram measurements | Partial: authentic UDP/TCP sockets/framing/bind/readiness pass Windows/WSL fixtures; independent measurement engine pending; TLS/QUIC later | [Protocols](DESIGN.md#authentic-protocol-coverage) |
| F03 | Tool-managed WireGuard and Hysteria2 with inner UDP/TCP test modes | Accepted; not started | [Protocols](DESIGN.md#authentic-protocol-coverage) |
| F04 | DNS port controls, direct and recursive data paths, transport/downlink experiments | Accepted; not started | [DNS](DESIGN.md#dns-experiments) |
| F05 | Server/destination ports, optional source ports, selected-port comparisons | Partial: explicit socket controls, server port policy and effective readiness bindings; matched-run comparisons/live NAT evidence pending | [Ports](DESIGN.md#port-selection) |
| F06 | Independent directional packet priority with outer-packet evidence | Partial: role-specific readiness probes requested markings; Linux DSCP/ECN readback tested, Windows baseline unknown/qWave compiles; directional engine, outer capture/native nonzero qWave gates pending | [Priority](DESIGN.md#independent-packet-priority) |
| F07 | FPS workloads, correct timing/delivery metrics, reproducible reports, evidence-based comparisons | Partial strict schema/hash, rational scheduling, local ledger/cutoff/percentile fixtures; network engine/storage/comparison/stall paths pending | [Measurements](DESIGN.md#timing-and-scheduling-correctness) |
| F08 | Extensible protocol adapters with shared measurement, capability-driven configuration/dashboard, and compatible results | Partial registry/descriptor/factory and test-only option extension; engine/UI/report extension gate pending | [Extensibility](DESIGN.md#protocol-extensibility) |
| F09 | Authenticated run-plan coordination and maintained clock calibration for directional one-way measurements with uncertainty | Partial: persistent trust, SSH plan agreement/authenticated readiness and leases, synthetic clock envelopes; start/calibration/report channels, live sampler and verified clock bounds pending | [Control and calibration](DESIGN.md#control-connection-and-clock-calibration) |

Keep implementation details, validation coverage, and completed changes in the work log. When progress begins, annotate this index with its actual state and a reference to the relevant work-log entry. Partial delivery of an item does not complete all its protocol/platform variants.

## Technical choice progress

Current scope/decisions remain in [DESIGN.md](DESIGN.md#first-slice-implementation-contract) and [CHOICES.md](CHOICES.md#first-slice-final-decisions). UDP/TCP portions of F01/F02/F05-F09, basic S08/S09 and S07/S10 gates are selected; S11/other carriers/DNS remain later. Foundation checks are partial; no full runtime gate is complete.

Detailed options/provenance belong in `CHOICES.md`; [WORK_LOG.md](WORK_LOG.md#2026-10-11-first-implementation-foundations) retains actual implementation/evidence. [ROADMAP.md](ROADMAP.md) keeps next actions. Do not promote partial features to complete without their acceptance evidence.

## Suggestions and accepted additions

S01-S06 remain proposed, with stable IDs. S07-S11 were accepted together on 2026-10-11; S08 has a partial local preflight, other additions remain pending. Acceptance alone did not authorize execution; the later first-slice instruction supplies that authorization.

### Reference review dispositions

Reviewed/answered: 2026-10-11, action 3b. User confirmed IRTT/iperf2/Crusader as design/external-validation references and accepted S07-S11. [Dispositions](CHOICES.md#selective-source-review-and-dispositions), [acceptance matrix](DESIGN.md#reference-comparison-acceptance-matrix), [historical review](WORK_LOG.md#2026-10-11-selective-reference-review-completed). Current partial implementation is indexed above.

| ID | Recommendation for the first-slice discussion | Decision / implementation state |
| --- | --- | --- |
| S07 | First-slice external cross-check gates: equivalent IRTT UDP, iperf2 receiver/TCP and Crusader path/load overlaps; exact builds/tolerances established before later execution. No product dependency. | Accepted future scope by user, 2026-10-11; first-slice checks specified, not implemented or executed |
| S08 | First-slice optional local scheduler preflight and evidence-based diagnostics; explicit duration/interval, no network data or OS tuning in the local preflight. | Partial: local CLI preflight implemented; dashboard/in-run diagnostics and clock-quality evidence pending |
| S09 | First-slice native SVG time/ECDF views and phase tables, exact final quantiles with bounded/labeled rendering and coalesced live updates. | Accepted future scope by user, 2026-10-11; basic slice contract settled, not implemented |
| S10 | First-slice external idle/upload/download/both-load validation gates; fixtures remain outside product, with resource/isolation/restoration and matched-workload evidence. | Accepted future scope by user, 2026-10-11; checks specified, no fixtures executed |
| S11 | Explicit continuous monitoring retained for a later slice; retention/windows/reconnect/eligibility remain future choices. | Accepted future scope by user, 2026-10-11; outside first slice, not implemented |
| S01-S06 | Keep existing proposals open; this review does not accept screening, new burst metrics, a separate coordination path, sweeps, proxy exports or a confidence model. | Proposed; no state change |

Reference versions/licenses/native builds and empirical comparison bands remain open under T19/T20. Basic S08/S09 and S07/S10 gates are settled in the first-slice contract; S11 remains later. Current code pins are separate from benchmark pins, and partial foundations do not fulfill runtime acceptance.

## S01: Staged experiment plan

- [ ] Not implemented; proposed for later consideration.

First check protocol reachability, then compare selected ports and directional priority settings, then run longer confirmation tests on promising combinations. Keep every attempted configuration and failure visible.

Purpose: make the large protocol/DNS/port/priority matrix manageable without selecting a winner from a short trial.

Before adoption: define screening rules, budgets, repeated confirmation trials, and how inconclusive or initially rejected cases can be revisited. Record which combinations were actually tested; screening must not imply exhaustive coverage.

## S02: Burst-loss and consecutive-deadline-miss metrics

- [ ] Not implemented; proposed for later consideration.

Report consecutive failures, delivery-interruption duration, event frequency, and spacing between failure bursts, alongside the overall loss/deadline-miss percentage.

Purpose: expose clustered interruptions that averages and total loss percentages can hide. Loss distribution matters to real-time delivery; [RFC 3357](https://www.rfc-editor.org/rfc/rfc3357.html) describes loss-pattern metrics.

Before adoption: define burst boundaries and denominators. Distinguish reconciled loss, late delivery, missed transaction deadlines, and local unsent probes. Stream deadline failures must not be relabeled as proven network packet loss.

## S03: Separate coordination and measured traffic

2026-10-11 clarification: basic run-plan coordination and maintained one-way clock calibration are now required under F09. This proposal's additional coordination minimization, endpoint persistence, reconciliation and recovery design remains proposed; do not treat basic coordination as awaiting S03 adoption.

- [ ] Not implemented; proposed for later consideration.

Exchange test plans before probing and collect server records afterward. Keep coordination traffic minimal during measurement, and persist evidence on both endpoints when the connection fails.

Purpose: preserve enough evidence to understand failed tests while limiting interference from test management.

Before adoption: choose authenticated coordination, bounded local persistence, reconciliation, and recovery behavior. Failure of the coordination path must not automatically classify every measurement protocol as unusable. Missing remote evidence must remain explicitly incomplete.

## S04: Packet-size and packet-rate sweeps

- [ ] Not implemented; proposed for later consideration.

Test a range of application payload sizes and offered packet rates to identify where each mode starts degrading. Include asymmetric bidirectional workloads and DNS polling/encoding costs.

Purpose: establish a useful operating range rather than only a favorable latency at one light workload. In particular, isolated small DNS exchanges do not establish sustained tunnel suitability.

Before adoption: define matched profiles, execution order, repetition, and stopping criteria. Record payload, actual offered rate, wire overhead, scheduling limits, and deadline outcomes. These are measurement workloads, not product traffic shaping.

## S05: Exportable recommendation profile

- [ ] Not implemented; proposed for later consideration.

Export the recommended protocol, DNS mode, ports, independent directional markings, workload, and supporting measurements in a machine-readable result. Include date, conditions, confidence, and a schema/version identifier.

Purpose: let the separate future gaming proxy consume the findings without embedding that proxy in this repository.

Before adoption: decide the format and compatibility contract. Preserve requested versus applied/observed settings, ties, and inconclusive outcomes. Exclude credentials/private keys, and avoid treating an old recommendation as a current guarantee. Exporting does not automatically apply settings or switch routes.

## S06: Separate connection quality from measurement confidence

- [ ] Not implemented; proposed for later consideration.

Present quality metrics and evidence confidence separately. Confidence should account for sample count, repeated-run variation, local scheduling errors, missing server records, and unobservable settings.

Purpose: prevent an apparently excellent connection from winning on weak or incomplete evidence, while avoiding classification of a measurement limitation as a network problem.

Before adoption: choose explainable confidence labels or uncertainty measures. Keep actual poor-delivery outcomes distinct from lack of evidence. Unknown marking observations limit claims about marking preservation; they do not automatically invalidate independently sound delivery measurements.

## Reference-tool candidates

Recorded: 2026-10-11 during action 3a. This initial inventory captures useful ideas, not a completed source audit or proven integration. The later [action 3b decisions](#reference-review-dispositions) and [source review](CHOICES.md#selective-source-review-and-dispositions) provide current dispositions. Prefer improvements that preserve our shared measurement contract across authentic adapters. Existing requirements below remain F01-F07; they are not claims of implementation. [Planning evidence](WORK_LOG.md#2026-10-11-reference-tool-planning-recorded).

| Reference | Components to evaluate | Existing scope and improvement to investigate |
| --- | --- | --- |
| [IRTT](https://github.com/heistp/irtt) | Periodic UDP probes, timer error, send-call/server processing timing, late/duplicate and directional-loss evidence, JSON records | F02/F07, T14: retain monotonic local timing and independent bidirectional data plus receipts; investigate timing diagnostics without replacing downstream generation with echoes or assuming calibrated one-way delay. |
| [iperf2](https://iperf2.sourceforge.io/iperf-manpage.html) | Periodic bursts, payload/rate controls, TCP bounceback, latency histograms, bidirectional tests, port binding and ToS controls | F02/F05-F07, T14/T15/T17: refine already confirmed configurable phases, payload-versus-wire size accounting and independently configured directional markings; verify actual outer packets and ECN preservation. |
| [Crusader](https://github.com/Zoxc/crusader) | Windows client/server workflow, idle/upload/download/bidirectional stages, latency/loss charts and continuous monitoring | F01/F07, T13/T15: consider editable scenario presentation and S09/S11; keep native GUI dependencies optional and preserve the confirmed embedded browser dashboard. |
| [Flent](https://flent.org/tests.html) | Repeated comparisons, RRUL contention scenarios and distribution plots | F07, T10/T18: investigate matched validation scenarios under S07/S10; external background load is distinct from the FPS workload and from product shaping. |
| [Graphite Meter](https://github.com/zR-JB/graphite-meter) | Receiver-side accounting, explicit partial evidence, HTTP/WebTransport paths, stage charts | F02/F07, T13/T14: investigate S09 presentation and missing-evidence handling; browser transports cannot silently substitute for raw UDP, QUIC or other authentic adapters. |
| [sockperf](https://github.com/mellanox/sockperf) | TCP/UDP latency-under-load and distribution measurements | F02/F07: potential S07 cross-check, with timing boundaries and platform limitations documented. |
| [dnstt](https://www.bamsoftware.com/software/dnstt/), [iodine](https://github.com/yarrick/iodine) | DNS framing, encoding, resolver limits, fragmentation, polling and sustained tunnel behavior | F04/F07, T16: inspect algorithms and constraints; neither supplies our complete measurement contract. Distinguish reliable TCP tunneling from datagram delivery, record polling/retry overhead, and prevent raw-UDP fallback from being labeled DNS. |
| [bufferbloat-lab](https://github.com/filip-lebiecki/bufferbloat-lab) | Isolated bottleneck fixtures, matched queue comparisons and live router telemetry | F07 and S10: validation/reference material only; CAKE tuning, shaping, SSH router control and queue management are outside this product's scope. |
| [StarTrinity Network Tester](https://startrinity.com/VoIP/NetworkTester/NetworkTester.aspx), [CST custom-server guide](https://startrinity.com/InternetQuality/CST/UseCases/MonitoringWithCustomServer.aspx), [SIP Tester tutorial](https://startrinity.com/VoIP/SipTester/SipTesterTutorial.aspx) | Bidirectional UDP size/rate controls, jitter distributions/history, simultaneous upload/download monitoring, quality thresholds, worker-delay diagnostics | F02/F07, S08/S09/S11: documentation/design references; external cross-check candidate only after source/build/license/measurement review. Proposed S12 adds classified degradation episodes. SIP/RTP/MOS, vendor servers, Center/MySQL and global QoS changes are not adopted. [Review evidence](WORK_LOG.md#2026-10-11-startrinity-reference-review-recorded). |

### S07: Independent reference-tool cross-checks

- [ ] Accepted future scope by user, 2026-10-11; implementation not started. [Decision/review evidence](WORK_LOG.md#2026-10-11-selective-reference-review-completed).

Define a repeatable validation comparison against IRTT, iperf2, sockperf, Crusader or Flent where their measurements overlap ours. Retain tool/version, settings, raw evidence, endpoint/path, timing boundaries and observed workload; explain discrepancies rather than assuming either tool is ground truth.

Purpose: independently check shared-engine timing, delivery accounting and report calculations for F02/F07. This extends validation evidence, not the product's normal managed endpoint workflow.

Before implementation: settle T20's reference matrix, compatible builds and tolerances; match payload including metadata/receipts, actual offered rates, direction, warmup, cutoff/grace and load. Compare only equivalent quantities. A UDP reference does not validate TLS/QUIC/WireGuard/Hysteria2/DNS behavior. External benchmark execution requires later authorization; no external CLI becomes a mandatory runtime dependency by this proposal.

### S08: Timing preflight and diagnostic report

- [ ] Accepted future scope by user, 2026-10-11; partial local preflight only, full diagnostics pending. [Implementation evidence](WORK_LOG.md#2026-10-11-first-implementation-foundations).

Offer an optional local preflight that measures timer wake-up error, scheduling lag and submission overhead, plus a diagnostic view during runs. Keep these observations separate from network RTT, receiver processing and adapter queues; identify CPU pressure and unsupported observations.

Purpose: extend F07's already required timing records with an explainable diagnostic workflow inspired by IRTT. This is not cross-host clock calibration and cannot establish one-way delay.

Remaining work: integrate the contract's explicit preflight/window limits and advice with run diagnostics, recording changes between preflight and probing. Do not change OS priority/settings, subtract a measured floor from all RTTs or dismiss network failures as host problems.

### S09: Distribution and phase comparison views

- [ ] Accepted future scope by user, 2026-10-11; implementation not started. [Decision/review evidence](WORK_LOG.md#2026-10-11-selective-reference-review-completed).

Add optional histograms/CDFs and matched-run overlays alongside phase timelines for completed RTT, scheduling error, deadline outcomes and delivery interruptions. Show direction, workload phase, units, sample counts, missing/late outcomes and measurement boundaries; allow inspection of the underlying records.

Purpose: extend F07's existing live summaries and comparison controls using ideas from iperf2, Flent, Crusader and Graphite Meter. Complements S02/S06 without adopting their additional metrics or confidence policy.

Before implementation: settle T13/T14/T18 chart layout, binning/percentile methods and bounded updates. Keep deadline misses first, no combined score, and explicit ties/inconclusive outcomes. Completed-sample distributions must not hide missing slow traffic. A phase-aligned display does not establish synchronized host clocks; plots never drive scheduling.

### S10: Controlled contention scenario catalogue

- [ ] Accepted future scope by user, 2026-10-11; implementation not started. [Decision/review evidence](WORK_LOG.md#2026-10-11-selective-reference-review-completed).

Document reproducible validation scenarios: idle, external upload load, external download load, simultaneous load, bulk TCP beside FPS traffic, and bounded non-responsive UDP contention. Add isolated queue-policy/DSCP-classification fixtures when relevant, with actual load and topology recorded separately from the measurement profile.

Purpose: make F07's existing externally busy-network validation reproducible, drawing on Flent and bufferbloat-lab. This is a validation catalogue, not in-product background-load orchestration, bandwidth control or router configuration.

Before implementation: settle T20 fixture ownership, isolation, resource budgets, restoration and matched repetitions; account for load-generator CPU and path bypass. Local CAKE class observations do not prove ISP prioritization. Router queue telemetry is optional local evidence, not an inferred measurement of inaccessible ISP queues or per-game-packet delay. Execution and temporary network changes require later authorization.

### S11: Continuous responsiveness monitoring

- [ ] Accepted future scope by user, 2026-10-11; implementation not started. [Decision/review evidence](WORK_LOG.md#2026-10-11-selective-reference-review-completed).

Offer an explicitly started/stopped low-rate session with bounded rolling windows for RTT, variation, deadline outcomes and delivery gaps, inspired by Crusader and IRTT. Keep it separate from finite matched comparison runs and export window identity/settings when retained.

Purpose: reveal intermittent degradation over longer periods without making an old short comparison a current guarantee.

Before implementation: settle T14 resource/retention limits, window denominators, reconnect handling, traffic/receipt cost, and protocol eligibility. Incomplete windows remain visible. Monitoring does not select winners, switch routes, forward games or silently test in the background; richer burst/phase behavior remains under the existing workload choices.

### S12: Classified degradation episodes and observed availability

- [ ] Not implemented; proposed for later consideration, 2026-10-11. [Review evidence](WORK_LOG.md#2026-10-11-startrinity-reference-review-recorded).

Extend accepted S11 monitoring with configurable, versioned criteria for no-arrival gaps, deadline misses, reconciled loss and latency/variation degradation. Record episode start/end, duration, direction, triggering evidence and recovery; display counts, longest episode and the fraction of eligible observed time meeting the chosen criteria. Keep no-arrival episodes separate from degraded-but-delivering periods and expose unknown/unobserved time.

Purpose: turn a long chart into an explainable event record, inspired by StarTrinity CST's documented downtime thresholds. This is measured availability for a specific path/protocol/workload/window, not proof that the whole Internet was down. It overlaps S02's burst/interruption ideas without accepting S02 or replacing raw delivery accounting.

Before adoption: settle T24's jitter definitions, window/denominator, minimum evidence, transition/debounce/hysteresis, quiet-phase handling and recovery rules. Pauses, intentional burst gaps, canceled runs, local unsent work, control/calibration failures and missing receiver records must not silently become network outages. Distinguish stream stalls from packet loss. No StarTrinity numerical thresholds, VoIP MOS grades, automatic route changes or external notification service are selected. Candidate status does not expand accepted S11 behavior by implication.

## Maintaining this file

Keep stable F/S identifiers with purpose, explicit status and open dependencies. Proposed means undecided; accepted means confirmed scope, not execution authorization. In-progress/partial/completed states need work-log evidence; completed requires applicable implementation/checks. Retain deferred/rejected decisions with date/reason.

Update affected `DESIGN.md` requirements, this forward status and backward `WORK_LOG.md` evidence when scope or implementation changes. Preserve completed identifiers and coverage gaps. Keep entries concise, link canonical details, and log significant compaction; do not invent acceptance, release dates or priority.
