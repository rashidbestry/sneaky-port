# First actions

Updated: 2026-10-15. Shipping targets are Windows 11 x86-64 clients and Linux x86-64 servers. Chrome QA 23c, named controlled cases 29-32 and the VMware subcases below pass; architecture 24-25 is retired. Full acceptance 35 still needs the selected Ubuntu 22.04.5 baseline, remaining marking/clock/reference and broader path evidence. [VMware deployment/capture/matched trials](WORK_LOG.md#2026-10-15-vmware-deployment-and-network-validation), [requirements](DESIGN.md), [choices](DESIGN.md#technical-choices), [feature status](#future-features-and-proposals).

Keep each task separately checkable and update its status/evidence before moving on. The 2026-10-13 user instruction authorizes this remaining release batch; future work retains the normal selected-task workflow. Historical four-target builds below are dated evidence, not current shipping requirements.

## 1. Settle the immediate foundation choices (completed)

**State: complete, 2026-10-11.** Go, local embedded plain-browser UI, client/server packaging, original platform roles, shared engine, YAML profiles, JSONL/JSON/CSV results and pairing direction settled. Shipping scope was revised to Windows/Linux x86-64 on 2026-10-13. [Original answers and provenance](ARCHIVE.md#foundation-discussion).

## 2. Define a small measurement contract (completed)

**State: complete, 2026-10-11.** Lifecycle, independent bidirectional data/receipts, configurable steady/burst phases, timing/accounting, dashboard/report concepts and illustrative acceptance examples documented. [Conceptual contract](DESIGN.md#measurement-contract-draft); task 4 supplies the final first-slice mechanics.

## 3. Check integration constraints before locking the plan (completed)

**State: complete as source review, 2026-10-11.** Managed lifecycle, socket/port/outer-priority, DNS delegation/polling and Linux packaging expectations/gaps recorded. [Integration review](ARCHIVE.md#integration-constraints-review). Runtime support remains unvalidated.

## 3a. Record useful reference-tool components (completed)

**State: complete, 2026-10-11.** [Candidate inventory](#reference-tool-candidates) and [reuse choices](ARCHIVE.md#reference-tool-reuse-and-validation) recorded. [Evidence](ARCHIVE.md#2026-10-11-reference-tool-planning-recorded).

## 3b. Evaluate selective reuse and validation baselines (completed)

**State: complete as source review, 2026-10-11.** IRTT/iperf2/Crusader selected as design references/external validation only; S07-S11 accepted for future scope. [Dispositions and source gaps](ARCHIVE.md#selective-source-review-and-dispositions), [acceptance matrix](DESIGN.md#reference-comparison-acceptance-matrix), [evidence](ARCHIVE.md#2026-10-11-selective-reference-review-completed). No reference experiments performed.

## 3c. Review the StarTrinity reference (completed)

**State: complete as vendor-documentation review, 2026-10-11.** [Review](ARCHIVE.md#startrinity-documentation-review) recorded; S12 remains proposed and T24 retains future metric/event choices. No StarTrinity component selected or executed. [Evidence](ARCHIVE.md#2026-10-11-startrinity-reference-review-recorded).

## 4. Agree on the first implementation slice (completed)

**State: complete as planning, 2026-10-11.** [Settled contract](DESIGN.md#first-slice-implementation-contract): real UDP/TCP, shared engine/dashboard/lifecycle, pairing, ports/directional priority, compile-time adapters, embedded key-only SSH coordination and calibration evidence. Basic S08/S09 and S07/S10 checks are included; S11 and remaining carriers/DNS stay future scope.

[Final choices](DESIGN.md#first-slice-final-decisions) distinguish user answers from engineering selections. [Evidence](ARCHIVE.md#2026-10-11-first-slice-planning-completed). Numerical operator inputs remain explicit; compatible releases, Linux minima and empirical timing bands need later checks. No runtime gate has passed.

<a id="5-implement-and-validate-the-agreed-first-slice"></a>

## First-slice task index

The former task 5 bundled implementation and all acceptance gates. On the user's instruction it is replaced by tasks **5-35** below; the old link target is retained for historical references. This changes task boundaries, not accepted F/S/T scope or the original implementation authorization. Dependencies and external access may affect later validation order; unavailable evidence stays pending.

## 5. Build and test foundation modules (completed)

**Complete for the foundation scope.** Strict profiles/hash, wire/adapters, scheduling, ledger/calibration primitives, local preflight and commented source/flowchart exist. Module fixtures and four builds passed; a connected measurement engine is outside this task. [Evidence](ARCHIVE.md#2026-10-11-first-implementation-foundations).

## 6. Implement pairing and authenticated readiness (completed)

**Complete for component implementation and recorded fixtures.** Persistent public trust, key-only embedded SSH, exact-plan UDP/TCP binding, effective settings, leases/revocation and clean stop pass Windows/WSL fixtures. This task starts no measurement; Windows-to-Linux CLI and native/platform acceptance remain tasks 23-27. [Evidence and access restriction](ARCHIVE.md#2026-10-11-ssh-pairing-and-readiness), [source](../internal/control/session.go).

<a id="7-implement-bounded-writer-queues"></a>

## 7. Implement bounded writer queues (completed)

**Complete for queue component implementation and fixtures.** F02/F07, T06/T15. Shared FIFO workers, active-inclusive message/byte caps, receipt reserves, typed admission/completion reasons and bounded cancellation/join exist. Saturation, FIFO/accounting, partial-write failure and cancellation fixtures pass on Windows and Ubuntu/CentOS WSL; four target package builds pass. User-reported Windows race checks pass for 20 queue repetitions and the full suite. No CLI measurement, sampler, reporting or UI connected. [Queue evidence](WORK_LOG.md#2026-10-11-bounded-writer-queues), [race evidence](WORK_LOG.md#2026-10-11-windows-race-instrumentation), [queue policy](DESIGN.md#phase-scheduling-and-resource-policy), [source](../internal/writer/queue.go).

<a id="8-implement-independent-udp-senderreceiver-workers"></a>

## 8. Implement independent UDP sender/receiver workers (completed)

**Complete for one-window UDP component implementation and fixtures.** F02/F07, T06/T15. Independent local schedules, reads and real FIFO writes, bounded events/identities/outstanding admission, reverse receipts and joined cancellation exist. Windows IPv4/IPv6 and Ubuntu/CentOS WSL fixtures, full Go checks, 20 worker race repetitions/full race suite and four target package builds pass. Task 9 below now supplies phase/ledger integration; TCP and CLI lifecycle remain tasks 10-11. [Evidence](WORK_LOG.md#2026-10-11-independent-udp-workers), [source](../internal/worker/udp.go).

<a id="9-integrate-phase-timing-and-measurement-accounting"></a>

## 9. Integrate phase timing and measurement accounting (completed)

**Complete for internal UDP phase/accounting integration and recorded fixtures.** F02/F07, T14/T15/T18. Frozen warmup/main/burst/quiet timelines, run-wide sequences/deadline expiry, reached-slot/admission/write/ticket ledgers, byte-bounded local records, strict cutoffs and RTT/variation/spike/stall analysis exist. Deterministic and Windows/Ubuntu/CentOS IPv4/IPv6 loopback fixtures, ordinary/race Go checks and four builds pass. [Evidence](WORK_LOG.md#2026-10-11-udp-phase-accounting), [source](../internal/engine/endpoint.go).

## 10. Connect TCP to the shared workers (completed)

**Complete for internal TCP integration and recorded fixtures.** F02/F07, T06/T14/T15/T18. UDP/TCP now share phase schedules, bounded FIFO workers and accounting. TCP retains complete-frame arrivals, four-byte prefix/partial-byte evidence, queued cancellation and application receipt stalls; underlying packet loss stays unknown. Windows/Ubuntu/CentOS IPv4/IPv6 loopback and injected local blockage/truncation fixtures, ordinary/full race checks, 20 worker/engine race repetitions and four builds pass. [Evidence](WORK_LOG.md#2026-10-11-tcp-integration), [source](../internal/engine/endpoint.go).

## 11. Connect finite run start/stop lifecycle (completed)

**Complete for finite CLI/component implementation and recorded fixtures.** F01/F02/F06/F07/F09, T09/T11/T14/T15/T17/T22. Authenticated frozen-plan start, independent local anchors, physical-phase writer markings, grace, bounded generation-stop/drain/join, lease/control-loss cleanup and exclusive client reservation connect the shared UDP/TCP engine to `client-run`. Public snapshots retain local partial/complete evidence and unknown remote delivery; server status is lifecycle evidence only. [Evidence](WORK_LOG.md#2026-10-11-finite-run-lifecycle), [source](../internal/control/run.go).

**Task 12 now supplies bounded local persistence.** Tasks 13-14 now supply replay/export and authenticated server reconciliation. Remaining native x86-64 deployment, outer captures, calibrated one-way and live acceptance use the validation table below.

## 12. Persist bounded local raw evidence and summaries (completed)

**Complete for local persistence and recorded fixtures.** F07/F09, T08/T14/T22. Both endpoints create exclusive restricted run files before measurement, preserve exact decimal JSONL observations through bounded storage queues/file reservations, and flush/publish endpoint-only summaries and manifests. Completed/interrupted UDP/TCP runs and storage fault/budget fixtures pass; incomplete or failed publication stays explicit. Tasks 13-14 now supply replay/export and authenticated server reconciliation. [Evidence](WORK_LOG.md#2026-10-11-local-evidence-persistence), [source](../internal/control/evidence.go).

**Task 13 now supplies local replay/export.** Full slice/platform/impairment/live acceptance remains incomplete.

## 13. Replay and export local reports (completed)

**Complete for bounded offline local replay/export and recorded fixtures.** F07/F09, T08/T14/T22. Fixed-file schema/identity/plan/hash/cutoff validation and shared accounting recompute finalized summaries exactly. JSON settings/summary, typed JSONL evidence bundles and fixed long-form CSV preserve units, exact decimal text, unknowns and partial status. Unfinalized/failed/torn-prefix evidence stays partial; checksums establish local consistency only. [Evidence](WORK_LOG.md#2026-10-11-local-report-replay-and-export), [source](../internal/control/replay.go), [commands](USAGE.md#replay-and-export-local-reports).

**Task 14 below supplies authenticated server evidence transfer/reconciliation.** Full slice/platform/impairment/live acceptance remains incomplete.

## 14. Transfer and reconcile server evidence (completed)

**Complete for authenticated post-stop transfer/reconciliation and recorded fixtures.** F07/F09, T08/T14/T22. One report channel on the existing pinned SSH connection streams fixed files in numbered 32 KiB chunks with checksums, acknowledged offsets, final content hash and explicit file/request/timeout budgets. Shared replay/ledger matches original run/plan/epochs/attempts and independently declared cutoffs, preserving unknown partial/incompatible delivery and ambiguous writes. Restricted transferred files and separate derived sidecars support exact offline replay/exports without rewriting endpoint-only summaries. Windows/Ubuntu/CentOS IPv4/IPv6 fixtures, failure/tampering/accounting tests, Windows race checks and four builds pass. [Evidence](WORK_LOG.md#2026-10-11-authenticated-server-evidence-transfer-and-reconciliation), [source](../internal/control/transfer.go), [operator behavior](USAGE.md#run-a-finite-measurement).

## 15. Maintain live SSH clock samples (completed)

**Complete for live sampling and recorded fixtures.** F09, T22/T23. One authenticated in-run SSH channel captures four timestamps in the data epochs, serialization/send-call costs, invalid exchanges, skipped cadence slots and application-byte counters. Bounded windows, expiry and separate raw-file reserves preserve replay/export without deriving one-way results. Windows/Ubuntu/CentOS IPv4/IPv6 UDP/TCP and fault fixtures, Windows race checks and four builds pass. [Evidence](WORK_LOG.md#2026-10-11-live-ssh-clock-sampling), [source](../internal/control/clock.go), [timing policy](DESIGN.md#calibration-and-one-way-evidence-policy).

## 16. Integrate uncertainty-gated one-way results (completed)

**Complete for post-reconciliation directional integration and recorded fixtures.** F09, T22/T23. Matched attempts/complete receiver arrivals use immutable causal clock-window revisions and full feasible offset/drift/error envelopes. Signed intervals, sample/model/raw-file IDs, age, clock-local units, exact directional populations and bounded detail omissions survive replay/export. Known offset/drift/asymmetry, inconsistent/foreign/expired/recovering clocks, both capture lifetimes, signed/large-integer and exceed-policy gates pass; real SSH/UDP/TCP IPv4/IPv6 reports retain estimates. Production bounds stay unverified, with estimates/unavailable results; independent/native/clock-change/live calibration remains task 28. [Evidence](WORK_LOG.md#2026-10-11-uncertainty-gated-one-way-integration), [source](../internal/control/one_way.go).

## 17. Protect the local run-control API (completed)

**Complete for the protected local adapter/start/cancel/status subset and recorded fixtures.** F01/F08/F09, T06/T14/T22. `client-api` serves literal IPv4/IPv6 loopback only, with process-random bearer entropy, exact Host/Origin/peer guards on every route, strict bounded JSON and atomic identity-scoped idempotency. An explicit launch-time frozen profile/hash connects independent HTTP controls to the existing finite SSH/UDP/TCP runner; one active run, bounded histories, honest unknown/terminal status and shutdown joins preserve engine ownership. Windows ordinary/race fixtures, Ubuntu/CentOS WSL API and real HTTP/SSH/UDP CLI fixtures and four builds pass. Task 18 supplies saved operations; checkpoint 23b supplies live phase/counters and full acceptance remains pending. [Evidence](WORK_LOG.md#2026-10-11-protected-local-run-api), [source](../internal/localapi/server.go), [operator contract](USAGE.md#protected-local-run-api).

## 18. Connect saved profile/pairing/report API operations (completed)

**Complete for protected saved operations and recorded fixtures.** F01/F07/F08/F09, T06/T14/T22. Safe IDs, bounded catalogs, shared profile validation, explicit revision-checked atomic saves, public pairing replacement/key generation and report replay/import/JSON/JSONL/CSV export are connected. Private admitted plans and accepted start/cancel retries remain unchanged by later saves. Imports validate fixed raw files before new-ID publication and never authenticate offline evidence or resume traffic. Windows ordinary/race and Ubuntu/CentOS WSL fixtures, comment audit and four builds pass. [Evidence](WORK_LOG.md#2026-10-11-saved-configuration-and-report-api), [source](../internal/localapi/saved.go), [operator contract](USAGE.md#protected-local-run-api).

## 19. Build dashboard setup and run controls (completed)

**Complete for embedded controls and recorded HTTP/script/CLI fixtures.** F01/F08/F10, T04/T06/T14/T25. Exact public shell assets preserve API guards; protected setup, public pairing/key controls, registry-driven options, editable ordered phases, strict validation, explicit revision saves and frozen start/cancel/status work through the existing API. Exact decimals, unsaved/conflict states, same-process lost-acknowledgment retries and coalesced status reads pass. Windows tests/vet/build, affected race checks, comment audit and four target builds pass. Current Test dashboard Chrome QA passes under completed checkpoint 23c; richer T25 controls are deferred; live counters are implemented in checkpoint 23b and full acceptance remains pending; task 20 below supplies saved diagnostic views. [Evidence](WORK_LOG.md#2026-10-11-dashboard-setup-and-run-controls), [source](../internal/localapi/dashboard.go), [operator workflow](USAGE.md#dashboard-setup-and-run-controls).

**Task 20 below supplies saved diagnostic/distribution views.** Live counters are implemented in checkpoint 23b; full slice acceptance remains incomplete.

## 20. Add diagnostic and distribution views (completed)

**Complete for saved diagnostic/distribution and local-preflight component implementation and recorded fixtures, 2026-10-12.** F01/F07, T13/T14, basic S08/S09. Protected bounded replay supplies endpoint/main/warmup/phase counts, exact full-population tails, phase boundaries and local fault evidence. Native SVG time/ECDF range groups preserve extrema/counts/missing/local markers; tables paginate and explicit local timer preflight reports lag/missed slots with unverified clock quality. Windows tests/vet/build, affected race checks, actual-script and real HTTP/SSH/UDP fixtures, Ubuntu/CentOS WSL API/CLI checks and four target builds pass. Live client-local phase/counters are implemented in checkpoint 23b; rendered-browser QA, verified clock quality and full acceptance remain pending. [Evidence](WORK_LOG.md#2026-10-12-diagnostic-and-distribution-views), [source](../internal/localapi/diagnostics.go), [operator workflow](USAGE.md#diagnostic-and-distribution-views).

Task 21 below supplies matched saved-run comparisons. Live counters are implemented in checkpoint 23b; full slice acceptance remains incomplete.

## 21. Compare matched saved runs (completed)

**Complete for protected saved comparison/API/dashboard implementation and recorded fixtures, 2026-10-12.** F05/F07, T10/T18/T14. Selected distinct A/B pairs replay under one aggregate read budget; frozen workloads, environment and platform provenance match outside predeclared adapter/port/directional DSCP axes. Exact deadline-first counts/fractions, full-population tails/variation, delivery and shared spike/stall episodes retain main/warmup/phase populations, per-trial deltas, ties and incompatible/incomplete/inconclusive results. Repeated ranges never pool quantiles or select winners. Windows ordinary/race, script, real sequential HTTP/SSH/UDP fixtures, WSL checks and four builds are recorded in [evidence](WORK_LOG.md#2026-10-12-matched-saved-run-comparisons); [source](../internal/control/compare.go), [operator workflow](USAGE.md#compare-matched-saved-runs). Environment/order/outer-path claims, calibrated one-way comparisons, rendered-browser QA and full acceptance remain unverified.

**Task 22 below completes the common adapter extension gate.**

## 22. Complete the adapter extension gate (completed)

**Complete for the common workflow extension gate and recorded fixtures, 2026-10-12.** F08, T06/T14/T21. One startup registry connects configuration/capability validation, authenticated coordination, finite lifecycle, shared workers/accounting and dashboard/report operations. Descriptor frame limits and fixed framing-byte costs replace finite protocol-name dispatch. A test-only TCP-backed adapter retains its distinct identity/options through both-family completion, cancellation, reconciliation, replay/import/JSON/JSONL/CSV export, diagnostics/comparison and actual dashboard script controls. Production defaults remain UDP/TCP. Windows ordinary/race checks and four builds pass; native/later-carrier/outer/live/full acceptance remains pending. [Boundary](USAGE.md#adapter-extension-gate), [source](../internal/adapter/adapter.go), [evidence](WORK_LOG.md#2026-10-12-adapter-extension-gate).

## 23. Validate the Windows x86-64 client to Linux x86-64 CLI workflow (completed)

**Complete for the named Windows 11 x86-64 to Ubuntu 24.04.4 x86-64 WSL2 virtual-network case, 2026-10-12.** F01/F07/F09, T08/T09/T12. [Opt-in real-process fixture](../cmd/sneaky-port/cross_endpoint_windows_test.go), [rerun instructions](USAGE.md#cross-endpoint-cli-validation), [evidence](WORK_LOG.md#2026-10-12-windows-to-linux-cli-validation).

- [x] Fresh endpoint-local keys, public enrollment/import and UDP/TCP exact-plan readiness.
- [x] UDP and TCP finite completion, authenticated server transfer and compatible reconciliation.
- [x] Real Windows console interrupt during UDP/TCP data; bounded nonzero exit and saved partial endpoint reports, with remote delivery unknown.
- [x] Fresh CLI replay and JSON/JSONL/CSV export on both endpoints, saved-summary/identity/platform checks.
- [x] Client reservations, server data ports and owned SSH server released; sandbox access resolved through narrow review without changing user services/settings.

This dated case does not establish bare-metal Linux, Linux minima, outer markings, production clock bounds, impairment or live ISP acceptance. The current x86-64 validation scope and named controlled cases are recorded below.

## Dashboard completion checkpoints

**Selected together by the user, 2026-10-12.** Implement the remaining dashboard checkpoints sequentially. The 2026-10-13 shipping decision is Windows/Linux x86-64; tasks 24-25 are retired.

### 23a. Add shared per-metric evidence presentation (completed)

**Complete for shared derived presentation and recorded component/script fixtures, 2026-10-12.** Accepted S06: derived versioned policy, bounded independent coverage/basis/count/reason labels, explicit presentation preferences, API/dashboard/replay/export consistency and unchanged measured values/validity gates.

### 23b. Add live client-local phase and counters (completed)

**Complete for common engine/API/dashboard progress and recorded component/script fixtures, 2026-10-12.** F01/S09: run-owned coalesced snapshots, independent accounting/publication, warmup/main separation, strict deadline/cutoff handling, bounded cadence, cancellation/status recovery and provisional UI.

[Checkpoint evidence](WORK_LOG.md#2026-10-12-dashboard-evidence-and-live-progress), [source](../internal/measure/evidence.go), [presentation preferences](USAGE.md#evidence-presentation), [live workflow](USAGE.md#live-run-progress).

### 23d. Replace the stacked dashboard with section navigation (completed)

**Complete for embedded implementation and functional/static fixtures, 2026-10-12.** User-approved fixed left sidebar, section-specific vertical subtabs and one visible view; paginated fields/tables/full details, clearer labels and phase timing/upstream/downstream tabs. Switching preserves malformed edits, frozen runs and retry identities without API requests. [Evidence](WORK_LOG.md#2026-10-12-dashboard-section-navigation), [source](../internal/localapi/web/navigation.js). Rendered-browser evidence remains checkpoint 23c.

### 23e. Open directly, connect over Linux SSH and run the edited plan (completed)

**Complete for implementation and recorded HTTP/script/SSH/UDP/TCP fixtures, 2026-10-12.** User-selected automatic local access, normal Linux SSH IP/port/password login (editable default username `sneak`), explicit host verification, disposable matching helper and Run Test validation/save/exact settings negotiation. Passwords remain transient; keyed CLI, original stores/services and state/retry/accounting boundaries remain intact. The initial PATH/manual-first-trust setup is superseded by checkpoints 23g/23h below; SSH control forwarding remains required. [Evidence](WORK_LOG.md#2026-10-12-dashboard-password-ssh-workflow), [source](../internal/control/ssh_dashboard.go). Rendered-browser/native sshd/deployment acceptance remain separate.

### 23f. Resolve copied server executables and preserve accepted SSH trust (completed)

**Complete for implementation and recorded script/SSH/native Linux shell/copied-helper fixtures, 2026-10-12.** Initial optional absolute executable selection (now a legacy API branch; the dashboard uses 23g), fixed startup diagnostics and authenticated host trust retained across helper failures. Real supplied Linux executable startup and negotiated direct measurement/report transfer pass under the configured account in isolated Ubuntu WSL tests. [Evidence](WORK_LOG.md#2026-10-12-copied-helper-startup-repair), [source](../internal/control/helper_launch.go). No deployment/permission changes or real sshd/browser/full-network acceptance.

<a id="23c-verify-the-rendered-dashboard"></a>

### 23c. Verify the rendered dashboard (completed)

Checkpoints 23g/23h were authorized together by the user on 2026-10-12 and are complete within their recorded implementation/fixture scopes. Shipping scope is Windows/Linux x86-64.

**Complete for the current Test dashboard in connected Chrome, 2026-10-13.** Fresh packaged client and matching-code owned loopback fixtures verify conditional settings, exact decimal edits, keyboard focus/dialog dismissal, 390/768-pixel layouts, live progress, UDP/TCP final cards/chart, active-run reload recovery, partial cancellation, preflight, saved-name conflicts and disconnect cleanup. Actual password/key SSH, carriers and reporting run; Linux upload/shell is modeled. Removed legacy comparison views and native sshd/live/clock-accuracy acceptance are separate. [Current evidence](WORK_LOG.md#2026-10-13-reference-runtime-and-rendered-browser-progress).

### 23g. Upload and start the bundled Linux helper (completed)

**Complete for implementation and recorded packaged/SSH/native Linux fixtures, 2026-10-12.** Self-contained Windows/Linux x86-64 dashboards bundle the matching helper; Connect transfers exact bytes, verifies SHA-256 and starts it in private temporary storage. Short/corrupt/stalled uploads, fixed platform/tool/storage failures, real native uploaded-helper/direct run/report transfer and cleanup pass. [Evidence](WORK_LOG.md#2026-10-12-managed-helper-and-top-connection-bar), [source](../build-dashboard.ps1). Existing server files/services remain intact; actual system sshd/live/full acceptance remains separate.

### 23h. Top connection bar and automatic first fingerprint trust (completed)

**Complete for implementation and recorded script/HTTP/SSH fixtures, 2026-10-12.** Persistent top IP/SSH port/username/password/status controls, four measurement sidebar sections, public Details dialog and inline changed-key replacement. First authenticated fingerprints save automatically; changed saved keys stop before password delivery. Actual script checks password clearing, absent path/sidebar controls, navigation/edit retention, replacement fields and lost acknowledgments; real SSH fixtures check first/repeated/changed trust and exact UDP/TCP agreement. [Evidence](WORK_LOG.md#2026-10-12-managed-helper-and-top-connection-bar), [source](../build-dashboard.ps1). Rendered-browser acceptance remains 23c.

### 23i. Make test setup usable without IDs or JSON (completed)

**Complete for implementation and recorded actual-script/real HTTP/SSH/UDP/TCP fixtures, 2026-10-12.** Compact normal workflow with named saved tests, real registered protocol controls, seconds/milliseconds, payload/rate/directional-priority inputs and direct Run/Stop/Clear actions. Manage internal identifiers automatically; retain exact advanced settings behind Advanced without silent rewriting. Decimal conversion, loaded/custom plan retention, explicit TCP options, visible automatic storage and exact saved/negotiated revisions pass. [Evidence](WORK_LOG.md#2026-10-12-dashboard-usability-and-automatic-results), [source](../internal/localapi/web/workspace.js).

### 23j. Show automatic readable test tracking and results (completed)

**Complete for implementation and recorded script/HTTP/SSH/native component fixtures; authorized with 23i, 2026-10-12.** Metric cards and charts on the main Test view, honest provisional/final/partial/unavailable states, automatic terminal report loading, friendly report selectors and recovery without pasting run/report IDs. Preserve complete technical evidence/exports under Advanced. Real guided UDP/TCP completion, exact automatic final-result loading, named declared-axis comparisons, same-request lost-acknowledgment recovery, per-metric notes, active snapshot and bounded label guards pass. [Evidence](WORK_LOG.md#2026-10-12-dashboard-usability-and-automatic-results), [source](../internal/localapi/web/workspace.js). Rendered/native sshd/live acceptance remains separate. Screenshot router shaping and mixed-carrier flood scenarios are outside this UI task.

### 23k. Put conditional test settings on one main page (completed)

**Complete for embedded implementation and actual-script/HTTP/SSH/direct UDP/TCP fixtures, 2026-10-12.** Latest user request supersedes 23d/23i sidebar/Advanced navigation: no sidebar columns; all supported test settings expand inline through typed groups. TCP checkbox, traffic pattern/priority/port/payload choices reveal relevant controls; custom phases, limits, calibration and scalar comparison metadata remain editable without JSON. Group changes preserve invalid inputs and send no requests. Two-phase burst tests verify exact settings/deadline negotiation, completion/replay/comparison and same-request recovery. [Evidence](WORK_LOG.md#2026-10-12-single-page-conditional-dashboard-settings), [source](../internal/localapi/web/index.html). Chrome QA 23c is complete; native sshd/live acceptance remains separate.

### 23l. Keep only the Test dashboard (completed)

**Complete for implementation and actual-script/HTTP fixtures, user-selected 2026-10-12.** Removed Saved results/Compare sections, navigation, script loading and comparison settings. Test retains conditional controls, live/current results, local timer and retry/status/Stop. Stored evidence/protected APIs remain. Absent-section, retained-edit, exact-setting and current-result fixtures pass. [Evidence](WORK_LOG.md#2026-10-12-test-only-dashboard-and-sixty-second-repair), [source](../internal/localapi/web/workspace.js).

### 23m. Repair sixty-second dashboard runs (completed)

**Complete for implementation and real elapsed 60-second UDP/TCP loopback fixtures; authorized with 23l, 2026-10-12.** Reproduced default 129-demand/128-cap startup failure; automatic resources now cover heartbeat/calibration/report transfer, with explicit manual recovery. Independent bounded evidence buffering preserves physical send queues. Current terminal client replay uses a 64-MiB cap, exact owned plan/measurement binding and 30-second HTTP / 35-second browser budgets; ordinary saved caps remain. Both endpoints completed both 60-second protocols and current charts/replay loaded (153.59-second fixture). [Evidence](WORK_LOG.md#2026-10-12-test-only-dashboard-and-sixty-second-repair), [source](../internal/localapi/web/workspace.js). Browser/native sshd/live/full acceptance remains separate.

### 23n. Replace receipt-count plotting with live latency (completed)

**Complete for implementation, actual-script/API and real finite UDP/TCP loopback fixtures; user-selected 2026-10-13.** Removed the count chart and show on-time/late/pending/missed outcomes numerically. One chart displays client attempt-to-receipt RTT in ms versus elapsed seconds, using copied main-only one-second min/max/count ranges bounded to 128 recent buckets; strict cutoff/warmup/duplicate/absent-receipt rules remain. Final replay replaces live ranges. Owned-source test/vet/build, focused race and rebuilt matching-helper checks pass. [Evidence](WORK_LOG.md#2026-10-13-numeric-receipts-and-live-latency), [source](../internal/measure/progress.go). Chrome QA 23c is complete; remaining native/clock/reference/live gates are recorded in task 35.

### 23o. Use built-in server defaults and port 22022

**Implemented; Linux runtime validation deferred by the user, 2026-10-15.** Linux no-argument startup (or `server`) opens app SSH on 22022 and initializes private state automatically. Compiled limits are shared by standalone initialization and the managed helper; client profiles still determine tests after existing keyed authentication. Existing identities, authorizations and explicit bindings/overrides remain unchanged. Windows ordinary/vet/focused race checks and Linux cross-build pass; native/WSL runtime acceptance is not complete. The user explicitly stopped Linux testing for this change. [Evidence](WORK_LOG.md#2026-10-15-built-in-server-defaults-and-port-22022), [source](../cmd/sneaky-port/server_start.go), [operator startup](USAGE.md#start-the-standalone-linux-server).

### 23p. Start clients with an optional profile file (completed)

**Complete for shared configuration loading and Windows startup checks, 2026-10-15.** Omitted `-profile` uses a compiled editable UDP starter for dashboard startup, validation, readiness and finite runs. Explicit overrides remain strict; saved dashboard edits retain precedence for the selected profile ID. Full Windows owned-source ordinary/vet, focused race, packaged no-profile API startup, both dashboard builds and helper equality pass. Linux runtime checks remain user-deferred; builds do not qualify those gates. [Defaults](USAGE.md#default-configuration), [source](../internal/profile/defaults.go), [checks and limits](WORK_LOG.md#2026-10-15-optional-profile-and-built-in-client-settings).

The user subsequently removed the shared sample folder; tests and archives now use the compiled defaults. Optional local overrides remain supported. [Cleanup checks](WORK_LOG.md#2026-10-15-sample-configuration-folder-removed).

### 23q. Start the Windows client without arguments (completed)

**Complete for Windows client startup, 2026-10-15.** Windows no-command/option-only startup selects the dashboard with a default per-user store. Explicit commands/data directories remain supported; Linux retains its server role. Affected Windows ordinary/vet/comment checks, focused race, real packaged zero-argument startup/protected setup/default-directory creation, both platform builds and helper equality pass. Linux runtime remains user-deferred. [Startup settings](USAGE.md#default-configuration), [evidence](WORK_LOG.md#2026-10-15-zero-argument-windows-client-startup).

## Queued validation tasks

Statuses below distinguish completed named cases, partial evidence and remaining prerequisites. Use [acceptance gates](DESIGN.md#validation-before-supportperformance-claims), the [reference matrix](DESIGN.md#reference-comparison-acceptance-matrix) and [resolved release baselines/policies](DESIGN.md#current-udptcp-release-decisions). Record one named environment/scenario at a time; split multi-case work into explicit checklist items before execution. Build, loopback, native, impairment and live evidence stay distinct. These tasks do not authorize changing user-owned settings or substituting new reference dependencies. The user authorized the remaining release validation on 2026-10-13; Windows/Linux x86-64 are the shipping targets.

**Dashboard checkpoint 23a:** implements the accepted [per-metric evidence label policy](#s06-separate-connection-quality-from-measurement-confidence) through shared derived reports/API/dashboard/exports, retaining measured values and mandatory gates. Verify sample/repetition boundaries, partial/missing/clock/marking independence, replay consistency, bounded reasons and secret-free output. The user selected this implementation with live progress on 2026-10-12; checks/evidence are recorded under dashboard completion below. Browser QA and remaining acceptance still gate task 35.

| Task | Bounded outcome | Completion check |
| --- | --- | --- |
| 24 | Retired client architecture case | Removed from shipping scope by user, 2026-10-13; ID retained for history |
| 25 | Retired server architecture case | Removed from shipping scope by user, 2026-10-13; ID retained for history |
| 26 | Establish a Linux support-baseline case | Start with the selected Ubuntu 22.04.5 LTS server case; record actual OS/kernel/native workflow and qualify the supported minimum only from evidence; additional claims get separate cases |
| 27 | Native directional marking and port evidence | Partial: guest ens33 and Windows Npcap Wi-Fi IPv4 observations pass for both carriers; server DSCP12/client DSCP0 observed. Windows DSCP46 requests reject at qWave. Remaining privilege/NAT/IPv6/observation limits stay explicit |
| 28 | Validate clock bounds and directional delay | Independently justified timing/error bounds and controlled offset/drift/asymmetry cases support or limit one-way classifications |
| 29 | Controlled delay/spike case | [Complete for named kernel and application-boundary cases](#task-29-controlled-delays-and-spikes-completed) |
| 30 | Controlled loss/delivery case | [Complete for exact seeded/burst data and separate receipt drops](#task-30-controlled-data-and-receipt-loss-completed) |
| 31 | Controlled duplicate/reorder case | [Complete for authenticated known-identity UDP cases](#task-31-duplicates-reordering-and-invalid-replay-completed) |
| 32 | Stream blockage, MTU and interruption cases | [Complete for named local stream/kernel/cancellation cases](#task-32-stream-mtu-rate-and-interruption-cases-completed) |
| 33 | Independent reference overlap cases | Partial: pinned artifacts/licenses, native iperf2 build/smokes and four isolated Crusader scenarios retained. Ten repaired-clock IRTT alias pairs retain 200 product/199 reference completed samples; independent boundary allowances, 2 held-out pairs and frozen bands remain |
| 34 | Matched live-path case | Named on-link VMware subcase passes three alternating port pairs per carrier, including actual local skips; broader path/load/uncertainty qualification remains pending |
| 35 | Review first-slice acceptance evidence | [Reviewed; full acceptance incomplete](#task-35-current-acceptance-review) until 26-28/33-34 have their required evidence |

F10's [protocol traffic controls](DESIGN.md#protocol-specific-traffic-controls) retain task 19's common editor and fixed version-1 framing for this release. T25's richer fragmentation/header/padding mechanisms are deferred beyond it; future work must first select a compatible adapter/layer/schema outcome. The accepted wider F10 intent is preserved.

<a id="task-24-native-windows-11-arm64-client"></a>

### Tasks 24-25: retired architecture cases

**Removed from release requirements by the user, 2026-10-13.** Windows/Linux x86-64 are the shipping targets. Stable IDs remain reserved; these cases are retired, not validated or marked completed. [Original checklist and prerequisite evidence](ARCHIVE.md#retired-native-arm-acceptance-checklist).

Later carriers, DNS and monitoring keep their existing future status in [feature proposals](#future-features-and-proposals). No release date or acceptance result is inferred from this breakdown.

### Task 26a: Ubuntu 24.04.4 VMware workflow (completed)

**Complete for the named guest/runtime case, 2026-10-15.** Windows 11 x86-64 connects through the guest's real OpenSSH daemon, uploads/verifies the matching helper and runs direct UDP/TCP. Fourteen completed and two cancelled runs retain native Linux and Windows replay/three-format exports; helper/upload/capture staging cleanup passes. All 16 owned test packages pass as cross-built Go processes on Ubuntu 24.04.4, kernel 6.17.0-14-generic, ordinary UID 1000. Native compiler/race and selected Ubuntu 22.04.5 qualification remain separate. [Evidence](WORK_LOG.md#2026-10-15-vmware-deployment-and-network-validation).

### Task 27a: IPv4 guest and Windows endpoint captures (completed)

**Complete for the named observation points, 2026-10-15.** Guest ens33 retains 2,768 packets; Windows Npcap Wi-Fi retains 5,536 records, including 2,768 identical capture records. Both report zero capture drops; raw duplicates are retained separately from network-duplication claims. Run-associated main data show server DSCP12/client DSCP0 and ECN0 for UDP/TCP at both endpoints. All 1,084 unique UDP payload/endpoint tuples match across captures. No radio-wire, NAT, global IPv6, checksum/offload or packet-loss claim. Windows nonzero DSCP46 rejects before measurement at qWave with access denied. [Evidence](WORK_LOG.md#2026-10-15-vmware-deployment-and-network-validation).

The existing four independent AF_PACKET UDP/TCP IPv4/IPv6 loopback cases additionally pass on the VMware kernel inside a guarded disposable namespace: directional DSCP46/12, UDP ECN3/TCP ECN0 and zero capture drops. This supplements endpoint captures without claiming routed IPv6 or nonzero Windows support.

### Task 34a: On-link matched port trials (completed)

**Complete for the named functional comparison case, 2026-10-15.** Three A/B pairs per UDP/TCP carrier alternate order over two selected ports, with 1.5-second main phases at 20 messages/s independently in each direction and 200-ms warmup/grace. All six pairs produce descriptive compatible comparisons; raw local skips remain visible and no winner or precision claim is derived. Each main phase plans 30 slots; two completed client trials retain one local skip each. The larger first group remains retained after correctly exceeding the comparison API's 8 MiB aggregate budget; the smaller case changes only declared fixture workload. This on-link IPv4 path has no controlled external load or Internet/ISP qualification. [Evidence](WORK_LOG.md#2026-10-15-vmware-deployment-and-network-validation).

### Task 29: controlled delays and spikes (completed)

**Complete for named controlled cases, 2026-10-13.** Actual Linux IPv4 UDP/TCP measurement-port filters apply 10/20-ms directional kernel delay, 4/6-ms jitter and two upstream 80-ms spike intervals. SSH/control stays outside these filters. A separate Windows real-UDP receive fixture retains exact 9/11/13-ms release floors and two 140-ms spikes. Authenticated receiver evidence, exact replay/exports, completed/late samples and actual command intervals are retained. This is loopback/WSL controlled evidence, with no physical-path or calibrated one-way claim. [Evidence](WORK_LOG.md#2026-10-13-controlled-validation-and-x86-64-release-scope), [source](../internal/validation/controlled_test.go).

### Task 30: controlled data and receipt loss (completed)

**Complete for the known-identity application-delivery case, 2026-10-13.** Real UDP plus production authenticated report transfer uses frozen seed 20261013: drop data 0/2/3/8 (including burst 2/3), separately drop receipts 1/4. Twelve written upstream data yield eight receiver arrivals, four missing application data, six unresolved origin receipts and six RTT samples; downstream remains independently complete. Logical receive-boundary drops and kernel interruption counters are distinct from inferred IP packet loss. Raw reports/truth and exact replay/export are retained. [Evidence](WORK_LOG.md#2026-10-13-controlled-validation-and-x86-64-release-scope).

### Task 31: duplicates, reordering and invalid replay (completed)

**Complete for named authenticated UDP cases, 2026-10-13.** Two real runs each retain 12 unique data, one duplicate datum/receipt and sequence 2 arriving before 1. Malformed/tampered-foreign frames are rejected; the second run additionally rejects actual authenticated bytes from the prior run. Counts/identity/sample populations, production receiver reconciliation and saved replay/exports match exactly. [Evidence](WORK_LOG.md#2026-10-13-controlled-validation-and-x86-64-release-scope).

The full-population reports above are retained named cases. Current CI separates deterministic complete fault populations from real socket runs with exact actual scheduling/cutoff outcomes; local skips do not imply network loss or full fault coverage. [Fixture repair and validation](WORK_LOG.md#2026-10-14-ci-controlled-population-fixture-repair), [source boundaries](../internal/validation/controlled_test.go).

### Task 32: stream, MTU, rate and interruption cases (completed)

**Complete for named local cases, 2026-10-13.** Both real carriers run with interface MTU 1200/content 1400, directional 128-kbit/s kernel rate limits and two 150-ms upstream 100-percent interruption intervals. Real stream stall/recovery and partial-write fixtures pass; ten UDP/TCP warmup/main/grace/control-loss/revocation cases retain 20 secret-free partial endpoint reports and unknown missing remote delivery. Qdisc backlog is recorded, then discarded in the owned namespace before readiness/reuse checks. This does not establish physical path MTU, fragmentation captures, sustained deployment or underlying TCP packet loss. [Evidence](WORK_LOG.md#2026-10-13-controlled-validation-and-x86-64-release-scope), [source](../internal/validation/controlled_test.go).

The 12 isolated kernel cases covering tasks 29/32 also pass on Ubuntu 24.04.4 VMware, kernel 6.17.0-14-generic, 2026-10-15. Both launcher and fixture verify a network namespace different from PID 1 with only loopback before any mutation; raw public reports and qdisc/interface truth are retained. [Native evidence](WORK_LOG.md#2026-10-15-vmware-deployment-and-network-validation).

### Task 35: current acceptance review

**Reviewed, incomplete, 2026-10-15.** The remaining gates are concrete evidence prerequisites, not unresolved routine choices:

| Gate | Current state / remaining prerequisite |
| --- | --- |
| 23c browser | Completed for the current Test dashboard in connected Chrome above. Native Linux shell/sshd, clock accuracy and live-network qualification remain separate. |
| 26 Linux baseline | Ubuntu 24.04.4 VMware runtime/deployment subcase 26a passes. Selected Ubuntu 22.04.5 qualification and native compiler/race remain unrun; no blanket Linux minimum is claimed. |
| 27 marking/ports | Native IPv4 endpoint capture subcase 27a passes using existing Npcap/tcpdump. Windows qWave nonzero DSCP is denied in the available session; NAT/global-IPv6 and remaining observation/privilege cases remain unqualified. |
| 28 clocks | Synthetic/causal gates and coherent QPC elapsed-source repair pass focused checks. Independent observations confirm the former Go timestamp plateaus. Production timestamp/model/drift bounds remain unqualified; one-way stays estimated/unavailable. |
| 29-32 controlled cases | Completed in their named scopes above; recorded results do not imply native deployment or live ISP evidence. |
| 33 references | Pinned iperf2 native build/smokes retain pacing warnings; four isolated Crusader scenarios complete. Supported IRTT loopback alias fixes its MTU receive-buffer failure: ten exploratory pairs retain 200 product/199 reference completions and one reference skip. Independent boundary allowances and valid baseline/held-out trials remain incomplete. |
| 34 live path | On-link VMware subcase 34a passes with actual per-trial workload, captures and descriptive matched comparisons. Internet/ISP, controlled external contention and broader timing/uncertainty qualification remain unrun. |
| Shipping architecture | Windows/Linux x86-64 only. Retired tasks 24-25 are absent from acceptance requirements. |

The full first slice remains incomplete. Further work resumes at these prerequisites; later carriers, DNS and monitoring keep their existing status.

## Future features and proposals

Recorded: 2026-10-10.

This section tracks future features, optional ideas, decisions and progress. [WORK_LOG.md](WORK_LOG.md) tracks backward: changes actually made, validation, and current implemented behavior. Product details remain in [DESIGN.md](DESIGN.md).

Accepted features reflect confirmed scope. S07-S11 were accepted for future scope on 2026-10-11. On 2026-10-12 the user limited resolution to the current UDP/TCP release: S06 design is accepted; S01-S05/S12 are deferred beyond it and remain future proposals. User explicitly authorized the agreed first slice on 2026-10-11; unrelated acceptance/plans do not expand execution scope.

Finite UDP/TCP lifecycle/accounting, bounded evidence/replay/export, authenticated reconciliation, one-way estimates and protected configuration/run/report API exist; dashboard setup/run, saved diagnostic/distribution and matched saved-comparison views exist; live client-local counters and S06 evidence presentation are implemented; current Test dashboard Chrome QA passes; full acceptance remains pending. [Saved API evidence](WORK_LOG.md#2026-10-11-saved-configuration-and-report-api), [source](../cmd/sneaky-port/main.go).

### Confirmed future features

This index tracks scope and partial progress, not a release schedule. [ROADMAP.md](#first-slice-task-index) splits the former umbrella into separately checked tasks; milestones 5-23 and dashboard implementation checkpoints pass within recorded scopes. Latest 23q adds zero-argument Windows startup; 23p provides optional profiles, 23n numeric receipts/live RTT, and 23l/23m the Test-only workflow/long-run repair. Chrome QA 23c and named controlled cases 29-32 pass. Shipping is Windows/Linux x86-64; the user retired tasks 24/25 on 2026-10-13. Production clock bounds and remaining release gates stay incomplete; no F item is complete.

| ID | Feature | Status | Design reference |
| --- | --- | --- | --- |
| F01 | Windows 11 clients, one Linux server across major distributions including CentOS/Ubuntu/Fedora/Arch, x86-64 on both, managed endpoint/session lifecycle | Partial: pairing/SSH/finite CLI, protected API/dashboard, live counters and evidence readers implemented; named WSL and [Ubuntu 24.04.4 VMware deployment/runtime](WORK_LOG.md#2026-10-15-vmware-deployment-and-network-validation) cases pass. [Built-in server defaults/app port 22022](WORK_LOG.md#2026-10-15-built-in-server-defaults-and-port-22022) implemented; its Linux runtime checks are user-deferred. Selected baseline/minima and full acceptance remain pending | [Scope](DESIGN.md#confirmed-intent-and-scope) |
| F02 | Real TCP, UDP, TLS, and QUIC stream/datagram measurements | Partial: UDP/TCP readiness, queues and shared UDP/TCP phase/accounting pass Windows/WSL fixtures; finite CLI lifecycle implemented; TLS/QUIC later | [Protocols](DESIGN.md#authentic-protocol-coverage) |
| F03 | Tool-managed WireGuard and Hysteria2 with inner UDP/TCP test modes | Accepted; not started | [Protocols](DESIGN.md#authentic-protocol-coverage) |
| F04 | DNS port controls, direct and recursive data paths, transport/downlink experiments | Accepted; not started | [DNS](DESIGN.md#dns-experiments) |
| F05 | Server/destination ports, optional source ports, selected-port comparisons | Partial: socket policy/bindings and saved-port comparisons implemented; three alternating port pairs per UDP/TCP carrier and matching endpoint capture tuples pass on the VMware on-link path. NAT and broader path qualification remain pending | [Ports](DESIGN.md#port-selection) |
| F06 | Independent directional packet priority with outer-packet evidence | Partial: phase-owned markings and native Linux IPv4/IPv6 DSCP/ECN capture pass. Guest ens33/Windows Npcap observe server DSCP12/client DSCP0 for real UDP/TCP. Windows DSCP46 explicitly rejects at qWave in the available session; privileged nonzero Windows and remaining observation cases remain unqualified | [Priority](DESIGN.md#independent-packet-priority) |
| F07 | FPS workloads, correct timing/delivery metrics, reproducible reports, evidence-based comparisons | Partial: UDP/TCP phases, deadline/cutoff/ticket accounting, bounded local observations and RTT/variation/spike/stall analysis; bounded local files/replay/exports and authenticated remote reconciliation and descriptive matched saved comparisons implemented | [Measurements](DESIGN.md#timing-and-scheduling-correctness) |
| F08 | Extensible protocol adapters with shared measurement, capability-driven configuration/dashboard, and compatible results | Partial: startup registry/descriptor/factory and test-only common configuration/control/engine/accounting/dashboard/report/cleanup gate pass ([evidence](WORK_LOG.md#2026-10-12-adapter-extension-gate)); future authentic carriers/capabilities and full acceptance pending | [Extensibility](DESIGN.md#protocol-extensibility) |
| F09 | Authenticated run-plan coordination and maintained clock calibration for directional one-way measurements with uncertainty | Partial: trust/readiness/leases, finite lifecycle, authenticated reconciliation, live SSH captures and causal one-way intervals/replay/export implemented. Synthetic envelope gates pass; independently verified production bounds and clock-change/native/live acceptance pending | [Control and calibration](DESIGN.md#control-connection-and-clock-calibration) |
| F10 | Capability-driven dashboard traffic tweaks, including supported fragmentation and header/framing/payload size controls | Partial: payload and common dashboard editor/registered options implemented; richer fragmentation/header controls and schema/mechanisms pending ([evidence](WORK_LOG.md#2026-10-11-protocol-traffic-tweak-requirement)) | [Traffic controls](DESIGN.md#protocol-specific-traffic-controls) |

Keep implementation details, validation coverage, and completed changes in the work log. When progress begins, annotate this index with its actual state and a reference to the relevant work-log entry. Partial delivery of an item does not complete all its protocol/platform variants.

### Technical choice progress

Current scope/decisions remain in [DESIGN.md](DESIGN.md#first-slice-implementation-contract) and [technical choices](DESIGN.md#first-slice-final-decisions). UDP/TCP portions of F01/F02/F05-F09, basic S08/S09 and S07/S10 gates are selected; S11/other carriers/DNS remain later. Foundation checks are partial; no full runtime gate is complete.

Detailed options/provenance belong in [technical choices](DESIGN.md#technical-choices); [WORK_LOG.md](ARCHIVE.md#2026-10-11-first-implementation-foundations) retains actual implementation/evidence. Next actions are listed above. Do not promote partial features to complete without their acceptance evidence.

### Suggestions and accepted additions

S01-S05/S12 are deferred beyond the current release; S06 release design and derived presentation are implemented, with stable IDs preserved. S07-S11 were accepted together on 2026-10-11; S08/S09 have local preflight and saved diagnostic/distribution views; live client-local counters implemented; current Test dashboard Chrome QA passes; verified clock quality and other additions remain pending. Acceptance alone did not authorize execution; the later first-slice instruction supplies that authorization.

#### Reference review dispositions

Reviewed/answered: 2026-10-11, action 3b. User confirmed IRTT/iperf2/Crusader as design/external-validation references and accepted S07-S11. [Dispositions](ARCHIVE.md#selective-source-review-and-dispositions), [acceptance matrix](DESIGN.md#reference-comparison-acceptance-matrix), [historical review](ARCHIVE.md#2026-10-11-selective-reference-review-completed). Current partial implementation is indexed above.

| ID | Recommendation for the first-slice discussion | Decision / implementation state |
| --- | --- | --- |
| S07 | First-slice external cross-check gates: equivalent IRTT UDP, iperf2 receiver/TCP and Crusader path/load overlaps; exact builds/tolerances established before later execution. No product dependency. | Partial: artifact/license/version-help and inconclusive IRTT attempts recorded; compatible measured bands/held-out comparisons incomplete |
| S08 | First-slice optional local scheduler preflight and evidence-based diagnostics; explicit duration/interval, no network data or OS tuning in the local preflight. | Partial: explicit CLI/dashboard local preflight, endpoint fault/local-miss counts and partial-stream/accounting evidence exist; clock source/resolution verification remains pending |
| S09 | First-slice native SVG time/ECDF views and phase tables, exact final quantiles with bounded/labeled rendering and coalesced live updates. | Partial: bounded SVG time/ECDF, exact phase/quantile/count and missing/local views exist; live client-local counters implemented; current Test dashboard Chrome QA passes under [23c](#23c-verify-the-rendered-dashboard); legacy removed views are outside that scope |
| S10 | First-slice external idle/upload/download/both-load validation gates; fixtures remain outside product, with resource/isolation/restoration and matched-workload evidence. | Partial prerequisites: isolated kernel impairment/MTU/rate cases executed; external contention/load reference cases remain pending |
| S11 | Explicit continuous monitoring retained for a later slice; retention/windows/reconnect/eligibility remain future choices. | Accepted future scope by user, 2026-10-11; outside first slice, not implemented |
| S01-S06 | Current release disposition: accept S06 per-metric evidence design; defer S01-S05 beyond this release. | Engineering selections under the 2026-10-12 scoped resolution instruction; S06 implemented within recorded component fixtures; future proposals retained |

T19/T20 reference release baselines and band-establishment procedure are selected in [current release decisions](DESIGN.md#current-udptcp-release-decisions). Pinned source/license and IRTT/Crusader native version-help evidence is recorded; remaining compatible builds, measured bands and held-out comparisons remain pending. Basic S08/S09 and S07/S10 gates are settled in the first-slice contract; S11 remains later. Product dependency pins are separate from benchmark pins, and scoped fixture evidence does not fulfill full runtime acceptance.

### S01: Staged experiment plan

- [ ] Deferred beyond the current UDP/TCP release, 2026-10-12; future proposal retained, not implemented.

Current release disposition: keep explicit manually selected matched trials. Automatic screening and candidate promotion require a later budget/stopping policy. Milestone suggestion, 2026-10-12: S06 evidence labels and client-local progress now make later bounded S01 screening easier to audit across many candidates; budgets, stopping/revisit rules and repeated confirmation still need decisions. S01 remains proposed/deferred, with no adoption or implementation.

First check protocol reachability, then compare selected ports and directional priority settings, then run longer confirmation tests on promising combinations. Keep every attempted configuration and failure visible.

Purpose: make the large protocol/DNS/port/priority matrix manageable without selecting a winner from a short trial.

Before adoption: define screening rules, budgets, repeated confirmation trials, and how inconclusive or initially rejected cases can be revisited. Record which combinations were actually tested; screening must not imply exhaustive coverage.

### S02: Burst-loss and consecutive-deadline-miss metrics

- [ ] Deferred beyond the current UDP/TCP release, 2026-10-12; future proposal retained, not implemented.

Current release disposition: retain existing deadline counts, gaps, spike/stall episodes and missing/local plot markers. Additional burst-loss/consecutive-miss formulas add a new metric contract and are deferred; no current-release adoption is inferred.

Report consecutive failures, delivery-interruption duration, event frequency, and spacing between failure bursts, alongside the overall loss/deadline-miss percentage.

Purpose: expose clustered interruptions that averages and total loss percentages can hide. Loss distribution matters to real-time delivery; [RFC 3357](https://www.rfc-editor.org/rfc/rfc3357.html) describes loss-pattern metrics.

Before adoption: define burst boundaries and denominators. Distinguish reconciled loss, late delivery, missed transaction deadlines, and local unsent probes. Stream deadline failures must not be relabeled as proven network packet loss.

Task 20 milestone suggestion (still proposed): S02 sequence-based interruption metrics could quantify clusters now visible in missing/local plot markers. The replay ledger and diagnostic entry point provide inputs; first define burst boundaries, populations and treatment of late/local-unsent/partial evidence, then validate synthetic sequences. S02 is not accepted or implemented.

### S03: Separate coordination and measured traffic

2026-10-11 clarification: basic run-plan coordination and maintained one-way clock calibration are now required under F09. Tasks 12-14 now supply baseline endpoint persistence, local replay/export and authenticated post-stop transfer/reconciliation under F07/F09. Additional coordination minimization and recovery behavior remain proposed.

- [ ] Deferred beyond the current UDP/TCP release, 2026-10-12; future proposal retained, not implemented.

Current release disposition: retain bounded endpoint persistence, post-stop authenticated transfer, offline import/replay and explicit incomplete remote evidence. No old-run retrieval, reconnect/resume or automatic deletion is added. Manual retained-file import establishes offline consistency only; recovery authorization/retention mechanics are future work.

Exchange test plans before probing and collect server records afterward. Keep coordination traffic minimal during measurement, and persist evidence on both endpoints when the connection fails.

Purpose: preserve enough evidence to understand failed tests while limiting interference from test management.

Before adopting additional S03 behavior: define recovery/import/retention policy and verify control-traffic minimization; baseline authenticated coordination, bounded local persistence, replay/export and post-stop reconciliation exist. Failure of the coordination path must not automatically classify every measurement protocol as unusable. Missing remote evidence must remain explicitly incomplete.

Task 14 milestone suggestion (still proposed): bounded manual authenticated recovery/import could make retained endpoint reports usable after SSH interruption. The transfer schema and replay checks now provide prerequisites. Before accepting S03 recovery, decide who can retrieve old runs, authorization expiry, input/file budgets and retention; validate stale/mismatched/partial evidence. No recovery implementation is authorized by this suggestion.

Task 15 milestone suggestion (still proposed): audit S03 coordination overhead across explicit clock cadences. Separate calibration byte/cadence/serialization/send-call records now allow measuring its workload and SSH blockage alongside RTT. Before accepting minimization, choose matched trial profiles/repetitions and predeclared comparison bands; preserve required clock coverage/expiry and validate actual SSH/network overhead separately. Do not silently reduce sampling or claim that application bytes measure outer traffic.

### S04: Packet-size and packet-rate sweeps

- [ ] Deferred beyond the current UDP/TCP release, 2026-10-12; future proposal retained, not implemented.

Current release disposition: keep explicit workload/rate/size inputs and manually repeated runs; automatic sweep order, stopping and run budgets are deferred.

Test a range of application payload sizes and offered packet rates to identify where each mode starts degrading. Include asymmetric bidirectional workloads and DNS polling/encoding costs.

Purpose: establish a useful operating range rather than only a favorable latency at one light workload. In particular, isolated small DNS exchanges do not establish sustained tunnel suitability.

Before adoption: define matched profiles, execution order, repetition, and stopping criteria. Record payload, actual offered rate, wire overhead, scheduling limits, and deadline outcomes. These are measurement workloads, not product traffic shaping.

### S05: Exportable recommendation profile

- [ ] Deferred beyond the current UDP/TCP release, 2026-10-12; future proposal retained, not implemented.

Current release disposition: ship existing JSON/JSONL/CSV evidence and descriptive comparisons. Automated recommendation/proxy-consumption contracts are outside this release.

Export the recommended protocol, DNS mode, ports, independent directional markings, workload, and supporting measurements in a machine-readable result. Include date, conditions, confidence, and a schema/version identifier.

Purpose: let the separate future gaming proxy consume the findings without embedding that proxy in this repository.

Before adoption: decide the format and compatibility contract. Preserve requested versus applied/observed settings, ties, and inconclusive outcomes. Exclude credentials/private keys, and avoid treating an old recommendation as a current guarantee. Exporting does not automatically apply settings or switch routes.

### S06: Separate connection quality from measurement confidence

- [x] Accepted design implemented as versioned derived report/API/dashboard/replay/export presentation, 2026-10-12. User selected per-metric labels/reasons; detailed policies are engineering selections. Current Test dashboard Chrome QA passes; full release acceptance remains separate.

Quality/deadline values remain primary. Add independent, bounded evidence fields per metric, without a numeric confidence score, ranking formula or automatic winner:

| Dimension | Vocabulary / applicability |
| --- | --- |
| Coverage | complete / partial / unavailable / incompatible, based on the required original records, identities and observation cutoffs for that metric. Complete evidence can contain poor delivery or losses. |
| Basis | locally measured / receiver reconciled / estimated / operator declared / unverified. One-way bounds and on-wire marking observations must carry their own status; API application is not captured traffic. |
| Reasons | Explicit eligible/completed/missing counts, local faults, missing remote records, clock expiry/unverified bounds, unobserved markings, omitted details and incompatible or mixed repetitions. Missing evidence for one metric does not downgrade independently valid metrics. |

**Additional sample/repetition targets are advisory.** Default completed-sample targets are 20 for median, 200 for p95 and 1,000 for p99 (ten expected observations in the upper tail, an engineering display heuristic rather than statistical confidence). Apply them to the actual eligible RTT/completion/variation population; gaps, missing samples and late samples stay explicit under existing cutoff rules. Target three complete compatible A/B pairs for repeated views. The existing two-pair minimum for a repeated description remains mandatory: all-zero deltas tie, same-sign-or-zero ranges are descriptive, opposite signs/missing/incompatible trials remain inconclusive. Falling below the extra targets adds small-sample/few-repeat reasons without hiding valid values or relaxing these gates.

Use a versioned derived presentation policy outside the frozen measurement plan. Display/export the policy version, selected targets and actual counts; overrides are explicit presentation preferences and do not change traffic, raw evidence or its hash. Per-metric known-reason vocabularies remain bounded and secret-free. Preserve observed outcomes beside limitations; unknown outer markings do not invalidate RTT, and complete receipts do not prove zero underlying packet loss.

The separate implementation checkpoint must verify threshold boundaries, two/three-pair behavior, ties/mixed/partial/missing populations, quantile/gap/cutoff consistency, independent marking/clock limitations, unchanged quality values and secret-free API/dashboard/replay/export behavior. It is implemented and checked within the recorded component fixtures; current Test dashboard Chrome QA passes; remaining release acceptance stays pending. Existing first-slice implementation/validation remains independently recorded.

Earlier task 16/21 suggestions and the initial presentation discussion are preserved in [ARCHIVE.md](ARCHIVE.md#s06-discussion-snapshot-before-release-resolution-2026-10-12); current [release policies](DESIGN.md#current-udptcp-release-decisions) supersede their unresolved status.

### Reference-tool candidates

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

#### S07: Independent reference-tool cross-checks

- [ ] Accepted future scope by user, 2026-10-11; pinned artifacts, native iperf2 build/smokes, isolated Crusader scenarios and usable IRTT exploratory overlap recorded on 2026-10-13. Independent boundary allowances/baseline/held-out bands remain incomplete. [Evidence](WORK_LOG.md#2026-10-13-reference-runtime-and-rendered-browser-progress).

Define a repeatable validation comparison against IRTT, iperf2, sockperf, Crusader or Flent where their measurements overlap ours. Retain tool/version, settings, raw evidence, endpoint/path, timing boundaries and observed workload; explain discrepancies rather than assuming either tool is ground truth.

Purpose: independently check shared-engine timing, delivery accounting and report calculations for F02/F07. This extends validation evidence, not the product's normal managed endpoint workflow.

Before execution: use the settled [reference matrix](DESIGN.md#reference-comparison-acceptance-matrix), select compatible builds and establish empirical tolerances; match payload including metadata/receipts, actual offered rates, direction, warmup, cutoff/grace and load. Compare only equivalent quantities. First-slice UDP/TCP cross-checks are included in authorized [task 33](#queued-validation-tasks); artifact checks are recorded and measured overlap/bands remain pending; other-carrier checks retain future scope. No external CLI becomes a mandatory runtime dependency.

#### S08: Timing preflight and diagnostic report

- [ ] Accepted future scope by user, 2026-10-11; local preflight and saved diagnostics implemented; live diagnostics and full timing evidence pending. [Evidence](WORK_LOG.md#2026-10-12-diagnostic-and-distribution-views).

Offer an optional local preflight that measures timer wake-up error, scheduling lag and submission overhead, plus a diagnostic view during runs. Keep these observations separate from network RTT, receiver processing and adapter queues; identify CPU pressure and unsupported observations.

Purpose: extend F07's already required timing records with an explainable diagnostic workflow inspired by IRTT. This is not cross-host clock calibration and cannot establish one-way delay.

Remaining work: integrate the contract's explicit preflight/window limits and advice with run diagnostics, recording changes between preflight and probing. Do not change OS priority/settings, subtract a measured floor from all RTTs or dismiss network failures as host problems.

#### S09: Distribution and phase comparison views

- [ ] Accepted future scope by user, 2026-10-11; saved time/ECDF, scalar matched phase comparisons and live client-local counters/RTT ranges implemented. Extra histograms/overlays and other live plots remain future scope. [Comparison evidence](WORK_LOG.md#2026-10-12-matched-saved-run-comparisons), [RTT evidence](WORK_LOG.md#2026-10-13-numeric-receipts-and-live-latency).

Add optional histograms/CDFs and matched-run overlays alongside phase timelines for completed RTT, scheduling error, deadline outcomes and delivery interruptions. Show direction, workload phase, units, sample counts, missing/late outcomes and measurement boundaries; allow inspection of the underlying records.

Purpose: extend F07's required live summaries and comparison controls using ideas from iperf2, Flent, Crusader and Graphite Meter. Saved time/ECDF, matched scalar comparisons and live client-local counters/RTT ranges are implemented; other live plots/overlays remain future scope. Complements S02/S06 without adopting their additional metrics or confidence policy.

First-slice T13/T14/T18 native SVG time/ECDF views, exact quantiles and bounded updates are settled in the [contract](DESIGN.md#first-slice-implementation-contract); tasks 20-21 implement saved integration, 23b live counters and 23n client RTT ranges. Further histograms/overlays need defined limits. Keep deadline misses first, no combined score, and explicit ties/inconclusive outcomes. Completed-sample distributions must not hide missing slow traffic. Phase alignment does not synchronize host clocks; plots never drive scheduling.

#### S10: Controlled contention scenario catalogue

- [ ] Accepted future scope by user, 2026-10-11; implementation not started. [Decision/review evidence](ARCHIVE.md#2026-10-11-selective-reference-review-completed).

Document reproducible validation scenarios: idle, external upload load, external download load, simultaneous load, bulk TCP beside FPS traffic, and bounded non-responsive UDP contention. Add isolated queue-policy/DSCP-classification fixtures when relevant, with actual load and topology recorded separately from the measurement profile.

Purpose: make F07's required externally busy-network validation reproducible, drawing on Flent and bufferbloat-lab. Isolated directional delay/rate/interruption/MTU kernel cases pass; external upload/download/both-load contention and matched reference evidence remain pending. [Evidence](WORK_LOG.md#2026-10-13-controlled-validation-and-x86-64-release-scope). This is a validation catalogue, not in-product background-load orchestration, bandwidth control or router configuration.

Before execution: record T20 fixture ownership, isolation, resource budgets, restoration and matched repetitions; account for load-generator CPU and path bypass. First-slice external-contention gates are included in the authorized validation scope, with evidence pending. Preserve user-owned services/settings; temporary changes outside owned isolated fixtures need separate authorization. Local CAKE class observations do not prove ISP prioritization. Router queue telemetry is optional local evidence, not an inferred measurement of inaccessible ISP queues or per-game-packet delay.

#### S11: Continuous responsiveness monitoring

- [ ] Accepted future scope by user, 2026-10-11; implementation not started. [Decision/review evidence](ARCHIVE.md#2026-10-11-selective-reference-review-completed).

Offer an explicitly started/stopped low-rate session with bounded rolling windows for RTT, variation, deadline outcomes and delivery gaps, inspired by Crusader and IRTT. Keep it separate from finite matched comparison runs and export window identity/settings when retained.

Purpose: reveal intermittent degradation over longer periods without making an old short comparison a current guarantee.

Before implementation: settle T14 resource/retention limits, window denominators, reconnect handling, traffic/receipt cost, and protocol eligibility. Incomplete windows remain visible. Monitoring does not select winners, switch routes, forward games or silently test in the background; richer burst/phase behavior remains under the existing workload choices.

#### S12: Classified degradation episodes and observed availability

- [ ] Deferred beyond the current UDP/TCP release; future proposal retained, not implemented. Original review: 2026-10-11; release disposition: 2026-10-12. [Review evidence](ARCHIVE.md#2026-10-11-startrinity-reference-review-recorded).

Extend accepted S11 monitoring with configurable, versioned criteria for no-arrival gaps, deadline misses, reconciled loss and latency/variation degradation. Record episode start/end, duration, direction, triggering evidence and recovery; display counts, longest episode and the fraction of eligible observed time meeting the chosen criteria. Keep no-arrival episodes separate from degraded-but-delivering periods and expose unknown/unobserved time.

Purpose: turn a long chart into an explainable event record, inspired by StarTrinity CST's documented downtime thresholds. This is measured availability for a specific path/protocol/workload/window, not proof that the whole Internet was down. It overlaps S02's burst/interruption ideas without accepting S02 or replacing raw delivery accounting.

Before adoption: settle T24's jitter definitions, window/denominator, minimum evidence, transition/debounce/hysteresis, quiet-phase handling and recovery rules. Pauses, intentional burst gaps, canceled runs, local unsent work, control/calibration failures and missing receiver records must not silently become network outages. Distinguish stream stalls from packet loss. No StarTrinity numerical thresholds, VoIP MOS grades, automatic route changes or external notification service are selected. Candidate status does not expand accepted S11 behavior by implication.

### Maintaining feature statuses

Keep stable F/S identifiers with purpose, explicit status and open dependencies. Proposed means undecided; accepted means confirmed scope, not execution authorization. In-progress/partial/completed states need work-log evidence; completed requires applicable implementation/checks. Retain deferred/rejected decisions with date/reason.

Update affected `DESIGN.md` requirements, this forward status and backward `WORK_LOG.md` evidence when scope or implementation changes. Preserve completed identifiers and coverage gaps. Keep entries concise, link canonical details, and log significant compaction; do not invent acceptance, release dates or priority.
