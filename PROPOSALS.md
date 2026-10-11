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

## Optional proposals

S01-S06 are the original suggestions; their stable identifiers are preserved. They are separate from confirmed F01-F07 coverage.

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

## Maintaining this file

Append future features or suggestions with stable identifiers, purpose, status, and known dependencies/open decisions. Do not invent a release date, priority, or user acceptance.

- **Proposed:** idea awaiting a decision. **Accepted:** confirmed scope, with execution authorization considered separately.
- **In progress / partial:** work has begun or only part of the feature exists; link to actual work-log evidence.
- **Completed:** implementation and applicable checks are complete; retain the identifier and a work-log reference so it is not proposed again. State any platform/protocol coverage limits there.
- **Deferred / rejected:** retain the dated decision and reason rather than deleting the idea and rediscovering it later.

When the user adopts a proposal, update `DESIGN.md` as needed and record the decision in `WORK_LOG.md`. After meaningful implementation changes, update forward status here and backward evidence there. Preserve completed items as concise references; keep detailed code history only in the work log. An implementation or validation gap remains visible rather than being marked complete.
