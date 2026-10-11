# First actions

Updated: 2026-10-11. This is a short, adjustable action sequence, not a complete release plan. Implementation remains on hold; a planned action does not authorize code, dependencies, services, or network experiments.

Use [CHOICES.md](CHOICES.md) for answers, [DESIGN.md](DESIGN.md) for requirements, [PROPOSALS.md](PROPOSALS.md) for feature scope, and [WORK_LOG.md](WORK_LOG.md) for completed work and checks. Go and a local browser dashboard are already confirmed.

## 1. Settle the immediate foundation choices (completed)

**State: complete; foundation answers and remaining uncertainties recorded on 2026-10-11.**

Decide packaging, dashboard approach, and initial supported OS/CPU targets (T03, T04/T13, T12). Then settle only the architecture, configuration/results, and pairing details needed for the first slice (T06-T09). Work through these in small batches; we do not need to answer every question before making progress.

Outcome: confirmed answers and remaining uncertainties recorded in `CHOICES.md`, without selecting dependencies or defaults by assumption.

Confirmed: one executable per OS/CPU with client/server modes (T03), embedded plain HTML/CSS/JavaScript (T04/T13), Windows 11 client/Linux server roles on ARM64/x86-64 with broad mainstream Linux distribution coverage (T12), shared-engine boundaries (T06), versioned YAML profiles with shared validation/explicit save (T07), JSONL/JSON results with CSV export (T08), and generated pairing plus user-provided TLS certificates (T09). [Foundation discussion](CHOICES.md#foundation-discussion) records answers and alternatives. Exact schemas/pairing mechanics and chart/layout needs follow in the contract discussion; Linux version/support boundaries follow in the integration review. Completion is a design milestone, not implementation authorization.

## 2. Define a small measurement contract

**State: next action; not started.**

Document the first run's lifecycle, message identities, independent directional scheduling, timing boundaries, delivery accounting, result fields, and dashboard controls (T14). Choose an initial configurable workload and comparison criteria (T15, T18), preserving the correctness rules in `DESIGN.md`.

Describe the selected pairing import and user-provided certificate flows sufficiently for the first slice, and identify unresolved credential lifecycle details. Use dashboard control needs to guide later chart/layout choices; do not select a library by assumption.

Outcome: a concrete example test profile and expected report described in documentation, with acceptance checks. No runnable configuration or application code yet.

## 3. Check integration constraints before locking the plan

**State: planned; documentation/source research only under current authorization.**

Review candidate protocol integrations for managed lifecycle, socket ownership, source/destination port control, and outer packet priority on Windows/Linux (T05, T11, T17). Identify DNS domain/delegation requirements and framing/polling questions (T16). Record unknowns and checks that will need later authorized implementation or experiments.

Assess exact Linux distribution/version boundaries and kernel/userspace/dependency requirements against the confirmed broad coverage (T12). Keep the intended support matrix separate from later validated coverage.

Outcome: documented capability expectations and gaps. Source/API review is not proof of runtime support. Keep all F01-F07 coverage in scope; do not silently drop difficult modes.

## 4. Agree on the first implementation slice

**State: planned; implementation requires explicit user authorization.**

Use the preceding answers to propose a small, reviewable first deliverable and its checks. A possible starting point is the shared measurement engine, managed client/server lifecycle, local dashboard, and real UDP/TCP exchanges with configurable ports and directional priority capability reporting (parts of F01, F02, F05-F07). This starting scope is a suggestion, not an accepted implementation order.

Outcome: agreed scope, success criteria, and explicit authorization before implementation begins. Once authorized, build and validate that slice, record actual evidence in `WORK_LOG.md`, and update partial feature status in `PROPOSALS.md`.

## After these actions

Choose the next small set of actions from what we learn. Do not schedule the rest of the protocol/DNS rollout now or invent release dates. Keep step status accurate; planning, source research, implementation, and runtime validation are distinct states.
