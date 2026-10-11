# First actions

Updated: 2026-10-11. Planning steps 1-4 and implementation milestones 5-18 are complete within their stated scopes. **Next: task 19 only.** The full first slice remains incomplete. Requirements are in [DESIGN.md](DESIGN.md), answers in [CHOICES.md](CHOICES.md), feature status in [PROPOSALS.md](PROPOSALS.md), and evidence in [WORK_LOG.md](WORK_LOG.md).

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

**Complete for one-window UDP component implementation and fixtures.** F02/F07, T06/T15. Independent local schedules, reads and real FIFO writes, bounded events/identities/outstanding admission, reverse receipts and joined cancellation exist. Windows IPv4/IPv6 and Ubuntu/CentOS WSL fixtures, full Go checks, 20 worker race repetitions/full race suite and four target package builds pass. Task 9 below now supplies phase/ledger integration; TCP and CLI lifecycle remain tasks 10-11. [Evidence](WORK_LOG.md#2026-10-11-independent-udp-workers), [code flow](FLOWCHART.md#independent-udp-workers).

<a id="9-integrate-phase-timing-and-measurement-accounting"></a>

## 9. Integrate phase timing and measurement accounting (completed)

**Complete for internal UDP phase/accounting integration and recorded fixtures.** F02/F07, T14/T15/T18. Frozen warmup/main/burst/quiet timelines, run-wide sequences/deadline expiry, reached-slot/admission/write/ticket ledgers, byte-bounded local records, strict cutoffs and RTT/variation/spike/stall analysis exist. Deterministic and Windows/Ubuntu/CentOS IPv4/IPv6 loopback fixtures, ordinary/race Go checks and four builds pass. [Evidence](WORK_LOG.md#2026-10-11-udp-phase-accounting), [code flow](FLOWCHART.md#udp-phase-accounting).

## 10. Connect TCP to the shared workers (completed)

**Complete for internal TCP integration and recorded fixtures.** F02/F07, T06/T14/T15/T18. UDP/TCP now share phase schedules, bounded FIFO workers and accounting. TCP retains complete-frame arrivals, four-byte prefix/partial-byte evidence, queued cancellation and application receipt stalls; underlying packet loss stays unknown. Windows/Ubuntu/CentOS IPv4/IPv6 loopback and injected local blockage/truncation fixtures, ordinary/full race checks, 20 worker/engine race repetitions and four builds pass. [Evidence](WORK_LOG.md#2026-10-11-tcp-integration), [code flow](FLOWCHART.md#udptcp-phase-accounting).

## 11. Connect finite run start/stop lifecycle (completed)

**Complete for finite CLI/component implementation and recorded fixtures.** F01/F02/F06/F07/F09, T09/T11/T14/T15/T17/T22. Authenticated frozen-plan start, independent local anchors, physical-phase writer markings, grace, bounded generation-stop/drain/join, lease/control-loss cleanup and exclusive client reservation connect the shared UDP/TCP engine to `client-run`. Public snapshots retain local partial/complete evidence and unknown remote delivery; server status is lifecycle evidence only. [Evidence](WORK_LOG.md#2026-10-11-finite-run-lifecycle), [code flow](FLOWCHART.md#finite-run-lifecycle).

**Task 12 now supplies bounded local persistence.** Tasks 13-14 now supply replay/export and authenticated server reconciliation. Native ARM64, cross-endpoint CLI deployment, outer captures, calibrated one-way, impairment and live acceptance remain pending.

## 12. Persist bounded local raw evidence and summaries (completed)

**Complete for local persistence and recorded fixtures.** F07/F09, T08/T14/T22. Both endpoints create exclusive restricted run files before measurement, preserve exact decimal JSONL observations through bounded storage queues/file reservations, and flush/publish endpoint-only summaries and manifests. Completed/interrupted UDP/TCP runs and storage fault/budget fixtures pass; incomplete or failed publication stays explicit. Tasks 13-14 now supply replay/export and authenticated server reconciliation. [Evidence](WORK_LOG.md#2026-10-11-local-evidence-persistence), [code flow](FLOWCHART.md#local-evidence-persistence).

**Task 13 now supplies local replay/export.** Full slice/platform/impairment/live acceptance remains incomplete.

## 13. Replay and export local reports (completed)

**Complete for bounded offline local replay/export and recorded fixtures.** F07/F09, T08/T14/T22. Fixed-file schema/identity/plan/hash/cutoff validation and shared accounting recompute finalized summaries exactly. JSON settings/summary, typed JSONL evidence bundles and fixed long-form CSV preserve units, exact decimal text, unknowns and partial status. Unfinalized/failed/torn-prefix evidence stays partial; checksums establish local consistency only. [Evidence](WORK_LOG.md#2026-10-11-local-report-replay-and-export), [code flow](FLOWCHART.md#local-report-replay-and-export), [commands](README.md#replay-and-export-local-reports).

**Task 14 below supplies authenticated server evidence transfer/reconciliation.** Full slice/platform/impairment/live acceptance remains incomplete.

## 14. Transfer and reconcile server evidence (completed)

**Complete for authenticated post-stop transfer/reconciliation and recorded fixtures.** F07/F09, T08/T14/T22. One report channel on the existing pinned SSH connection streams fixed files in numbered 32 KiB chunks with checksums, acknowledged offsets, final content hash and explicit file/request/timeout budgets. Shared replay/ledger matches original run/plan/epochs/attempts and independently declared cutoffs, preserving unknown partial/incompatible delivery and ambiguous writes. Restricted transferred files and separate derived sidecars support exact offline replay/exports without rewriting endpoint-only summaries. Windows/Ubuntu/CentOS IPv4/IPv6 fixtures, failure/tampering/accounting tests, Windows race checks and four builds pass. [Evidence](WORK_LOG.md#2026-10-11-authenticated-server-evidence-transfer-and-reconciliation), [code flow](FLOWCHART.md#authenticated-server-evidence-transfer-and-reconciliation), [operator behavior](README.md#run-a-finite-measurement).

## 15. Maintain live SSH clock samples (completed)

**Complete for live sampling and recorded fixtures.** F09, T22/T23. One authenticated in-run SSH channel captures four timestamps in the data epochs, serialization/send-call costs, invalid exchanges, skipped cadence slots and application-byte counters. Bounded windows, expiry and separate raw-file reserves preserve replay/export without deriving one-way results. Windows/Ubuntu/CentOS IPv4/IPv6 UDP/TCP and fault fixtures, Windows race checks and four builds pass. [Evidence](WORK_LOG.md#2026-10-11-live-ssh-clock-sampling), [code flow](FLOWCHART.md#live-ssh-clock-sampling), [timing policy](DESIGN.md#calibration-and-one-way-evidence-policy).

## 16. Integrate uncertainty-gated one-way results (completed)

**Complete for post-reconciliation directional integration and recorded fixtures.** F09, T22/T23. Matched attempts/complete receiver arrivals use immutable causal clock-window revisions and full feasible offset/drift/error envelopes. Signed intervals, sample/model/raw-file IDs, age, clock-local units, exact directional populations and bounded detail omissions survive replay/export. Known offset/drift/asymmetry, inconsistent/foreign/expired/recovering clocks, both capture lifetimes, signed/large-integer and exceed-policy gates pass; real SSH/UDP/TCP IPv4/IPv6 reports retain estimates. Production bounds stay unverified, with estimates/unavailable results; independent/native/clock-change/live calibration remains task 28. [Evidence](WORK_LOG.md#2026-10-11-uncertainty-gated-one-way-integration), [code flow](FLOWCHART.md#uncertainty-gated-one-way-results).

## 17. Protect the local run-control API (completed)

**Complete for the protected local adapter/start/cancel/status subset and recorded fixtures.** F01/F08/F09, T06/T14/T22. `client-api` serves literal IPv4/IPv6 loopback only, with process-random bearer entropy, exact Host/Origin/peer guards on every route, strict bounded JSON and atomic identity-scoped idempotency. An explicit launch-time frozen profile/hash connects independent HTTP controls to the existing finite SSH/UDP/TCP runner; one active run, bounded histories, honest unknown/terminal status and shutdown joins preserve engine ownership. Windows ordinary/race fixtures, Ubuntu/CentOS WSL API and real HTTP/SSH/UDP CLI fixtures and four builds pass. Task 18 supplies saved operations; live phase/counter views and full acceptance remain pending. [Evidence](WORK_LOG.md#2026-10-11-protected-local-run-api), [code flow](FLOWCHART.md#protected-local-run-api), [operator contract](README.md#protected-local-run-api).

## 18. Connect saved profile/pairing/report API operations (completed)

**Complete for protected saved operations and recorded fixtures.** F01/F07/F08/F09, T06/T14/T22. Safe IDs, bounded catalogs, shared profile validation, explicit revision-checked atomic saves, public pairing replacement/key generation and report replay/import/JSON/JSONL/CSV export are connected. Private admitted plans and accepted start/cancel retries remain unchanged by later saves. Imports validate fixed raw files before new-ID publication and never authenticate offline evidence or resume traffic. Windows ordinary/race and Ubuntu/CentOS WSL fixtures, comment audit and four builds pass. [Evidence](WORK_LOG.md#2026-10-11-saved-configuration-and-report-api), [source flow](FLOWCHART.md#saved-configuration-and-report-api), [operator contract](README.md#protected-local-run-api).

**Next: task 19 only, build dashboard setup and run controls.** Live views, comparisons and full slice acceptance remain incomplete.

## Queued implementation tasks

Every row is a separate task, currently **not started**; task 19 is next. These outcomes extend the finite engine using existing primitives; they do not accept additional features.

| Task | Bounded outcome | Completion check |
| --- | --- | --- |
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
