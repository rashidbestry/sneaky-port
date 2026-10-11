# Future features and proposals

Recorded: 2026-10-10.

Use this file to track forward: upcoming features, optional ideas, their decisions, and progress. [WORK_LOG.md](WORK_LOG.md) tracks backward: changes actually made, validation, and current implemented behavior. Product details remain in [DESIGN.md](DESIGN.md).

Accepted features below reflect the already confirmed scope. Optional suggestions remain unapproved. Neither list authorizes implementation; execution is still on hold until the user requests it.

The project is still at the documentation stage. Some suggestions extend correctness rules already described in [DESIGN.md](DESIGN.md).

## Confirmed future features

This index tracks scope, not a release schedule or implementation order. The next few actions are in [ROADMAP.md](ROADMAP.md). All items currently have no application implementation.

| ID | Feature | Status | Design reference |
| --- | --- | --- | --- |
| F01 | Windows 11 clients, one Linux server across major distributions including CentOS/Ubuntu/Fedora/Arch, ARM64/x86-64 on both, managed endpoint/session lifecycle | Accepted; not started; exact Linux version limits pending T12 | [Scope](DESIGN.md#confirmed-intent-and-scope) |
| F02 | Real TCP, UDP, TLS, and QUIC stream/datagram measurements | Accepted; not started | [Protocols](DESIGN.md#authentic-protocol-coverage) |
| F03 | Tool-managed WireGuard and Hysteria2 with inner UDP/TCP test modes | Accepted; not started | [Protocols](DESIGN.md#authentic-protocol-coverage) |
| F04 | DNS port controls, direct and recursive data paths, transport/downlink experiments | Accepted; not started | [DNS](DESIGN.md#dns-experiments) |
| F05 | Server/destination ports, optional source ports, selected-port comparisons | Accepted; not started | [Ports](DESIGN.md#port-selection) |
| F06 | Independent directional packet priority with outer-packet evidence | Accepted; not started | [Priority](DESIGN.md#independent-packet-priority) |
| F07 | FPS workloads, correct timing/delivery metrics, reproducible reports, evidence-based comparisons | Accepted; implementation not started; conceptual contract documented ([evidence](WORK_LOG.md#2026-10-11-0624-utc0300-measurement-pattern-confirmed-and-contract-milestone-completed)) | [Measurements](DESIGN.md#timing-and-scheduling-correctness) |
| F08 | Extensible protocol adapters with shared measurement, capability-driven configuration/dashboard, and compatible results | Accepted; implementation not started; requirement documented ([evidence](WORK_LOG.md#2026-10-11-protocol-extensibility-requirement-recorded)) | [Extensibility](DESIGN.md#protocol-extensibility) |

Keep implementation details, validation coverage, and completed changes in the work log. When progress begins, annotate this index with its actual state and a reference to the relevant work-log entry. Partial delivery of an item does not complete all its protocol/platform variants.

## Technical choice progress

Technical questions and answers are maintained in [CHOICES.md](CHOICES.md); feature progress remains here. Decisions there do not mark F01-F07 implemented or adopt S01-S06. Decision history is recorded in [WORK_LOG.md](WORK_LOG.md).

2026-10-11: foundation discussion started for T03, T04/T13, and T12; user answers are pending. T01-T02 remain the only confirmed choices. See [work-log evidence](WORK_LOG.md#2026-10-11-0548-utc0300-foundation-discussion-started). F01-F07 remain accepted/not started; S01-S06 remain proposed.

2026-10-11 update: user confirmed T03-T04 and T13's plain frontend stack. T12 now specifies Windows 11 client only, Linux server only, and ARM/x86 families on both, superseding Linux-client scope; bitness and Linux versions remain open. See [decision evidence](WORK_LOG.md#2026-10-11-0550-utc0300-first-foundation-answers-recorded). T06-T09 remain undecided; all F01-F07 remain not started and S01-S06 proposed.

2026-10-11 further update: user clarified ARM64/x86-64 only on both OSes and confirmed shared-engine boundaries (T06) and versioned YAML profiles (T07). Results (T08), pairing (T09), and Linux version targets remain pending. See [decision evidence](WORK_LOG.md#2026-10-11-0552-utc0300-architecture-and-profile-answers-recorded). Feature implementation and optional-proposal states are unchanged.

2026-10-11 foundation completion: T08 is confirmed as JSONL raw observations, JSON summary/settings, and CSV export. T09 is confirmed as generated pairing plus user-provided TLS certificates after clarification. T12 requires broad major Linux distributions including CentOS/Ubuntu/Fedora/Arch and others; exact versions remain open for integration review. Roadmap action 1 is complete as a documentation milestone; action 2 is next and not started. See [decision/check evidence](WORK_LOG.md#2026-10-11-0559-utc0300-foundation-decisions-completed). F01-F07 remain accepted/not started; S01-S06 remain proposed.

2026-10-11 measurement-contract discussion: roadmap action 2 is in discussion. T14 has a lifecycle/accounting/report/dashboard/pairing draft; T15 broad traffic configurability and T18 deadline-miss-first presentation with RTT tails/variation, ties/inconclusive and no combined score are confirmed. Burst mechanics/data-plus-receipt design and evidence rules remain open; numerical defaults are not selected. The [draft](DESIGN.md#measurement-contract-draft) includes a prose profile, synthetic accounting report, and future acceptance examples. See [initial evidence](WORK_LOG.md#2026-10-11-0614-utc0300-measurement-contract-draft-started) and [user-answer evidence](WORK_LOG.md#2026-10-11-0616-utc0300-configurable-workload-and-report-presentation-answers). F01-F07 still have no application implementation; S01-S06 remain proposed. Pairing refinements also concern F01; port/priority reporting concepts preserve F05/F06 scope without validating capability.

2026-10-11 contract extension: user confirmed ordered traffic-phase sequences in addition to independent steady/burst controls (T15), and requested an explanation of T14's data-plus-receipt model rather than accepting it. Phase controls/reporting are documented; implementation remains not started. See [decision evidence](WORK_LOG.md#2026-10-11-0618-utc0300-traffic-phase-sequences-and-receipt-clarification). This extends F07's configurable workload contract; S01-S06 remain unapproved, including automated sweeps under S04.

2026-10-11 measurement-contract completion: user confirmed T14's independent bidirectional data plus correlated receipts and carrying information in the payload after explanation. T15 traffic/phase controls and T18 report presentation are already confirmed. Roadmap action 2's documentation outcome is complete; exact schemas, operational limits, numerical defaults, credential mechanics, and recommendation evidence thresholds remain open for integration/slice planning. See [completion/check evidence](WORK_LOG.md#2026-10-11-0624-utc0300-measurement-pattern-confirmed-and-contract-milestone-completed). F01-F07 remain unimplemented and S01-S06 unapproved; no implementation authorization inferred.

2026-10-11 integration-review completion: roadmap action 3's source/documentation milestone is complete. T05/T11/T12/T16/T17 now have [capability expectations and gaps](CHOICES.md#integration-constraints-review): managed socket/port hooks, WireGuard DSCP integration work, Windows priority privileges/observations, recursive DNS delegation and polling, and Linux compatibility boundaries. See [review/check evidence](WORK_LOG.md#2026-10-11-integration-constraints-review-completed). This advances design research for F01-F06 and adapter-specific F07 accounting, not application implementation. F01-F07 remain not started/unimplemented, S01-S06 unapproved, exact dependencies/platform minima unselected, and implementation on hold. Action 4 is next/planned.

2026-10-11 reference-tool planning: user confirmed selective learning/improvement from existing tools while explicitly keeping implementation on hold. The inventory below maps existing scope and records proposed S07-S11 additions; T19/T20 hold reuse and validation choices. Roadmap action 3a is complete as documentation, action 3b is next/planned, and action 4 still requires explicit implementation authorization. F01-F07 remain unimplemented; S01-S11 remain proposed. See [planning/check evidence](WORK_LOG.md#2026-10-11-reference-tool-planning-recorded).

## Optional proposals

S01-S06 are the original suggestions; their stable identifiers are preserved. S07-S11 are reference-tool candidates recorded on 2026-10-11. All remain proposed, separate from confirmed F01-F07 coverage. The user's direction to selectively learn from existing tools and improve suitable functionality authorizes this planning, not adoption of every candidate or implementation.

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

Recorded: 2026-10-11. This inventory captures useful ideas from the researched tools, not a completed source audit or proven integration. Prefer improvements that preserve our shared measurement contract across authentic adapters. Existing requirements below remain F01-F07; they are not new optional features or claims of implementation. [Planning evidence](WORK_LOG.md#2026-10-11-reference-tool-planning-recorded).

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

### S07: Independent reference-tool cross-checks

- [ ] Not implemented; proposed for later consideration. [Planning evidence](WORK_LOG.md#2026-10-11-reference-tool-planning-recorded).

Define a repeatable validation comparison against IRTT, iperf2, sockperf, Crusader or Flent where their measurements overlap ours. Retain tool/version, settings, raw evidence, endpoint/path, timing boundaries and observed workload; explain discrepancies rather than assuming either tool is ground truth.

Purpose: independently check shared-engine timing, delivery accounting and report calculations for F02/F07. This extends validation evidence, not the product's normal managed endpoint workflow.

Before adoption: settle T20's reference matrix, compatible builds and tolerances; match payload including metadata/receipts, actual offered rates, direction, warmup, cutoff/grace and load. Compare only equivalent quantities. A UDP reference does not validate TLS/QUIC/WireGuard/Hysteria2/DNS behavior. External benchmark execution requires later authorization; no external CLI becomes a mandatory runtime dependency by this proposal.

### S08: Timing preflight and diagnostic report

- [ ] Not implemented; proposed for later consideration. [Planning evidence](WORK_LOG.md#2026-10-11-reference-tool-planning-recorded).

Offer an optional local preflight that measures timer wake-up error, scheduling lag and submission overhead, plus a diagnostic view during runs. Keep these observations separate from network RTT, receiver processing and adapter queues; identify CPU pressure and unsupported observations.

Purpose: extend F07's already required timing records with an explainable diagnostic workflow inspired by IRTT. This is not cross-host clock calibration and cannot establish one-way delay.

Before adoption: define preflight duration, bounded resource use, sample windows and advice under T14/T18. Record changes between preflight and the run; do not automatically raise OS scheduling priority, change global settings, subtract a measured floor from all RTTs or dismiss network failures as host problems.

### S09: Distribution and phase comparison views

- [ ] Not implemented; proposed for later consideration. [Planning evidence](WORK_LOG.md#2026-10-11-reference-tool-planning-recorded).

Add optional histograms/CDFs and matched-run overlays alongside phase timelines for completed RTT, scheduling error, deadline outcomes and delivery interruptions. Show direction, workload phase, units, sample counts, missing/late outcomes and measurement boundaries; allow inspection of the underlying records.

Purpose: extend F07's existing live summaries and comparison controls using ideas from iperf2, Flent, Crusader and Graphite Meter. Complements S02/S06 without adopting their additional metrics or confidence policy.

Before adoption: settle T13/T14/T18 chart layout, binning/percentile methods and bounded updates. Keep deadline misses first, no combined score, and explicit ties/inconclusive outcomes. Completed-sample distributions must not hide missing slow traffic. A phase-aligned display does not establish synchronized host clocks; plots never drive scheduling.

### S10: Controlled contention scenario catalogue

- [ ] Not implemented; proposed for later consideration. [Planning evidence](WORK_LOG.md#2026-10-11-reference-tool-planning-recorded).

Document reproducible validation scenarios: idle, external upload load, external download load, simultaneous load, bulk TCP beside FPS traffic, and bounded non-responsive UDP contention. Add isolated queue-policy/DSCP-classification fixtures when relevant, with actual load and topology recorded separately from the measurement profile.

Purpose: make F07's existing externally busy-network validation reproducible, drawing on Flent and bufferbloat-lab. This is a validation catalogue, not in-product background-load orchestration, bandwidth control or router configuration.

Before adoption: settle T20 fixture ownership, isolation, resource budgets, restoration and matched repetitions; account for load-generator CPU and path bypass. Local CAKE class observations do not prove ISP prioritization. Router queue telemetry is optional local evidence, not an inferred measurement of inaccessible ISP queues or per-game-packet delay. Execution and temporary network changes require later authorization.

### S11: Continuous responsiveness monitoring

- [ ] Not implemented; proposed for later consideration. [Planning evidence](WORK_LOG.md#2026-10-11-reference-tool-planning-recorded).

Offer an explicitly started/stopped low-rate session with bounded rolling windows for RTT, variation, deadline outcomes and delivery gaps, inspired by Crusader and IRTT. Keep it separate from finite matched comparison runs and export window identity/settings when retained.

Purpose: reveal intermittent degradation over longer periods without making an old short comparison a current guarantee.

Before adoption: settle T14 resource/retention limits, window denominators, reconnect handling, traffic/receipt cost, and protocol eligibility. Incomplete windows remain visible. Monitoring does not select winners, switch routes, forward games or silently test in the background; richer burst/phase behavior remains under the existing workload choices.

## Maintaining this file

Append future features or suggestions with stable identifiers, purpose, status, and known dependencies/open decisions. Do not invent a release date, priority, or user acceptance.

- **Proposed:** idea awaiting a decision. **Accepted:** confirmed scope, with execution authorization considered separately.
- **In progress / partial:** work has begun or only part of the feature exists; link to actual work-log evidence.
- **Completed:** implementation and applicable checks are complete; retain the identifier and a work-log reference so it is not proposed again. State any platform/protocol coverage limits there.
- **Deferred / rejected:** retain the dated decision and reason rather than deleting the idea and rediscovering it later.

When the user adopts a proposal, update `DESIGN.md` as needed and record the decision in `WORK_LOG.md`. After meaningful implementation changes, update forward status here and backward evidence there. Preserve completed items as concise references; keep detailed code history only in the work log. An implementation or validation gap remains visible rather than being marked complete.
