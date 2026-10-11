# First actions

Updated: 2026-10-11. This is a short, adjustable action sequence, not a complete release plan. Implementation remains on hold; a planned action does not authorize code, dependencies, services, or network experiments.

Use [CHOICES.md](CHOICES.md) for answers, [DESIGN.md](DESIGN.md) for requirements, [PROPOSALS.md](PROPOSALS.md) for feature scope, and [WORK_LOG.md](WORK_LOG.md) for completed work and checks. Go and a local browser dashboard are already confirmed.

## 1. Settle the immediate foundation choices (completed)

**State: complete; foundation answers and remaining uncertainties recorded on 2026-10-11.**

Decide packaging, dashboard approach, and initial supported OS/CPU targets (T03, T04/T13, T12). Then settle only the architecture, configuration/results, and pairing details needed for the first slice (T06-T09). Work through these in small batches; we do not need to answer every question before making progress.

Outcome: confirmed answers and remaining uncertainties recorded in `CHOICES.md`, without selecting dependencies or defaults by assumption.

Confirmed: one executable per OS/CPU with client/server modes (T03), embedded plain HTML/CSS/JavaScript (T04/T13), Windows 11 client/Linux server roles on ARM64/x86-64 with broad mainstream Linux distribution coverage (T12), shared-engine boundaries (T06), versioned YAML profiles with shared validation/explicit save (T07), JSONL/JSON results with CSV export (T08), and generated pairing plus user-provided TLS certificates (T09). [Foundation discussion](CHOICES.md#foundation-discussion) records answers and alternatives. Exact schemas/pairing mechanics and chart/layout needs follow in the contract discussion; Linux version/support boundaries follow in the integration review. Completion is a design milestone, not implementation authorization.

## 2. Define a small measurement contract (completed)

**State: complete as a documentation milestone, 2026-10-11. Configurable steady/burst traffic with ordered phase sequences, independent bidirectional data plus receipts carrying payload information, and report presentation confirmed; detailed schemas/defaults/mechanics remain explicitly open.**

Document the first run's lifecycle, message identities, independent directional scheduling, timing boundaries, delivery accounting, result fields, and dashboard controls (T14). Choose an initial configurable workload and comparison criteria (T15, T18), preserving the correctness rules in `DESIGN.md`.

Describe the selected pairing import and user-provided certificate flows sufficiently for the first slice, and identify unresolved credential lifecycle details. Use dashboard control needs to guide later chart/layout choices; do not select a library by assumption.

Outcome: a concrete example test profile and expected report described in documentation, with acceptance checks. No runnable configuration or application code yet.

Outcome recorded: [measurement contract](DESIGN.md#measurement-contract-draft) covers lifecycle, identities, scheduling/timing, data-versus-receipt accounting, logical report/control fields, pairing/certificate flows, an editable prose example workload, a synthetic expected report, and future acceptance checks. [Contract discussion](CHOICES.md#measurement-contract-discussion) distinguishes confirmed T14/T15/T18 answers from unresolved refinements. Numerical examples are not selected defaults; exact schemas, resource/phase policies, credential lifecycle and recommendation evidence rules follow as needed before an implementation slice. Completion does not authorize implementation.

## 3. Check integration constraints before locking the plan (completed)

**State: complete as a documentation/source-review milestone, 2026-10-11; integrations and platform support remain unvalidated.**

Review candidate protocol integrations for managed lifecycle, socket ownership, source/destination port control, and outer packet priority on Windows/Linux (T05, T11, T17). Identify DNS domain/delegation requirements and framing/polling questions (T16). Record unknowns and checks that will need later authorized implementation or experiments.

Assess exact Linux distribution/version boundaries and kernel/userspace/dependency requirements against the confirmed broad coverage (T12). Keep the intended support matrix separate from later validated coverage.

Outcome: documented capability expectations and gaps. Source/API review is not proof of runtime support. Keep all F01-F07 coverage in scope; do not silently drop difficult modes.

Outcome recorded: [integration review](CHOICES.md#integration-constraints-review) covers managed protocol lifecycle/socket/port hooks, outer-priority limitations and observation, DNS delegation/framing/polling, Linux kernel/toolchain/userspace and packaging boundaries, and checks requiring later authorization. T05/T11/T16/T17 remain open/recommended; exact T12 release minima are unselected. All protocol modes remain in scope. Action 4 is next; this completion neither approves dependencies/defaults nor authorizes implementation.

## 3a. Record useful reference-tool components (completed)

**State: complete as a planning/documentation milestone, 2026-10-11; no functionality implemented or integration validated.**

Outcome: [reference inventory](PROPOSALS.md#reference-tool-candidates) maps IRTT, iperf2, Crusader, Flent, Graphite Meter, sockperf, dnstt, iodine and bufferbloat-lab to existing F01-F07 requirements. S07-S11 record probable additions: independent cross-checks, timing preflight, distribution/phase views, a controlled-contention validation catalogue and continuous monitoring. Existing S01-S06 and confirmed choices retain their status.

[Reuse choices](CHOICES.md#reference-tool-reuse-and-validation) record the user's selective-learning direction and open T19/T20 questions. [Work-log evidence](WORK_LOG.md#2026-10-11-reference-tool-planning-recorded) records the actual documentation and checks. Completion means candidates are recorded, not selected, source-audited, copied, built or tested.

## 3b. Evaluate selective reuse and validation baselines

**State: planned; documentation/source evaluation only while implementation is on hold.**

Start with IRTT for timing/accounting, iperf2 for traffic/port/priority controls and Crusader for the client/server workflow. Evaluate other references where they address a concrete gap. Inspect exact versions/source symbols, platform behavior, timing/denominator semantics, lifecycle/socket access and license/redistribution constraints under T19/T20. Record findings rather than assuming a documented feature fits every adapter.

For each useful component, choose among a design reference, compatible embedded code/library, a tool-managed bundled component or an independent validation utility; leave unresolved choices explicit. Link to existing T13-T18 and F01-F07 instead of duplicating confirmed requirements. Decide individually whether S07-S11 or older suggestions should enter confirmed scope; do not adopt them all by implication.

Prepare a prose acceptance matrix for overlapping UDP/TCP measurements, port/outer-DSCP observations, matched idle/loaded phases, receiver accounting, and DNS framing/polling/limits. Specify required Windows 11/Linux ARM64/x86-64 evidence, reference-tool gaps, and each future check's timing boundaries/tolerances. Keep calibrated one-way delay optional and preserve missing/late evidence. External load/queue fixtures remain outside the product.

Outcome: documented component dispositions, proposal decisions, reuse/license/platform gaps and validation criteria needed for the first slice. No dependency installation, code copying, runnable profiles, helper execution, services or network experiments are authorized by this step. Source-only review does not fulfill later runtime acceptance checks.

## 4. Agree on the first implementation slice

**State: planned; implementation requires explicit user authorization.**

Use the preceding answers to propose a small, reviewable first deliverable and its checks. A possible starting point is the shared measurement engine, managed client/server lifecycle, local dashboard, and real UDP/TCP exchanges with configurable ports and directional priority capability reporting (parts of F01, F02, F05-F07). This starting scope is a suggestion, not an accepted implementation order.

Resolve the precise schemas, resource/phase policies, credential mechanics, and comparison evidence rules needed by the selected slice, using action 3's integration findings and action 3b's relevant reuse/validation dispositions. Select only necessary components; leave optional candidates outside the slice unless explicitly adopted. Keep other unresolved choices explicit rather than inventing defaults.

Include F08's [protocol extensibility contract](DESIGN.md#protocol-extensibility) from the start: settle only the needed T21 adapter/capability/registration details and define a later authorized check that adding an adapter does not require broadly rewriting shared modules. Do not schedule specific future protocols now.

Outcome: agreed scope, success criteria, and explicit authorization before implementation begins. Once authorized, build and validate that slice, record actual evidence in `WORK_LOG.md`, and update partial feature status in `PROPOSALS.md`.

## After these actions

Choose the next small set of actions from what we learn. Do not schedule the rest of the protocol/DNS rollout now or invent release dates. Keep step status accurate; planning, source research, implementation, and runtime validation are distinct states.
