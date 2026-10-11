# Work log

Use this file to track backward: completed changes, validation, and current implemented behavior across conversations. [PROPOSALS.md](PROPOSALS.md) tracks forward: future features, ideas, decisions, and progress. [DESIGN.md](DESIGN.md) contains requirements. A log entry is evidence to check, not a substitute for inspecting current files.

## Current state

**Documentation only. No application code or runtime features are implemented.**

| Area | State | Evidence / location | Last changed |
| --- | --- | --- | --- |
| Project scope and measurement design | Documented | [DESIGN.md](DESIGN.md) | 2026-10-11 |
| Agent instructions and work tracking | Documented; automatic roadmap completion updates and `(completed)` heading markers required | [AGENTS.md](AGENTS.md), this file | 2026-10-11 |
| Forward feature/proposal tracking | Documented | [PROPOSALS.md](PROPOSALS.md), F01-F07 and S01-S06 | 2026-10-11 |
| Technical choices | Foundation confirmations unchanged; T14 independent bidirectional data/receipts with payload information, T15 traffic/phase controls and T18 presentation confirmed; detailed limits open | [CHOICES.md](CHOICES.md) | 2026-10-11 |
| Measurement contract | Conceptual documentation milestone complete; editable example workload, synthetic report and acceptance checks; final schemas/defaults/mechanics/evidence rules open | [DESIGN.md](DESIGN.md#measurement-contract-draft), [discussion](CHOICES.md#measurement-contract-discussion) | 2026-10-11 |
| Near-term action sequence | Actions 1 and 2 complete as documentation milestones; action 3 is next/planned integration review | [ROADMAP.md](ROADMAP.md) | 2026-10-11 |
| Implemented application features | None | No application source or build manifest; future scope is indexed in `PROPOSALS.md` | 2026-10-10 |

## Recording rules

- Update the current-state index and append history after meaningful repository changes. Read-only replies do not need entries.
- Use a date and, when known, time with timezone. New timestamps use Europe/Istanbul (UTC+03:00); do not invent historical times.
- For code changes, record the actual behavior, reason, affected source paths/symbols, implementation state, validation performed/results, and remaining gaps. Include a commit identifier only if one exists.
- Distinguish documented, proposed, in progress, partially implemented, implemented, validated, reverted, and removed. A design entry must not become an implementation claim.
- Record failed or unrun checks honestly. Say which platform/protocol was tested; compilation or loopback success does not imply complete runtime validation.
- Keep earlier entries as history. Record later modifications or reversions in a new entry and update current state; correct factual errors explicitly.
- Reference matching feature/proposal identifiers from `PROPOSALS.md` when work or decisions affect them. Update their forward status while recording actual changed behavior and checks here. Do not duplicate detailed future feature specifications in the work log.
- Reference technical decision identifiers from `CHOICES.md` for significant answers or changes. Keep detailed options/research there and dated actual decisions here.

## History

### 2026-10-10 — Initial requirements and documentation

- Captured measurement-only scope: Windows/Linux clients, one Linux server, real tool-managed protocols, DNS experiments, ports, independent directional packet priority, and FPS delivery quality.
- Created agent instructions, four separate design/validation documents, and a repository-local measurement skill; updated the overview and the former Claude entrypoint.
- Validation at that stage: local links and whitespace checked, skill validator passed, and documentation/skill behavior reviewed. No application builds, runtime tests, services, or network experiments were performed.
- State: documentation completed; all application features remained unimplemented. Exact historical time was not recorded.

### 2026-10-10 — Optional suggestions saved

- Created the suggestions file originally named `NotImplementedYet.md` (now [PROPOSALS.md](PROPOSALS.md)) with S01-S06: staged experiments, burst metrics, coordination separation, size/rate sweeps, exportable recommendations, and separate quality/confidence.
- Linked the backlog from the overview and instructions. Suggestions remain proposals rather than approved requirements.
- Validation: local links and whitespace checked. No implementation added. Exact historical time was not recorded.

### 2026-10-10 21:28 UTC+03:00 — Documentation consolidation and persistent work tracking

- Consolidated confirmed requirements, protocol/DNS coverage, ports/priority, measurement methods, and acceptance checks into [DESIGN.md](DESIGN.md).
- Simplified [AGENTS.md](AGENTS.md), added this work log, and linked both from [README.md](README.md). Future work checks the current-state index and relevant history before assuming a feature is absent or complete.
- Removed the unused Claude entrypoint, superseded `docs/` documents, and redundant repository-local skill. Preserved the optional backlog with its links updated.
- Preserved the existing `.gitignore` unchanged; it currently excludes Markdown documents from Git unless already tracked.
- State: documentation cleanup and tracking only. Application implementation remains on hold; all runtime features remain unimplemented.
- Validation: all five Markdown files passed local-link and trailing-whitespace checks; `git diff --check` passed. Final inventory confirmed only the five intended Markdown files, with no application code, dependency manifest, or runtime configuration added. No runtime tests were performed.

### 2026-10-10 21:33 UTC+03:00 — Forward proposals and backward work history

- Renamed `NotImplementedYet.md` to [PROPOSALS.md](PROPOSALS.md), preserving S01-S06 and their unapproved status.
- Added F01-F07 to index the already accepted future scope; all remain not started. No new feature acceptance or implementation authorization was inferred.
- Updated [AGENTS.md](AGENTS.md), [README.md](README.md), and [DESIGN.md](DESIGN.md) links and tracking rules. Forward statuses/decisions belong in `PROPOSALS.md`; completed changes and validation belong here, linked by feature/proposal identifier.
- Simplified this current-state index to actual repository/implementation state; future feature details now live in the proposals index.
- State: documentation only; application code and runtime features remain unimplemented. The implementation hold and existing `.gitignore` are unchanged.
- Validation: rename/old-file absence, five-file inventory, local links and section anchors, S01-S06/F01-F07 uniqueness, trailing whitespace, and `git diff --check` all passed. No application tests or network experiments were run.

### 2026-10-11 05:20 UTC+03:00 — Initial technical decisions

- User confirmed Go (T01) and a local browser dashboard (T02). Recorded both in `DESIGN.md` and forward choice progress in `PROPOSALS.md`; F01-F07 are still not started.
- Added proposed choices T03-T11 for packaging, embedded UI, protocol integrations, measurement boundaries, configuration/results, trust, execution, and platform priority. These are recommendations, not accepted decisions. S01-S06 were not promoted.
- Inspected primary Go, QUIC, WireGuard, Hysteria2, DNS, and Microsoft QoS sources to identify integration candidates and capability limits. This was source research, not integration validation.
- DNS source review found the original miekg/dns implementation is receiving specific fixes only and points to a new Codeberg implementation. Kept the DNS dependency choice open for API/stability/integration assessment rather than finalizing the older candidate.
- State: documentation only; implementation hold remains. No application source, dependency installation/manifest, running service, or network measurement was added.
- Validation: all five Markdown files passed local-link, section-anchor, and trailing-whitespace checks; `git diff --check` passed. Verified that only T01-T02 are marked confirmed and F01-F07 remain not started. No builds, runtime tests, or network experiments were performed.

### 2026-10-11 05:29 UTC+03:00 — Dedicated technical choices file

- Created [CHOICES.md](CHOICES.md) at the user's request and moved the existing T01-T11 table and research notes out of `DESIGN.md`. Preserved Go/local browser dashboard as the only confirmed technical answers.
- Added explicit discussion questions for the existing recommendations and T12-T18 for previously unresolved platform, frontend, schema, workload, DNS, priority, and ranking details. No additional feature or choice was approved.
- Updated `AGENTS.md`, `README.md`, `PROPOSALS.md`, and the current-state index to route technical options/answers to `CHOICES.md`, future features to `PROPOSALS.md`, and completed work/history here. Kept a brief confirmed-stack pointer in `DESIGN.md`.
- State: documentation only; no implementation, dependencies, runtime configuration, services, or network experiments added. Existing ignore rules are unchanged.
- Validation: six-file Markdown inventory, local links and heading anchors, T01-T18 identifier uniqueness, exactly two confirmed choices, and whitespace checks passed. `git diff --check` passed. No runtime tests were needed for this documentation change.

### 2026-10-11 05:45 UTC+03:00 — Initial roadmap

- Created [ROADMAP.md](ROADMAP.md) at the user's request with only the first actions: settle immediate choices, define a measurement contract, review integration constraints, and agree on an explicitly authorized first implementation slice. Later actions remain open to evidence and discussion.
- Linked the roadmap from `AGENTS.md`, `README.md`, and `PROPOSALS.md`; added it to this current-state index. Technical answers remain in `CHOICES.md`, feature scope/status in `PROPOSALS.md`, and completed work here.
- State: documentation only. The possible UDP/TCP starting slice is a suggestion, not an accepted order or implementation authorization. F01-F07 remain not started, S01-S06 remain unapproved, and T01-T02 remain the only confirmed technical choices.
- Validation: seven-file Markdown inventory, local links/anchors, trailing whitespace, four roadmap steps, unchanged confirmed-choice/feature states, and `git diff --check` passed. No runtime tests were needed. No application code, manifests, runtime configuration, services, or network experiments added; ignore rules and unrelated files preserved.

### 2026-10-11 05:48 UTC+03:00 Foundation discussion started

- Started roadmap step 1 at the user's request. Added a focused first batch in `CHOICES.md` for packaging (T03), dashboard stack (T04/T13), and OS/CPU targets (T12), with alternatives and explicit pending-answer states. T06-T09 remain the next discussion batch.
- Updated `ROADMAP.md` to in discussion, the current-state index here, and technical-choice progress in `PROPOSALS.md`. No additional technical answer was inferred; only T01-T02 are confirmed.
- Checked primary Go embedding documentation to support the proposed asset-bundling approach. Platform targets, chart library/layout, and protocol integration feasibility remain unresolved; this research is not runtime support evidence.
- State: documentation only. F01-F07 remain accepted/not started and S01-S06 remain proposed. No code, dependencies/manifests, runnable configuration, services, or network experiments added. Existing ignore rules and unrelated changes preserved; changes left uncommitted.
- Validation: all seven Markdown files passed local-link/heading-anchor and trailing-whitespace checks; confirmed-choice states, F01-F07 not-started states, and S01-S06 inventory passed. `git diff --check` passed for tracked files; explicit Markdown checks covered ignored documents. Python was unavailable, so the checks ran through native PowerShell. No runtime tests or network experiments were performed.

### 2026-10-11 05:50 UTC+03:00 First foundation answers recorded

- Recorded the user's packaging answer (T03: one executable per OS/CPU with client/server modes) and embedded plain HTML/CSS/JavaScript dashboard answer (T04/T13) in `CHOICES.md`, dated 2026-10-11. Chart library/layout remain open; no unasked defaults or dependencies were adopted.
- Recorded T12 as partly confirmed: Windows 11 client only, Linux server only, ARM/x86 families on both. This supersedes the previous Windows/Linux-client scope. Exact bitness, Linux distributions/versions, and actual server environment remain open; requested clarification rather than silently choosing ARM64/amd64 or 32-bit targets.
- Updated current scope in `DESIGN.md`, `README.md`, `AGENTS.md`, and F01 in `PROPOSALS.md`; updated roadmap progress and this current-state index. Prepared the next questions for T06-T07; T08-T09 follow. Inspected primary YAML/Go JSON documentation for profile-format options; no parser or schema selected.
- State: documentation only, implementation still on hold. F01-F07 remain accepted/not started; S01-S06 remain proposed. No application code, manifests, runnable configuration, services, or network experiments added. Ignore rules and unrelated user changes preserved; changes left uncommitted.
- Validation: checked with the following decision update; no application/runtime checks performed.

### 2026-10-11 05:52 UTC+03:00 Architecture and profile answers recorded

- User clarified T12 as 64-bit ARM64 and x86-64 only for Windows 11 clients and Linux servers. Recorded the four intended Go build targets without claiming builds or runtime validation; Linux version/distribution targets remain open.
- Confirmed T06's shared measurement engine and separate protocol adapters, platform port/priority support, and dashboard/API modules. Confirmed T07's versioned YAML profiles, shared validation rules, and explicit save for dashboard edits. Parser, schema, defaults, and storage paths remain undecided; S03 was not adopted.
- Updated `CHOICES.md`, the application foundation/platform requirements in `DESIGN.md`, `README.md`, `ROADMAP.md`, F01/choice progress in `PROPOSALS.md`, and this current-state index. Asked the final foundation batch for T08 results, T09 pairing, and T12 Linux versions.
- State: documentation only; implementation remains on hold. F01-F07 remain accepted/not started, S01-S06 remain proposed, and all changes are uncommitted. No application code, dependency manifests, runnable configuration, services, or network experiments added.
- Validation: all seven Markdown files passed local-link/heading-anchor and trailing-whitespace checks; confirmed-choice states, primary T01-T18 uniqueness, F01-F07 not-started states, S01-S06 inventory, and current-scope checks passed. The first identifier check counted repeated discussion references; restricting it to the primary choice tables resolved that checker error. `git diff --check` passed for tracked files; explicit checks covered ignored Markdown files. File inventory contains only the seven documents and existing `.gitignore`. No application/runtime checks or network experiments performed.

### 2026-10-11 05:59 UTC+03:00 Foundation decisions completed

- Confirmed T08 from the user's answer: per-run JSONL raw observations, JSON summary/settings snapshot, and CSV export. Updated `CHOICES.md` and `DESIGN.md`; exact schemas/file lifecycle remain open under T14, and S05 was not adopted.
- Recorded T12's broad Linux server distribution scope, including the user's CentOS/Ubuntu/Fedora/Arch examples and other mainstream distributions. Updated `CHOICES.md`, `DESIGN.md`, `README.md`, and F01 in `PROPOSALS.md`. Exact version/kernel/userspace/dependency limits remain open for roadmap action 3; no distribution/runtime compatibility was tested or promised.
- Explained T09 after the user said the earlier question was unclear; did not infer acceptance. User then selected generated pairing setup plus support for TLS certificates they provide. Recorded the trusted transfer/client dashboard import approach, explicit identity verification, and remaining enrollment/storage/certificate lifecycle details in `CHOICES.md` and `DESIGN.md`. Inspected primary Go TLS verification documentation; this is source research, not a validated integration.
- Marked roadmap action 1 complete because foundation answers and remaining uncertainties are recorded. Action 2 (measurement contract) is next/not started; schemas, pairing mechanics, chart/layout needs, and Linux support boundaries are assigned to the following discussions. Updated `ROADMAP.md`, choice progress in `PROPOSALS.md`, and this current-state index.
- State: documentation only; implementation remains on hold. F01-F07 remain accepted/not started and S01-S06 proposed. No code, dependency manifests, runnable configuration, services, or network experiments added. Changes left uncommitted; existing ignore rules and unrelated files/settings preserved.
- Validation: all seven Markdown files passed local-link/heading-anchor and trailing-whitespace checks. Primary T01-T18 uniqueness, eight confirmed core choices (T01-T04/T06-T09), unchanged F01-F07/S01-S06 implementation/proposal states, action 1 completion/action 2 not-started states, current-scope consistency, and documentation-only inventory passed. `git diff --check` passed for tracked files; explicit Markdown checks covered ignored documents. No application/runtime checks or network experiments performed.

### 2026-10-11 06:01 UTC+03:00 Automatic roadmap completion tracking

- Added the user's standing instruction to `AGENTS.md`: after a roadmap task's stated outcome and required checks are complete, mark it complete in `ROADMAP.md` automatically before reporting, without waiting for a reminder. Partial/incomplete tasks retain accurate states.
- Verified that roadmap action 1 is already complete and action 2 remains next/not started; no additional task was completed by this instruction update. Updated this current-state index.
- State: documentation only, changes uncommitted. Implementation hold, F01-F07/S01-S06 states, technical decisions, and existing ignore rules are unchanged. No application code or network experiments added.
- Validation: all seven Markdown files passed local-link/heading-anchor and trailing-whitespace checks. Verified the automatic tracking instruction, unchanged implementation hold, action 1 complete/action 2 not-started states; `git diff --check` passed. No runtime checks needed or performed.

### 2026-10-11 06:05 UTC+03:00 Completed roadmap heading format

- Added `(completed)` to action 1's heading in `ROADMAP.md`, matching the user's requested format. Updated `AGENTS.md` so every future completed roadmap task gets this heading suffix automatically as well as a complete state. Updated this current-state index.
- State: documentation only, changes uncommitted. No additional roadmap action completed; implementation authorization, feature/proposal states, technical decisions, and unrelated user changes remain unchanged.
- Validation: all seven Markdown files passed local-link/heading-anchor and trailing-whitespace checks. Verified the exact completed heading format, only action 1 marked completed, and action 2 still next/not started; `git diff --check` passed. No runtime checks needed or performed.

### 2026-10-11 06:14 UTC+03:00 Measurement contract draft started

- Started roadmap action 2 at the user's request. Inspected current documentation, relevant history, branch/status, and file inventory; no application implementation exists to duplicate. Added the measurement-contract draft to `DESIGN.md` and T14/T15/T18 discussion to `CHOICES.md`.
- Draft covers phase/session/message identities, independent bidirectional data plus correlated receipts, monotonic timing boundaries, separate data/receipt accounting, explicit denominators/incomplete evidence, logical profile/event/report fields, dashboard controls, and generated-pairing/user-certificate flows. Numerical workload, proposed mechanics, comparison thresholds, detailed schemas, and credential lifecycle remain unconfirmed/open.
- Included a provisional prose workload and explicitly synthetic report with count identities and future acceptance examples. Asked the user to choose the T15 starting workload and T18 initial comparison approach; no acceptance inferred from recommendations or silence. No protocol dependency, chart library, default port, first implementation slice, T10 execution policy, or optional S01-S06 adoption selected.
- Updated `ROADMAP.md` to in discussion/incomplete, this current-state index, and F07/technical-choice progress in `PROPOSALS.md` with evidence links; F01/F05/F06 design connections recorded. Action 2 awaits decisions and is not marked completed.
- State: documentation only. All application features remain unimplemented; implementation hold and confirmed foundation choices remain intact. Changes uncommitted; existing ignore rules and unrelated files/settings preserved. No application code, helpers, manifests, runnable configuration, services, or network experiments added.
- Validation: local-link/heading-anchor, trailing-whitespace, F/S identifiers, roadmap states, documentation-only inventory, and synthetic count reconciliation checked. The initial choice-row checker also counted repeated foundation/discussion references; scoped primary-table checking then passed T01-T18 uniqueness and eight unchanged core confirmations. `git diff --check` passed. No application/runtime tests or network experiments performed.

### 2026-10-11 06:16 UTC+03:00 Configurable workload and report presentation answers

- User requested configurable packet size, burst time, payload, and other traffic properties (T15), rather than selecting the proposed fixed numbers. Recorded broad configurability in `CHOICES.md` and `DESIGN.md`; values remain illustrative, not accepted defaults. Proposed independent steady/repeating-burst controls, burst duration/gap/rate, and reproducible payload content; asked for the detailed burst answer. Exact outer packet size remains adapter-dependent and must be distinguished from configured payload size.
- User confirmed T18 presentation: deadline misses first, then p95/p99 RTT and variation, ties/inconclusive, no combined score. Updated `CHOICES.md`/`DESIGN.md`; actual ranking evidence thresholds and repetitions remain open. Also asked about T14's independent bidirectional data plus correlated receipts, with overhead explicit.
- Updated `ROADMAP.md`/current-state index and F07 choice progress in `PROPOSALS.md`. Action 2 remains in discussion/incomplete; no numerical defaults or optional S01-S06 were adopted. Configuring a workload does not approve automated S04 sweeps or product traffic shaping.
- State: documentation only, implementation still on hold, changes uncommitted. No application code, runnable profiles, dependencies, services, or network experiments added.
- Validation: all seven Markdown files passed local-link/heading-anchor and trailing-whitespace checks. Primary T01-T18 identifiers, unchanged core confirmations, recorded T15/T18 answers, F01-F07 unimplemented/S01-S06 proposed states, incomplete roadmap action 2, documentation-only inventory, synthetic count reconciliation, and `git diff --check` passed. No application/runtime tests performed.

### 2026-10-11 06:18 UTC+03:00 Traffic phase sequences and receipt clarification

- User extended T15's proposed steady/burst controls with an ordered sequence of different traffic phases. Updated `DESIGN.md`/`CHOICES.md` with per-phase duration, independent directional traffic settings, payload controls, an illustrative phase sequence, dashboard phase editing, and proposed phase-aware reporting/accounting. Exact phase transitions, rounding, and deadline overrides remain open; no numerical defaults accepted.
- User asked for a clearer T14 explanation. Recorded the independent client/server data plus small correlated application-receipt model, extra overhead, separate delivery versus receipt outcomes, and a data-only alternative. The request to explain is not acceptance; T14 remains pending.
- Updated `ROADMAP.md`, F07 choice-progress evidence in `PROPOSALS.md`, and this current-state index. Roadmap action 2 remains in discussion/incomplete. F01-F07 have no implementation; S01-S06 remain proposed, including automated S04 sweeps.
- State: documentation only, implementation still on hold, changes uncommitted; user-owned files/settings preserved. No application code, helpers, manifests, runnable configuration, services, or network experiments added.
- Validation: all seven Markdown files passed local-link/heading-anchor and trailing-whitespace checks. Primary choice identifiers, unchanged feature/proposal implementation states, incomplete roadmap action 2, documentation-only inventory, confirmed phase-sequence scope, pending T14 decision, and `git diff --check` passed. No application/runtime tests performed.

### 2026-10-11 06:24 UTC+03:00 Measurement pattern confirmed and contract milestone completed

- After explanation, user confirmed T14's independently scheduled data in both directions plus correlated application receipts, adding that information is carried in the payload. Updated `CHOICES.md` and `DESIGN.md` with this dated decision and logical payload content: measurement metadata/identities plus configurable test content, and receipt correlation/receiver processing interval where available. Exact encoding/widths and adapter mappings remain open; no final schema selected.
- Clarified size accounting: example 128 bytes means configurable test content, with measurement metadata/framing, receipt bytes, and outer overhead recorded separately. Timing values remain host-local; mandatory identities are preserved through authentic protocol payloads and do not replace authentication. Added a payload/framing acceptance example for later authorized implementation.
- Reviewed action 2's outcome: lifecycle, identities, independent scheduling, timing boundaries, delivery accounting, logical result/dashboard fields, configurable steady/burst phase sequences, paired receipt model, pairing/user-certificate flows, editable example profile, synthetic expected-report counts, and future acceptance checks are documented. T15 numerical values were not adopted as defaults. Exact resource/phase rules, schemas, credential lifecycle and recommendation evidence thresholds remain explicit refinements for integration and agreed-slice planning.
- Marked action 2 complete and appended `(completed)` to its heading in `ROADMAP.md` automatically; action 3 is next/planned. Updated this current-state index and F07/technical-choice progress in `PROPOSALS.md`, linked to this evidence. Completion is a conceptual documentation milestone, not a claim that every schema/mechanism is finalized or runtime behavior exists.
- State: documentation only, implementation still on hold. F01-F07 remain unimplemented and S01-S06 proposed. Changes uncommitted; unrelated user files/settings preserved. No application code, helpers, manifests, runnable configuration, services, or network experiments added.
- Validation: all seven Markdown files passed local-link/heading-anchor and trailing-whitespace checks. Primary T01-T18 identifiers, unchanged foundation decisions, confirmed T14 payload/receipt choice, F01-F07 implementation-not-started/S01-S06 proposed states, exact `(completed)` headings/states for actions 1 and 2, planned documentation-only action 3, synthetic count reconciliation, documentation-only inventory, and `git diff --check` passed. No application/runtime tests performed.
