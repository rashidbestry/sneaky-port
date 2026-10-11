# First actions

Updated: 2026-10-11. Planning steps 1-4 and implementation milestones 5-8 are complete within their stated scopes. **Next: task 9 only.** The full first slice remains incomplete. Requirements are in [DESIGN.md](DESIGN.md), answers in [CHOICES.md](CHOICES.md), feature status in [PROPOSALS.md](PROPOSALS.md), and evidence in [WORK_LOG.md](WORK_LOG.md).

Work one selected task at a time. Each task has a bounded outcome and completion check; update its status/evidence before moving on. Split any task that expands into several outcomes before implementation. Starting one task does not mean continuing through the rest in the same conversation.

## 1. Settle the immediate foundation choices (completed)

**State: complete, 2026-10-11.** Go, local embedded plain-browser UI, client/server packaging, Windows 11/Linux ARM64/x86-64 roles, shared engine, YAML profiles, JSONL/JSON/CSV results and pairing direction settled. [Answers and remaining platform details](ARCHIVE.md#foundation-discussion).

## 2. Define a small measurement contract (completed)

**State: complete, 2026-10-11.** Lifecycle, independent bidirectional data/receipts, configurable steady/burst phases, timing/accounting, dashboard/report concepts and illustrative acceptance examples documented. [Conceptual contract](DESIGN.md#measurement-contract-draft); task 4 supplies the final first-slice mechanics.

## 3. Check integration constraints before locking the plan (completed)

**State: complete as source review, 2026-10-11.** Managed lifecycle, socket/port/outer-priority, DNS delegation/polling and Linux packaging expectations/gaps recorded. [Integration review](ARCHIVE.md#integration-constraints-review). Runtime support remains unvalidated.

## 3a. Record useful reference-tool components (completed)

**State: complete, 2026-10-11.** [Candidate inventory](PROPOSALS.md#reference-tool-candidates) and [reuse choices](ARCHIVE.md#reference-tool-reuse-and-validation) recorded. [Evidence](ARCHIVE.md#2026-10-11-reference-tool-planning-recorded).

## 3b. Evaluate selective reuse and validation baselines (completed)

**State: complete as source review, 2026-10-11.** IRTT/iperf2/Crusader selected as design references/external validation only; S07-S11 accepted for future scope. [Dispositions and source gaps](ARCHIVE.md#selective-source-review-and-dispositions), [acceptance matrix](DESIGN.md#reference-comparison-acceptance-matrix), [evidence](ARCHIVE.md#2026-10-11-selective-reference-review-completed). No reference experiments performed.

## 3c. Review the StarTrinity reference (completed)

**State: complete as vendor-documentation review, 2026-10-11.** [Review](ARCHIVE.md#startrinity-documentation-review) recorded; S12 remains proposed and T24 retains future metric/event choices. No StarTrinity component selected or executed. [Evidence](ARCHIVE.md#2026-10-11-startrinity-reference-review-recorded).

## 4. Agree on the first implementation slice (completed)

**State: complete as planning, 2026-10-11.** [Settled contract](DESIGN.md#first-slice-implementation-contract): real UDP/TCP, shared engine/dashboard/lifecycle, pairing, ports/directional priority, compile-time adapters, embedded key-only SSH coordination and calibration evidence. Basic S08/S09 and S07/S10 checks are included; S11 and remaining carriers/DNS stay future scope.

[Final choices](CHOICES.md#first-slice-final-decisions) distinguish user answers from engineering selections. [Evidence](ARCHIVE.md#2026-10-11-first-slice-planning-completed). Numerical operator inputs remain explicit; compatible releases, Linux minima and empirical timing bands need later checks. No runtime gate has passed.

<a id="5-implement-and-validate-the-agreed-first-slice"></a>

## First-slice task index

The former task 5 bundled implementation and all acceptance gates. On the user's instruction it is replaced by tasks **5-35** below; the old link target is retained for historical references. This changes task boundaries, not accepted F/S/T scope or the original implementation authorization. Dependencies and external access may affect later validation order; unavailable evidence stays pending.

## 5. Build and test foundation modules (completed)

**Complete for the foundation scope.** Strict profiles/hash, wire/adapters, scheduling, ledger/calibration primitives, local preflight and commented source/flowchart exist. Module fixtures and four builds passed; a connected measurement engine is outside this task. [Evidence](ARCHIVE.md#2026-10-11-first-implementation-foundations).

## 6. Implement pairing and authenticated readiness (completed)

**Complete for component implementation and recorded fixtures.** Persistent public trust, key-only embedded SSH, exact-plan UDP/TCP binding, effective settings, leases/revocation and clean stop pass Windows/WSL fixtures. This task starts no measurement; Windows-to-Linux CLI and native/platform acceptance remain tasks 23-27. [Evidence and access restriction](ARCHIVE.md#2026-10-11-ssh-pairing-and-readiness), [code flow](FLOWCHART.md#persistent-trust-and-endpoint-readiness).

<a id="7-implement-bounded-writer-queues"></a>

## 7. Implement bounded writer queues (completed)

**Complete for queue component implementation and fixtures.** F02/F07, T06/T15. Shared FIFO workers, active-inclusive message/byte caps, receipt reserves, typed admission/completion reasons and bounded cancellation/join exist. Saturation, FIFO/accounting, partial-write failure and cancellation fixtures pass on Windows and Ubuntu/CentOS WSL; four target package builds pass. User-reported Windows race checks pass for 20 queue repetitions and the full suite. No CLI measurement, sampler, reporting or UI connected. [Queue evidence](WORK_LOG.md#2026-10-11-bounded-writer-queues), [race evidence](WORK_LOG.md#2026-10-11-windows-race-instrumentation), [queue policy](DESIGN.md#phase-scheduling-and-resource-policy), [code flow](FLOWCHART.md#bounded-writer-queues).

<a id="8-implement-independent-udp-senderreceiver-workers"></a>

## 8. Implement independent UDP sender/receiver workers (completed)

**Complete for one-window UDP component implementation and fixtures.** F02/F07, T06/T15. Independent local schedules, reads and real FIFO writes, bounded events/identities/outstanding admission, reverse receipts and joined cancellation exist. Windows IPv4/IPv6 and Ubuntu/CentOS WSL fixtures, full Go checks, 20 worker race repetitions/full race suite and four target package builds pass. Full phase/ledger integration, TCP and CLI lifecycle remain tasks 9-11. [Evidence](WORK_LOG.md#2026-10-11-independent-udp-workers), [code flow](FLOWCHART.md#independent-udp-workers).

## 9. Integrate phase timing and measurement accounting

**State: next; not started.**

Scope: connect the component events/tickets and existing ledger to warmup/main phases, burst/quiet windows, original identities, deadlines and cutoffs. Extend the one-window UDP connection through the agreed multi-phase contract; enforce run-wide outstanding/evidence budgets and preserve local skips, partial writes and interrupted windows.

Done when timing/accounting fixtures reconcile all reached slots, admissions/attempts/completions, receiver data and receipts; deadline/cutoff, RTT/variation and spike/stall cases pass. Complete applicable Go checks, comments, flowchart and status/evidence updates, then report task 10 as next. TCP and finite CLI lifecycle remain separate tasks.

## Queued implementation tasks

Every row is a separate task, currently **not started**. Existing primitives are reused; these rows describe integration outcomes rather than new feature acceptance. Tasks 9-11 finish assembling the finite measurement engine in separate checkpoints.

| Task | Bounded outcome | Completion check |
| --- | --- | --- |
| 10 | Connect TCP to the shared workers | Blocked/partial writes preserve independent scheduling, complete-frame arrivals and honest stream-stall/unknown packet-loss outcomes |
| 11 | Connect finite run start/stop lifecycle | Frozen-plan start, phase-specific markings, grace, cancellation/control loss and lease cleanup pass; expose the finite CLI run workflow |
| 12 | Persist bounded local raw evidence and summaries | Completed and interrupted runs retain exact events/cutoffs within file budgets; finalization does not expose secrets |
| 13 | Replay and export local reports | Recomputed summaries match raw records; JSON/JSONL/CSV preserve units, decimal precision, unknowns and partial status |
| 14 | Transfer and reconcile server evidence | Bounded authenticated chunks/checksums bind to the same run/plan; missing/incompatible receiver evidence remains unknown |
| 15 | Maintain live SSH clock samples | Four-timestamp captures, epochs, cadence/window/expiry and invalid samples are retained within budgets |
| 16 | Integrate uncertainty-gated one-way results | Known offset/drift/asymmetry/expiry fixtures pass; unverified production bounds remain estimated or unavailable |
| 17 | Protect the local run-control API | Loopback binding, session token and Host/Origin guards protect adapter/status/start/cancel routes |
| 18 | Connect saved profile/pairing/report API operations | Validated IDs, explicit saves and bounded imports share existing validators; active frozen plans stay unchanged |
| 19 | Build dashboard setup and run controls | Pairing/profile editing and explicit save/start/cancel/status work through the protected API |
| 20 | Add diagnostic and distribution views | Bounded SVG time/ECDF and phase/preflight views preserve exact final quantiles, counts and missing samples |
| 21 | Compare matched saved runs | Selected repeated trials show compatible deltas/ties/inconclusive states without inventing winners |
| 22 | Complete the adapter extension gate | One test adapter uses common configuration, lifecycle, accounting, UI and reports without protocol switches in shared code |

## Queued validation tasks

All are **pending evidence**. Use [acceptance gates](DESIGN.md#validation-before-supportperformance-claims) and the [reference matrix](DESIGN.md#reference-comparison-acceptance-matrix) for exact cases and provenance. Record one named environment/scenario at a time; split multi-case work into explicit checklist items before execution. Build, loopback, native, impairment and live evidence stay distinct. These tasks do not authorize changing user-owned settings or selecting new reference dependencies.

| Task | Bounded outcome | Completion check |
| --- | --- | --- |
| 23 | Windows x86-64 client to Linux x86-64 CLI workflow | Real pairing/readiness/run/cancel/export completes across endpoints; current access restriction resolved or recorded as outstanding |
| 24 | Native Windows 11 ARM64 client case | Client workflow passes on native ARM64 against a documented Linux server; compilation alone does not complete it |
| 25 | Native Linux ARM64 server case | Server workflow passes on native ARM64 with recorded OS/kernel, permissions and cleanup |
| 26 | Establish a Linux support-baseline case | Record and validate a named distribution/version minimum; additional claimed distributions/versions get separate cases |
| 27 | Native directional marking and port evidence | Named backend/direction cases distinguish requested/applied/captured DSCP, ECN and actual/NAT ports, including unsupported outcomes |
| 28 | Validate clock bounds and directional delay | Independently justified timing/error bounds and controlled offset/drift/asymmetry cases support or limit one-way classifications |
| 29 | Controlled delay/spike case | Known directional delay/spikes reconcile with timestamps, completed samples and deadlines |
| 30 | Controlled loss/delivery case | Random/burst data loss and lost receipts reconcile separately with complete receiver evidence |
| 31 | Controlled duplicate/reorder case | Duplicate/reordered/foreign/replayed messages preserve unique counts, identities and valid sample populations |
| 32 | Stream blockage, MTU and interruption cases | Each named case retains partial writes/skips, bounded resources, observation limits, partial reports and cleanup |
| 33 | Independent reference overlap cases | Separate UDP/TCP/load-reference cases retain compatible workloads/windows, raw evidence and predeclared comparison bands |
| 34 | Matched live-path case | Repeated matched trials retain environment, offered workload and uncertainty; incompatible/incomplete results remain inconclusive |
| 35 | Review first-slice acceptance evidence | Every applicable contract gate is linked to evidence; unresolved required cases keep the full slice incomplete |

F10's [protocol traffic controls](DESIGN.md#protocol-specific-traffic-controls) inform dashboard task 19. Split richer tweak implementation into named adapter/layer outcomes before starting it; T25's schema/fragmentation/header decisions remain open. This requirement update does not complete UI work or change the existing version-1 wire format.

Later carriers, DNS and monitoring keep their existing future status in [PROPOSALS.md](PROPOSALS.md). No release date or acceptance result is inferred from this breakdown.
