<a id="first-actions"></a>

# Roadmap

Current release: Windows 11/Linux x86-64 UDP/TCP only. Implementation checkpoints pass within recorded scopes; **full acceptance remains incomplete**. Latest Linux runtime checks are user-deferred. [Contract/decisions](DESIGN.md), [validation summaries](WORK_LOG.md).

## 1. Settle the immediate foundation choices (completed)

Foundation decisions recorded; Go/browser/roles/configuration/reporting selected. Current answers are in [DESIGN.md](DESIGN.md#technical-choices).

## 2. Define a small measurement contract (completed)

Independent bidirectional data plus correlated receipts, configurable phases and deadline-first presentation documented. Numerical examples were not accepted as universal tuning.

## 3. Check integration constraints before locking the plan (completed)

Integration review documented socket/priority/platform/DNS constraints; it is source evidence, not runtime qualification.

## 3a. Record useful reference-tool components (completed)

Reference components identified; external design/validation scope retained.

## 3b. Evaluate selective reuse and validation baselines (completed)

IRTT/iperf2/Crusader selected as references; S07-S11 accepted for future scope on 2026-10-11.

## 3c. Review the StarTrinity reference (completed)

StarTrinity informed monitoring/distribution ideas; no MOS, availability thresholds or formula adoption.

## 4. Agree on the first implementation slice (completed)

UDP/TCP first-slice contract and engineering choices settled; implementation authorized 2026-10-11. Planning completion is separate from acceptance.

## First-slice task index

## 5. Build and test foundation modules (completed)

Shared strict profiles/canonical hashing, scheduling, probe/adapter/platform/accounting foundations and scoped fixtures implemented.

## 6. Implement pairing and authenticated readiness (completed)

Generated key pairing, authorized SSH, frozen-plan negotiation, authenticated UDP/TCP binding, readiness and bounded lease/cleanup implemented.

## 7. Implement bounded writer queues (completed)

Active-inclusive bounded FIFO writers, receipt reserves and exact ticket/partial-write/cancellation evidence implemented; recorded component/race scopes pass.

## 8. Implement independent UDP sender/receiver workers (completed)

Independent UDP schedules/readers/writers and authenticated data/receipt accounting implemented; local skips do not become packet loss.

## 9. Integrate phase timing and measurement accounting (completed)

Warmup/main/burst/quiet, cutoff/deadline attribution, ledger populations and RTT/variation/spike/stall fixtures implemented.

## 10. Connect TCP to the shared workers (completed)

Framed TCP uses shared workers/accounting, complete-frame arrival and explicit partial-write/read failures; eventual delivery does not prove zero network loss.

## 11. Connect finite run start/stop lifecycle (completed)

Authenticated finite start/heartbeat/stop, local anchors, physical-phase marking, grace, cancellation and endpoint cleanup integrated.

## 12. Persist bounded local raw evidence and summaries (completed)

Bounded append-only local evidence, storage failure/partial state and atomic manifest/summary finalization implemented.

## 13. Replay and export local reports (completed)

Fixed-file bounded replay and exact JSON/JSONL/CSV exports implemented; offline integrity does not authenticate imported evidence.

## 14. Transfer and reconcile server evidence (completed)

Post-stop authenticated server transfer and identity/cutoff-gated directional reconciliation implemented; missing remote records stay unknown.

## 15. Maintain live SSH clock samples (completed)

Bounded live SSH four-timestamp captures, cadence/invalid samples and immutable raw evidence implemented; no clock-accuracy claim.

## 16. Integrate uncertainty-gated one-way results (completed)

Causal uncertainty envelopes and directional estimate/unavailable results survive replay/export; production bounds remain task 28.

## 17. Protect the local run-control API (completed)

Loopback Host/Origin/peer/bearer protection, frozen run control/status and accepted start/cancel retries implemented.

## 18. Connect saved profile/pairing/report API operations (completed)

Revision-checked profiles, public trust/key operations and validated saved report import/replay/export implemented.

## 19. Build dashboard setup and run controls (completed)

Embedded dashboard setup/run/recovery integrated; later checkpoints define the current Test-only layout.

## 20. Add diagnostic and distribution views (completed)

Saved diagnostics/ECDF/time views and optional local preflight implemented; native clock quality remains separate.

## 21. Compare matched saved runs (completed)

Bounded exact B-minus-A saved-run comparisons implemented with ties/incompatible/inconclusive states and no winner.

## 22. Complete the adapter extension gate (completed)

Compile-time test-only adapter passes common configuration/control/engine/accounting/UI/report/cleanup gates; production stays UDP/TCP.

## 23. Validate the Windows x86-64 client to Linux x86-64 CLI workflow (completed)

Named Windows x86-64 to Ubuntu x86-64 WSL2 UDP/TCP readiness/completion/cancellation/replay/export workflow passes; virtual-path evidence is not native minimum/live acceptance.

## Dashboard completion checkpoints

### 23a. Add shared per-metric evidence presentation (completed)

Accepted S06 labels add per-metric coverage/basis/count/reason without changing values or mandatory gates.

### 23b. Add live client-local phase and counters (completed)

Bounded copied client-local phase/count/RTT snapshots implemented; terminal replay is authoritative.

### 23d. Replace the stacked dashboard with section navigation (completed)

Section-navigation milestone completed historically; superseded by Test-only 23l.

### 23e. Open directly, connect over Linux SSH and run the edited plan (completed)

Automatic local dashboard access and normal Linux SSH password workflow implemented; exact edited plans reach the owned helper.

### 23f. Resolve copied server executables and preserve accepted SSH trust (completed)

Copied helper resolution and persistence of authenticated pins repaired; later bundling supersedes manual executable selection.

### 23c. Verify the rendered dashboard (completed)

Current Test dashboard rendered QA completed in connected Chrome: edits/layout/keyboard, live/final UDP/TCP, cancellation/recovery/preflight/conflicts/cleanup. Modeled Linux shell/upload fixtures do not qualify native sshd/clock/live paths.

### 23g. Upload and start the bundled Linux helper (completed)

Matching Linux helper embedded/uploaded/checked/started in owned temporary state; EOF/shutdown cleanup implemented.

### 23h. Top connection bar and automatic first fingerprint trust (completed)

Persistent connection bar and authenticated first-pin trust; changed pins require explicit replacement.

### 23i. Make test setup usable without IDs or JSON (completed)

Usable settings, hidden managed IDs/revisions, readable automatic results and bounded storage autofill implemented.

### 23j. Show automatic readable test tracking and results (completed)

Automatic terminal replay/cards/charts and run/recovery tracking implemented; incomplete evidence remains visible.

### 23k. Put conditional test settings on one main page (completed)

Conditional protocol settings consolidated on one page; exact manual values retained.

### 23l. Keep only the Test dashboard (completed)

Current dashboard keeps only Test; saved diagnostic/comparison APIs remain outside the normal UI.

### 23m. Repair sixty-second dashboard runs (completed)

Real 60-second UDP/TCP loopback runs complete after control/storage budgeting repair; terminal diagnostics use the owned-run 64-MiB cap and 30-s/35-s HTTP/browser budgets.

### 23n. Replace receipt-count plotting with live latency (completed)

Numeric receipt outcomes and a client RTT chart in ms over elapsed seconds replace receipt-count plotting; provisional/final boundaries retained.

### 23o. Use built-in server defaults and port 22022

Implemented 2026-10-15: Linux no-argument server, app SSH 22022, generated private state and compiled ceilings/client plans. Windows checks/cross-build pass; Linux runtime validation is user-deferred, so this heading is not completed.

### 23p. Start clients with an optional profile file (completed)

Optional profile loading and fresh compiled UDP starter implemented; strict overrides/saved precedence preserved. Windows ordinary/vet/race/no-profile startup and both builds/helper equality pass; Linux runtime remains deferred.

### 23q. Start the Windows client without arguments (completed)

Zero-argument Windows dashboard and per-user store implemented; explicit commands/stores remain. Native packaged empty-argument startup, affected checks/builds/helper equality pass. Linux retains server role; no automatic store migration.

## Queued validation tasks

Remaining platform/clock/reference/live gates are listed under task 35. No deferred Linux run is authorized by documentation maintenance.

### Tasks 24-25: retired architecture cases

Windows/Linux ARM64 cases 24-25 were retired by the user on 2026-10-13. They are not shipping targets or release prerequisites.

### Task 26a: Ubuntu 24.04.4 VMware workflow (completed)

Named Ubuntu 24.04.4 VMware OpenSSH/upload/direct-data/report/cleanup and 16 cross-built ordinary package binaries pass as native Linux processes. This is not Ubuntu 22.04.5 qualification or a native compiler/race check.

### Task 27a: IPv4 guest and Windows endpoint captures (completed)

Named VMware/Windows Npcap IPv4 captures match; server DSCP 12/client DSCP 0 and ECN 0 observed. Windows DSCP 46 rejects at qWave. Native isolated IPv4/IPv6 marking fixtures pass; NAT/global-IPv6/remaining outer cases remain open.

### Task 34a: On-link matched port trials (completed)

Three alternating matched port pairs per UDP/TCP carrier pass on the on-link VMware path with actual populations/captures. Descriptive comparisons do not establish Internet/ISP or external-load performance.

### Task 29: controlled delays and spikes (completed)

Named controlled delay/jitter/spike cases pass with separate expected-truth/boundary evidence; this is controlled-path acceptance only.

### Task 30: controlled data and receipt loss (completed)

Known-identity data/receipt loss cases pass with exact independent outcomes; missing receipts alone never become receiver loss.

### Task 31: duplicates, reordering and invalid replay (completed)

Duplicate/reorder/invalid replay cases pass with exact identities/populations and no duplicate RTT samples.

### Task 32: stream, MTU, rate and interruption cases (completed)

Named blocked/partial TCP, MTU/rate/interruption and bounded partial-report/cleanup cases pass; no blanket packet-loss or platform claim.

### Task 35: current acceptance review

**Reviewed, incomplete, 2026-10-15.** Completed component/named-path cases do not close the evidence prerequisites below.

| Gate | Remaining prerequisite |
| --- | --- |
| 26 Linux baseline | Qualify Ubuntu 22.04.5 and native compiler/race; record each distribution/kernel/toolchain. VMware24.04.4 success is a named case, not a Linux minimum |
| 27 ports/marking | Privileged nonzero Windows marking, NAT/global IPv6 and remaining independent capture/offload/observation cases |
| 28 clocks | Independent production timestamp/model/drift bounds and clock/rate-change/asymmetry tests; one-way stays estimate/unavailable |
| 29-32 controlled | Named recorded cases pass; they do not qualify arbitrary deployment/live paths |
| 33 references | Compatible independent boundaries and valid 10-baseline/two-held-out calibration/bands. iperf2 pacing warnings remain; four Crusader scenarios and exploratory IRTT overlap do not close this gate |
| 34 live paths | Internet/ISP and controlled external upload/download/both-load trials with broader timing/uncertainty qualification |
| 35 release review | Close applicable evidence gates without promoting partial/unknown results; no ARM requirement |

The full first slice remains incomplete. Routine design choices are settled; the remaining gates need actual prerequisites/evidence.

<a id="technical-choice-progress"></a>

## Future features and proposals

Feature acceptance defines scope, not execution authorization or completion. F01-F10 remain partial/not-started overall. S07-S11 were accepted on 2026-10-11; 2026-10-12 release work was explicitly limited to UDP/TCP.

### Confirmed future features

| ID | Scope | Current status |
| --- | --- | --- |
| F01 | Windows 11 client/one Linux server, broad intended distributions, x86-64 | Partial; managed lifecycle/named WSL/VMware work, selected baseline/minimum and full acceptance pending |
| F02 | Authentic UDP/TCP/TLS/QUIC stream/datagram measurements | Partial; UDP/TCP implemented, TLS/QUIC future |
| F03 | Tool-managed WireGuard/Hysteria2 with inner UDP/TCP | Accepted; not started |
| F04 | DNS ports/direct and resolver-mediated paths | Accepted; not started |
| F05 | Destination/server/optional source ports and comparisons | Partial; selected-port/on-link trials pass, broader NAT/path evidence open |
| F06 | Independent directional priority with outer evidence | Partial; native Linux and named endpoint capture pass, Windows qWave46 denies and remaining qualifications open |
| F07 | Configurable FPS workloads/timing/accounting/replayable evidence | Partial; finite UDP/TCP engine/reports/comparisons implemented, full validation open |
| F08 | Extensible authentic adapters through shared configuration/UI/report contracts | Partial; compile-time extension gate passes, future carrier implementations pending |
| F09 | Authenticated control and maintained uncertainty-aware one-way calibration | Partial; captures/causal models/replay implemented, independently verified production bounds open |
| F10 | Protocol-specific payload/header/fragmentation controls | Partial; current payload/options/phases/ports implemented, richer header/padding/reassembly mechanics deferred |

<a id="reference-review-dispositions"></a>
<a id="reference-tool-candidates"></a>

### Suggestions and accepted additions

### S01: Staged experiment plan

**Deferred proposal.** Automatic staged screening needs experiment selection/order, evidence thresholds and escalation rules. Current execution remains explicit sequential trials; no automatic winner/routing.

### S02: Burst-loss and consecutive-deadline-miss metrics

**Deferred proposal.** Burst/consecutive deadline-miss metrics need identity/window/quiet/partial-run semantics and separation of local unsent work from receiver outcomes. Existing spike/stall accounting is not acceptance of a broader loss-episode metric.

### S03: Separate coordination and measured traffic

**Deferred beyond baseline coordination.** F09 control/calibration and F07 persistence/post-stop transfer are implemented. Extra traffic minimization, old-run retrieval, reconnect/resume and product retention/recovery policies remain open; offline import proves consistency only.

### S04: Packet-size and packet-rate sweeps

**Deferred proposal.** Automated size/rate sweeps need finite bounds, compatibility/scheduling policy and matched trials; current editable controls do not authorize an automatic sweep.

### S05: Exportable recommendation profile

**Deferred proposal.** Exported recommendations need eligibility/material-difference/uncertainty rules. Current exact exports and descriptive comparisons do not select a winner or change routes.

### S06: Separate connection quality from measurement confidence

**Accepted and implemented within recorded component/dashboard scopes.** Per-metric coverage/basis/count/reason preserve values and mandatory evidence gates. Advisory median/p95/p99/repeat targets do not create confidence scores or recommendations. See [presentation policy](USAGE.md#evidence-presentation-and-live-run-progress).

### S07: Independent reference-tool cross-checks

**Accepted future validation scope; partial.** External IRTT/iperf2/Crusader overlaps require matching quantities, pinned verified builds, independent boundary allowances and frozen engineering bands. Runtime smokes/exploratory pairs do not close task 33.

### S08: Timing preflight and diagnostic report

**Accepted; basic implementation exists.** Optional bounded local timer preflight reports scheduling/source evidence without network traffic or OS tuning. It does not establish native accuracy/drift or calibrated one-way delay.

### S09: Distribution and phase comparison views

**Accepted; saved diagnostics implemented.** Bounded time/ECDF/phase views use all eligible raw quantiles and explicit gaps. Saved readers/APIs remain outside the Test-only normal UI; broader clock/native acceptance stays separate.

### S10: Controlled contention scenario catalogue

**Accepted; named controlled cases implemented.** Delay/loss/reorder/stream/MTU/rate/interruption cases pass in recorded scopes. General contention catalogue, external live load and compatible reference comparisons remain incomplete.

### S11: Continuous responsiveness monitoring

**Accepted future scope; not implemented.** Continuous responsiveness monitoring needs lifecycle/storage/aggregation/resource decisions beyond finite runs; no monitoring behavior or thresholds are selected by implication.

### S12: Classified degradation episodes and observed availability

**Deferred proposal.** Versioned degradation/availability episodes need T24 definitions, denominator/window, evidence minima, transitions/debounce/hysteresis, quiet/partial/fault handling and recovery. No-arrival, degraded delivery, local failures and unknown time must remain separate; no borrowed MOS/outage thresholds or ISP-wide conclusion.

### Maintaining feature statuses

Keep stable IDs, provenance, explicit proposed/accepted/partial/completed states and evidence links. Acceptance is not execution authorization; mark completed only after applicable checks. Full older discussions are in the [pinned historical record](ARCHIVE.md#full-historical-record).
