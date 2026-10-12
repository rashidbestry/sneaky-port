# Future features and proposals

Recorded: 2026-10-10.

Use this file to track forward: upcoming features, optional ideas, their decisions, and progress. [WORK_LOG.md](WORK_LOG.md) tracks backward: changes actually made, validation, and current implemented behavior. Product details remain in [DESIGN.md](DESIGN.md).

Accepted features reflect confirmed scope. S07-S11 were accepted for future scope on 2026-10-11; S01-S06/S12 remain proposals. User explicitly authorized the agreed first slice on 2026-10-11; unrelated acceptance/plans do not expand execution scope.

Finite UDP/TCP lifecycle/accounting, bounded evidence/replay/export, authenticated reconciliation, one-way estimates and protected configuration/run/report API exist; dashboard setup/run and saved diagnostic/distribution views exist; live counters, comparisons and full acceptance remain pending. [Saved API evidence](WORK_LOG.md#2026-10-11-saved-configuration-and-report-api), [code flow](FLOWCHART.md).

## Confirmed future features

This index tracks scope and partial progress, not a release schedule. [ROADMAP.md](ROADMAP.md#first-slice-task-index) splits the former umbrella into separately checked tasks; milestones 5-18 are complete within scope. Production clock bounds, UI/live views, comparisons and full acceptance remain pending; no F item is complete.

| ID | Feature | Status | Design reference |
| --- | --- | --- | --- |
| F01 | Windows 11 clients, one Linux server across major distributions including CentOS/Ubuntu/Fedora/Arch, ARM64/x86-64 on both, managed endpoint/session lifecycle | Partial: persistent pairing, key-only SSH, finite CLI, protected configuration/run/report API and dashboard setup/run controls; four builds; saved diagnostics/distributions implemented; live counters and native ARM64/Linux minima pending | [Scope](DESIGN.md#confirmed-intent-and-scope) |
| F02 | Real TCP, UDP, TLS, and QUIC stream/datagram measurements | Partial: UDP/TCP readiness, queues and shared UDP/TCP phase/accounting pass Windows/WSL fixtures; finite CLI lifecycle implemented; TLS/QUIC later | [Protocols](DESIGN.md#authentic-protocol-coverage) |
| F03 | Tool-managed WireGuard and Hysteria2 with inner UDP/TCP test modes | Accepted; not started | [Protocols](DESIGN.md#authentic-protocol-coverage) |
| F04 | DNS port controls, direct and recursive data paths, transport/downlink experiments | Accepted; not started | [DNS](DESIGN.md#dns-experiments) |
| F05 | Server/destination ports, optional source ports, selected-port comparisons | Partial: explicit socket controls, server port policy and effective readiness bindings; matched-run comparisons/live NAT evidence pending | [Ports](DESIGN.md#port-selection) |
| F06 | Independent directional packet priority with outer-packet evidence | Partial: role-specific readiness probes requested markings; Linux DSCP/ECN readback tested, Windows baseline unknown/qWave compiles; writer-owned physical-phase transitions implemented; outer capture/native nonzero qWave gates pending | [Priority](DESIGN.md#independent-packet-priority) |
| F07 | FPS workloads, correct timing/delivery metrics, reproducible reports, evidence-based comparisons | Partial: UDP/TCP phases, deadline/cutoff/ticket accounting, bounded local observations and RTT/variation/spike/stall analysis; bounded local files/replay/exports and authenticated remote reconciliation implemented; comparisons pending | [Measurements](DESIGN.md#timing-and-scheduling-correctness) |
| F08 | Extensible protocol adapters with shared measurement, capability-driven configuration/dashboard, and compatible results | Partial registry/descriptor/factory, protected API descriptors, saved validation/replay and registry-driven dashboard options, test-only option extension; common engine/UI/report extension gate pending | [Extensibility](DESIGN.md#protocol-extensibility) |
| F09 | Authenticated run-plan coordination and maintained clock calibration for directional one-way measurements with uncertainty | Partial: trust/readiness/leases, finite lifecycle, authenticated reconciliation, live SSH captures and causal one-way intervals/replay/export implemented. Synthetic envelope gates pass; independently verified production bounds and clock-change/native/live acceptance pending | [Control and calibration](DESIGN.md#control-connection-and-clock-calibration) |
| F10 | Capability-driven dashboard traffic tweaks, including supported fragmentation and header/framing/payload size controls | Partial: payload and common dashboard editor/registered options implemented; richer fragmentation/header controls and schema/mechanisms pending ([evidence](WORK_LOG.md#2026-10-11-protocol-traffic-tweak-requirement)) | [Traffic controls](DESIGN.md#protocol-specific-traffic-controls) |

Keep implementation details, validation coverage, and completed changes in the work log. When progress begins, annotate this index with its actual state and a reference to the relevant work-log entry. Partial delivery of an item does not complete all its protocol/platform variants.

## Technical choice progress

Current scope/decisions remain in [DESIGN.md](DESIGN.md#first-slice-implementation-contract) and [CHOICES.md](CHOICES.md#first-slice-final-decisions). UDP/TCP portions of F01/F02/F05-F09, basic S08/S09 and S07/S10 gates are selected; S11/other carriers/DNS remain later. Foundation checks are partial; no full runtime gate is complete.

Detailed options/provenance belong in `CHOICES.md`; [WORK_LOG.md](ARCHIVE.md#2026-10-11-first-implementation-foundations) retains actual implementation/evidence. [ROADMAP.md](ROADMAP.md) keeps next actions. Do not promote partial features to complete without their acceptance evidence.

## Suggestions and accepted additions

S01-S06 remain proposed, with stable IDs. S07-S11 were accepted together on 2026-10-11; S08/S09 have local preflight and saved diagnostic/distribution views; verified clock quality, live counters and other additions remain pending. Acceptance alone did not authorize execution; the later first-slice instruction supplies that authorization.

### Reference review dispositions

Reviewed/answered: 2026-10-11, action 3b. User confirmed IRTT/iperf2/Crusader as design/external-validation references and accepted S07-S11. [Dispositions](ARCHIVE.md#selective-source-review-and-dispositions), [acceptance matrix](DESIGN.md#reference-comparison-acceptance-matrix), [historical review](ARCHIVE.md#2026-10-11-selective-reference-review-completed). Current partial implementation is indexed above.

| ID | Recommendation for the first-slice discussion | Decision / implementation state |
| --- | --- | --- |
| S07 | First-slice external cross-check gates: equivalent IRTT UDP, iperf2 receiver/TCP and Crusader path/load overlaps; exact builds/tolerances established before later execution. No product dependency. | Accepted future scope by user, 2026-10-11; first-slice checks specified, not implemented or executed |
| S08 | First-slice optional local scheduler preflight and evidence-based diagnostics; explicit duration/interval, no network data or OS tuning in the local preflight. | Partial: explicit CLI/dashboard local preflight, endpoint fault/local-miss counts and partial-stream/accounting evidence exist; clock source/resolution verification remains pending |
| S09 | First-slice native SVG time/ECDF views and phase tables, exact final quantiles with bounded/labeled rendering and coalesced live updates. | Partial: bounded SVG time/ECDF, exact phase/quantile/count and missing/local views exist; coalesced live counters and rendered-browser QA remain pending |
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

Task 20 milestone suggestion (still proposed): S02 sequence-based interruption metrics could quantify clusters now visible in missing/local plot markers. The replay ledger and diagnostic entry point provide inputs; first define burst boundaries, populations and treatment of late/local-unsent/partial evidence, then validate synthetic sequences. S02 is not accepted or implemented.

## S03: Separate coordination and measured traffic

2026-10-11 clarification: basic run-plan coordination and maintained one-way clock calibration are now required under F09. Tasks 12-14 now supply baseline endpoint persistence, local replay/export and authenticated post-stop transfer/reconciliation under F07/F09. Additional coordination minimization and recovery behavior remain proposed.

- [ ] Not implemented; proposed for later consideration.

Exchange test plans before probing and collect server records afterward. Keep coordination traffic minimal during measurement, and persist evidence on both endpoints when the connection fails.

Purpose: preserve enough evidence to understand failed tests while limiting interference from test management.

Before adopting additional S03 behavior: define recovery/import/retention policy and verify control-traffic minimization; baseline authenticated coordination, bounded local persistence, replay/export and post-stop reconciliation exist. Failure of the coordination path must not automatically classify every measurement protocol as unusable. Missing remote evidence must remain explicitly incomplete.

Task 14 milestone suggestion (still proposed): bounded manual authenticated recovery/import could make retained endpoint reports usable after SSH interruption. The transfer schema and replay checks now provide prerequisites. Before accepting S03 recovery, decide who can retrieve old runs, authorization expiry, input/file budgets and retention; validate stale/mismatched/partial evidence. No recovery implementation is authorized by this suggestion.

Task 15 milestone suggestion (still proposed): audit S03 coordination overhead across explicit clock cadences. Separate calibration byte/cadence/serialization/send-call records now allow measuring its workload and SSH blockage alongside RTT. Before accepting minimization, choose matched trial profiles/repetitions and predeclared comparison bands; preserve required clock coverage/expiry and validate actual SSH/network overhead separately. Do not silently reduce sampling or claim that application bytes measure outer traffic.

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

Task 16 milestone suggestion (still proposed): one-way results now retain uncertainty, expiry, missing/inconsistent-clock reasons and explicit detail omissions alongside unchanged local RTT/delivery. These inputs can support descriptive evidence labels such as “clock bounds unverified” or “missing receiver records,” without a combined score or automatic winner. User requested an explanation on 2026-10-11; acceptance remains open. Before later implementation, decide which labels apply per metric, how repeated-run evidence is described, and verify that poor timely-delivery results remain visible beside measurement limitations.

## Reference-tool candidates

Recorded: 2026-10-11 during action 3a. This initial inventory captures useful ideas, not a completed source audit or proven integration. The later [action 3b decisions](#reference-review-dispositions) and [source review](ARCHIVE.md#selective-source-review-and-dispositions) provide current dispositions. Prefer improvements that preserve our shared measurement contract across authentic adapters. Existing requirements below remain F01-F07; they are not claims of implementation. [Planning evidence](ARCHIVE.md#2026-10-11-reference-tool-planning-recorded).

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
| [StarTrinity Network Tester](https://startrinity.com/VoIP/NetworkTester/NetworkTester.aspx), [CST custom-server guide](https://startrinity.com/InternetQuality/CST/UseCases/MonitoringWithCustomServer.aspx), [SIP Tester tutorial](https://startrinity.com/VoIP/SipTester/SipTesterTutorial.aspx) | Bidirectional UDP size/rate controls, jitter distributions/history, simultaneous upload/download monitoring, quality thresholds, worker-delay diagnostics | F02/F07, S08/S09/S11: documentation/design references; external cross-check candidate only after source/build/license/measurement review. Proposed S12 adds classified degradation episodes. SIP/RTP/MOS, vendor servers, Center/MySQL and global QoS changes are not adopted. [Review evidence](ARCHIVE.md#2026-10-11-startrinity-reference-review-recorded). |

### S07: Independent reference-tool cross-checks

- [ ] Accepted future scope by user, 2026-10-11; implementation not started. [Decision/review evidence](ARCHIVE.md#2026-10-11-selective-reference-review-completed).

Define a repeatable validation comparison against IRTT, iperf2, sockperf, Crusader or Flent where their measurements overlap ours. Retain tool/version, settings, raw evidence, endpoint/path, timing boundaries and observed workload; explain discrepancies rather than assuming either tool is ground truth.

Purpose: independently check shared-engine timing, delivery accounting and report calculations for F02/F07. This extends validation evidence, not the product's normal managed endpoint workflow.

Before execution: use the settled [reference matrix](DESIGN.md#reference-comparison-acceptance-matrix), select compatible builds and establish empirical tolerances; match payload including metadata/receipts, actual offered rates, direction, warmup, cutoff/grace and load. Compare only equivalent quantities. First-slice UDP/TCP cross-checks are included in authorized [task 33](ROADMAP.md#queued-validation-tasks), with evidence pending; other-carrier checks retain future scope. No external CLI becomes a mandatory runtime dependency.

### S08: Timing preflight and diagnostic report

- [ ] Accepted future scope by user, 2026-10-11; partial local preflight only, full diagnostics pending. [Implementation evidence](ARCHIVE.md#2026-10-11-first-implementation-foundations).

Offer an optional local preflight that measures timer wake-up error, scheduling lag and submission overhead, plus a diagnostic view during runs. Keep these observations separate from network RTT, receiver processing and adapter queues; identify CPU pressure and unsupported observations.

Purpose: extend F07's already required timing records with an explainable diagnostic workflow inspired by IRTT. This is not cross-host clock calibration and cannot establish one-way delay.

Remaining work: integrate the contract's explicit preflight/window limits and advice with run diagnostics, recording changes between preflight and probing. Do not change OS priority/settings, subtract a measured floor from all RTTs or dismiss network failures as host problems.

### S09: Distribution and phase comparison views

- [ ] Accepted future scope by user, 2026-10-11; implementation not started. [Decision/review evidence](ARCHIVE.md#2026-10-11-selective-reference-review-completed).

Add optional histograms/CDFs and matched-run overlays alongside phase timelines for completed RTT, scheduling error, deadline outcomes and delivery interruptions. Show direction, workload phase, units, sample counts, missing/late outcomes and measurement boundaries; allow inspection of the underlying records.

Purpose: extend F07's required live summaries and comparison controls using ideas from iperf2, Flent, Crusader and Graphite Meter. Those UI/report paths remain pending. Complements S02/S06 without adopting their additional metrics or confidence policy.

First-slice T13/T14/T18 native SVG time/ECDF views, exact quantiles and bounded updates are settled in the [contract](DESIGN.md#first-slice-implementation-contract); dashboard/report integration remains pending. Additional histograms/overlays need their own defined limits. Keep deadline misses first, no combined score, and explicit ties/inconclusive outcomes. Completed-sample distributions must not hide missing slow traffic. A phase-aligned display does not establish synchronized host clocks; plots never drive scheduling.

### S10: Controlled contention scenario catalogue

- [ ] Accepted future scope by user, 2026-10-11; implementation not started. [Decision/review evidence](ARCHIVE.md#2026-10-11-selective-reference-review-completed).

Document reproducible validation scenarios: idle, external upload load, external download load, simultaneous load, bulk TCP beside FPS traffic, and bounded non-responsive UDP contention. Add isolated queue-policy/DSCP-classification fixtures when relevant, with actual load and topology recorded separately from the measurement profile.

Purpose: make F07's required externally busy-network validation reproducible, drawing on Flent and bufferbloat-lab; runtime evidence remains pending. This is a validation catalogue, not in-product background-load orchestration, bandwidth control or router configuration.

Before execution: record T20 fixture ownership, isolation, resource budgets, restoration and matched repetitions; account for load-generator CPU and path bypass. First-slice external-contention gates are included in the authorized validation scope, with evidence pending. Preserve user-owned services/settings; temporary changes outside owned isolated fixtures need separate authorization. Local CAKE class observations do not prove ISP prioritization. Router queue telemetry is optional local evidence, not an inferred measurement of inaccessible ISP queues or per-game-packet delay.

### S11: Continuous responsiveness monitoring

- [ ] Accepted future scope by user, 2026-10-11; implementation not started. [Decision/review evidence](ARCHIVE.md#2026-10-11-selective-reference-review-completed).

Offer an explicitly started/stopped low-rate session with bounded rolling windows for RTT, variation, deadline outcomes and delivery gaps, inspired by Crusader and IRTT. Keep it separate from finite matched comparison runs and export window identity/settings when retained.

Purpose: reveal intermittent degradation over longer periods without making an old short comparison a current guarantee.

Before implementation: settle T14 resource/retention limits, window denominators, reconnect handling, traffic/receipt cost, and protocol eligibility. Incomplete windows remain visible. Monitoring does not select winners, switch routes, forward games or silently test in the background; richer burst/phase behavior remains under the existing workload choices.

### S12: Classified degradation episodes and observed availability

- [ ] Not implemented; proposed for later consideration, 2026-10-11. [Review evidence](ARCHIVE.md#2026-10-11-startrinity-reference-review-recorded).

Extend accepted S11 monitoring with configurable, versioned criteria for no-arrival gaps, deadline misses, reconciled loss and latency/variation degradation. Record episode start/end, duration, direction, triggering evidence and recovery; display counts, longest episode and the fraction of eligible observed time meeting the chosen criteria. Keep no-arrival episodes separate from degraded-but-delivering periods and expose unknown/unobserved time.

Purpose: turn a long chart into an explainable event record, inspired by StarTrinity CST's documented downtime thresholds. This is measured availability for a specific path/protocol/workload/window, not proof that the whole Internet was down. It overlaps S02's burst/interruption ideas without accepting S02 or replacing raw delivery accounting.

Before adoption: settle T24's jitter definitions, window/denominator, minimum evidence, transition/debounce/hysteresis, quiet-phase handling and recovery rules. Pauses, intentional burst gaps, canceled runs, local unsent work, control/calibration failures and missing receiver records must not silently become network outages. Distinguish stream stalls from packet loss. No StarTrinity numerical thresholds, VoIP MOS grades, automatic route changes or external notification service are selected. Candidate status does not expand accepted S11 behavior by implication.

## Maintaining this file

Keep stable F/S identifiers with purpose, explicit status and open dependencies. Proposed means undecided; accepted means confirmed scope, not execution authorization. In-progress/partial/completed states need work-log evidence; completed requires applicable implementation/checks. Retain deferred/rejected decisions with date/reason.

Update affected `DESIGN.md` requirements, this forward status and backward `WORK_LOG.md` evidence when scope or implementation changes. Preserve completed identifiers and coverage gaps. Keep entries concise, link canonical details, and log significant compaction; do not invent acceptance, release dates or priority.
