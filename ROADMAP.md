# First actions

Updated: 2026-10-11. Steps 1-4 are completed documentation milestones. Step 5 is authorized and in progress; foundations exist, complete runtime workflow/acceptance remain pending. Requirements are in [DESIGN.md](DESIGN.md), answers in [CHOICES.md](CHOICES.md), feature status in [PROPOSALS.md](PROPOSALS.md), and evidence in [WORK_LOG.md](WORK_LOG.md).

## 1. Settle the immediate foundation choices (completed)

**State: complete, 2026-10-11.** Go, local embedded plain-browser UI, client/server packaging, Windows 11/Linux ARM64/x86-64 roles, shared engine, YAML profiles, JSONL/JSON/CSV results and pairing direction settled. [Answers and remaining platform details](CHOICES.md#foundation-discussion).

## 2. Define a small measurement contract (completed)

**State: complete, 2026-10-11.** Lifecycle, independent bidirectional data/receipts, configurable steady/burst phases, timing/accounting, dashboard/report concepts and illustrative acceptance examples documented. [Conceptual contract](DESIGN.md#measurement-contract-draft); task 4 supplies the final first-slice mechanics.

## 3. Check integration constraints before locking the plan (completed)

**State: complete as source review, 2026-10-11.** Managed lifecycle, socket/port/outer-priority, DNS delegation/polling and Linux packaging expectations/gaps recorded. [Integration review](CHOICES.md#integration-constraints-review). Runtime support remains unvalidated.

## 3a. Record useful reference-tool components (completed)

**State: complete, 2026-10-11.** [Candidate inventory](PROPOSALS.md#reference-tool-candidates) and [reuse choices](CHOICES.md#reference-tool-reuse-and-validation) recorded. [Evidence](WORK_LOG.md#2026-10-11-reference-tool-planning-recorded).

## 3b. Evaluate selective reuse and validation baselines (completed)

**State: complete as source review, 2026-10-11.** IRTT/iperf2/Crusader selected as design references/external validation only; S07-S11 accepted for future scope. [Dispositions and source gaps](CHOICES.md#selective-source-review-and-dispositions), [acceptance matrix](DESIGN.md#reference-comparison-acceptance-matrix), [evidence](WORK_LOG.md#2026-10-11-selective-reference-review-completed). No reference experiments performed.

## 3c. Review the StarTrinity reference (completed)

**State: complete as vendor-documentation review, 2026-10-11.** [Review](CHOICES.md#startrinity-documentation-review) recorded; S12 remains proposed and T24 retains future metric/event choices. No StarTrinity component selected or executed. [Evidence](WORK_LOG.md#2026-10-11-startrinity-reference-review-recorded).

## 4. Agree on the first implementation slice (completed)

**State: complete as planning, 2026-10-11.** [Settled contract](DESIGN.md#first-slice-implementation-contract): real UDP/TCP, shared engine/dashboard/lifecycle, pairing, ports/directional priority, compile-time adapters, embedded key-only SSH coordination and calibration evidence. Basic S08/S09 and S07/S10 checks are included; S11 and remaining carriers/DNS stay future scope.

[Final choices](CHOICES.md#first-slice-final-decisions) distinguish user answers from engineering selections. [Evidence](WORK_LOG.md#2026-10-11-first-slice-planning-completed). Numerical operator inputs remain explicit; compatible releases, Linux minima and empirical timing bands need later checks. No runtime gate has passed.

## 5. Implement and validate the agreed first slice

**State: authorized by user, 2026-10-11; in progress, incomplete.**

First foundations: pinned components/licenses, strict profiles/hash, wire codec, real UDP/TCP sockets, registry, rational slots, accounting, clock/SSH primitives and local preflight. Every function is commented and [FLOWCHART.md](FLOWCHART.md) maps actual code. Four builds and foundation Windows/WSL tests pass; this does not complete the first slice. [Evidence and gaps](WORK_LOG.md#2026-10-11-first-implementation-foundations).

Next:

1. Connect the independent bounded measurement engine to the implemented SSH/pairing/bind/readiness/lease path. [Readiness evidence](WORK_LOG.md#2026-10-11-ssh-pairing-and-readiness); cross-platform CLI/native/live validation remains outstanding.
2. Connect maintained calibration, raw evidence/report persistence/replay/export, protected loopback API and embedded dashboard/comparisons.
3. Run applicable deterministic, build, loopback, native priority/calibration, impairment, reference and live-path gates. Establish experimental timing bands before comparisons; retain missing evidence as outstanding.
4. Update actual implementation/check evidence in `WORK_LOG.md` and partial F01/F02/F05-F09/S07-S10 status in `PROPOSALS.md`.

## After these actions

Choose the next small set from what we learn. Keep this file brief, link completed details to evidence, and avoid inventing later rollout order or release dates.
