# Project archive

Historical summaries, not current instructions or runtime qualification. Current answers are in [DESIGN.md](DESIGN.md#technical-choices), statuses in [ROADMAP.md](ROADMAP.md), and recent evidence in [WORK_LOG.md](WORK_LOG.md). Original dates/titles are retained; the full previous Markdown is available from the [pinned Git revision](#full-historical-record). Earlier raw test artifacts were cleaned at the user's request.

## Archived work log

These 2026-10-10/11 records establish requirements, accepted choices and the initial implementation authorization. Later scope/status changes supersede them.

### 2026-10-10 — Initial requirements and documentation

Captured measurement-only scope: Windows/Linux clients, one Linux server, real tool-managed protocols, DNS experiments, ports, independent directional packet priority, and FPS delivery quality.

### 2026-10-10 — Optional suggestions saved

Created the suggestions file originally named `NotImplementedYet.md` (now [feature proposals](ROADMAP.md#future-features-and-proposals)) with S01-S06: staged experiments, burst metrics, coordination separation, size/rate sweeps, exportable recommendations, and separate quality/confidence.

### 2026-10-10 21:28 UTC+03:00 — Documentation consolidation and persistent work tracking

Consolidated confirmed requirements, protocol/DNS coverage, ports/priority, measurement methods, and acceptance checks into [DESIGN.md](DESIGN.md).

### 2026-10-10 21:33 UTC+03:00 — Forward proposals and backward work history

Renamed `NotImplementedYet.md` to [feature proposals](ROADMAP.md#future-features-and-proposals), preserving S01-S06 and their unapproved status.

### 2026-10-11 05:20 UTC+03:00 — Initial technical decisions

User confirmed Go (T01) and a local browser dashboard (T02).

### 2026-10-11 05:29 UTC+03:00 — Dedicated technical choices file

Created [technical choices](DESIGN.md#technical-choices) at the user's request and moved the existing T01-T11 table and research notes out of `DESIGN.md`.

### 2026-10-11 05:45 UTC+03:00 — Initial roadmap

Created [ROADMAP.md](ROADMAP.md) at the user's request with only the first actions: settle immediate choices, define a measurement contract, review integration constraints, and agree on an explicitly authorized first implementation slice.

### 2026-10-11 05:48 UTC+03:00 Foundation discussion started

Started roadmap step 1 at the user's request.

### 2026-10-11 05:50 UTC+03:00 First foundation answers recorded

Updated current scope in `DESIGN.md`, `README.md`, `AGENTS.md`, and F01 in `PROPOSALS.md`; updated roadmap progress and this current-state index.

### 2026-10-11 05:52 UTC+03:00 Architecture and profile answers recorded

User clarified T12 as 64-bit ARM64 and x86-64 only for Windows 11 clients and Linux servers.

### 2026-10-11 05:59 UTC+03:00 Foundation decisions completed

Confirmed T08 from the user's answer: per-run JSONL raw observations, JSON summary/settings snapshot, and CSV export.

### 2026-10-11 06:01 UTC+03:00 Automatic roadmap completion tracking

Added the user's standing instruction to `AGENTS.md`: after a roadmap task's stated outcome and required checks are complete, mark it complete in `ROADMAP.md` automatically before reporting, without waiting for a reminder.

### 2026-10-11 06:05 UTC+03:00 Completed roadmap heading format

Added `(completed)` to action 1's heading in `ROADMAP.md`, matching the user's requested format.

### 2026-10-11 06:14 UTC+03:00 Measurement contract draft started

Started roadmap action 2 at the user's request.

### 2026-10-11 06:16 UTC+03:00 Configurable workload and report presentation answers

User requested configurable packet size, burst time, payload, and other traffic properties (T15), rather than selecting the proposed fixed numbers.

### 2026-10-11 06:18 UTC+03:00 Traffic phase sequences and receipt clarification

User extended T15's proposed steady/burst controls with an ordered sequence of different traffic phases.

### 2026-10-11 06:24 UTC+03:00 Measurement pattern confirmed and contract milestone completed

After explanation, user confirmed T14's independently scheduled data in both directions plus correlated application receipts, adding that information is carried in the payload.

### 2026-10-11 Integration constraints review completed

Review recorded at 06:35 UTC+03:00 (Europe/Istanbul).

### 2026-10-11 Reference-tool planning recorded

Added a reference inventory in `PROPOSALS.md` for IRTT, iperf2, Crusader, Flent, Graphite Meter, sockperf, dnstt, iodine and bufferbloat-lab, mapped to existing F01-F07/T13-T18.

### 2026-10-11 Protocol extensibility requirement recorded

Added `DESIGN.md` extensibility requirements and future acceptance criteria: a common adapter lifecycle/probe/capability/options contract, shared timing/accounting, capability-driven dashboard/configuration/reporting, stable identifiers and compatible versioned profiles/results, and explicit unsupported outcomes.

### 2026-10-11 Selective reference review completed

User requested the next roadmap task, action 3b.

### 2026-10-11 Control and clock calibration requirements recorded

User clarified that a server connection must communicate the traffic being tested and maintain a cross-host time relationship for one-way delay.

### 2026-10-11 StarTrinity reference review recorded

Read official Network Tester, CST FAQ/custom-server deployment, SIP Tester product/tutorial and Windows DSCP guidance.

### 2026-10-11 First-slice discussion started

Added a concrete proposed deliverable, feature/addition mapping, first answer batch, necessary mechanics and success criteria in `CHOICES.md`.

### 2026-10-11 First-slice scope and SSH control confirmed

Updated T22 and action 4 discussion in `CHOICES.md`, confirmed slice/control intent in `DESIGN.md`, forward scope progress in `PROPOSALS.md`, in-discussion state in `ROADMAP.md` and this index.

### 2026-10-11 Tool-owned SSH lifecycle confirmed

Updated T22 and next T09/T14/T21 discussion in `CHOICES.md`, confirmed control/lifecycle intent in `DESIGN.md`, action 4 progress in `ROADMAP.md`, matching F01/F09 design progress in `PROPOSALS.md` and this index.

### 2026-10-11 Client-local keys and skip accounting confirmed

User requested explanation of compile-time registration versus runtime plugins, not acceptance.

### 2026-10-11 First-slice planning completed

Updated T21 to the user's confirmed answer.

### 2026-10-11 Documentation guidance condensed

User requested short, direct Markdown instructions, ongoing summarization as files grow, and tracking of that maintenance.

### 2026-10-11 First implementation foundations

Added `go.mod`/`go.sum`, `cmd/sneaky-port` and `internal/{profile,schedule,probe,adapter,platform,measure,calibration,control,quality,testutil}`.

### 2026-10-11 09:54 UTC+03:00 Token-efficient workflow

Added a concise `AGENTS.md` rule for targeted context, batched independent checks, reused verified results and brief updates, without skipping required work/validation.

### 2026-10-11 SSH pairing and readiness

Continued the user's authorized first slice after successful adapter/preflight checks.

<a id="index"></a>
<a id="technical-decisions"></a>
<a id="foundation-discussion"></a>
<a id="remaining-technical-choices"></a>
<a id="questions-for-the-next-discussion"></a>
<a id="additional-open-choices"></a>
<a id="measurement-contract-discussion"></a>
<a id="integration-constraints-review"></a>
<a id="managed-protocols-sockets-and-ports-t05-t11"></a>
<a id="directional-priority-and-observation-t11-t17"></a>
<a id="dns-prerequisites-framing-and-polling-t16"></a>
<a id="linux-and-packaging-boundaries-t12"></a>
<a id="handoff-to-the-first-slice-discussion"></a>
<a id="reference-tool-reuse-and-validation"></a>
<a id="selective-source-review-and-dispositions"></a>
<a id="measurement-differences-that-constrain-t20"></a>
<a id="license-maintenance-and-platform-boundaries"></a>
<a id="validation-handoff-and-optional-scope-decisions"></a>
<a id="protocol-extensibility-choices"></a>
<a id="control-connection-and-one-way-timing-choices"></a>
<a id="startrinity-documentation-review"></a>
<a id="first-implementation-slice-discussion"></a>
<a id="proposed-deliverable"></a>
<a id="first-answer-batch-and-answers"></a>
<a id="ssh-lifecycle-and-authentication-discussion"></a>
<a id="pairing-extensibility-and-backpressure-discussion"></a>
<a id="mechanics-identified-during-discussion"></a>
<a id="proposed-success-criteria"></a>
<a id="first-slice-final-decisions"></a>
<a id="protocol-traffic-tweak-choices"></a>
<a id="maintaining-choices"></a>
<a id="s06-separate-connection-quality-from-measurement-confidence"></a>
<a id="task-24-native-windows-11-arm64-client"></a>

## Archived choices snapshot 2026-10-11

- The user confirmed Go/local browser, embedded assets, shared engine/adapters, typed profiles, JSONL/JSON/CSV, endpoint-local keys and trusted public enrollment. Configurable ordered phases and independent bidirectional data plus receipts replaced proposed fixed workload values.
- Compile-time adapter registration and tool-owned SSH were explicitly confirmed. Remaining first-slice mechanics were engineering selections under task-4 completion; first-slice implementation was then authorized. Current T01-T25 answers/provenance are canonical in [DESIGN.md](DESIGN.md#technical-choices).
- Source reviews distinguished SO_MARK routing from DSCP, arbitrary Windows marking privileges, recursive DNS/authoritative-hop limits and genuine carrier boundaries. IRTT/iperf2/Crusader were selected as external design/validation references, not bundled code. StarTrinity informed candidate diagnostics/monitoring ideas without adopting MOS/outage thresholds.
- S07-S11 were accepted for future scope on 2026-10-11. Later user instructions limited current work to UDP/TCP; S01-S05/S12 remain deferred proposals. Broader carrier/DNS/monitoring choices are still open.

## S06 discussion snapshot before release resolution, 2026-10-12

Earlier confidence/presentation alternatives are historical. The user selected per-metric coverage/basis/count/reason labels, no numeric confidence score or automatic winner. Current advisory targets and mandatory gates are in [DESIGN.md](DESIGN.md#presentation-comparison-and-acceptance-handoff) and [USAGE.md](USAGE.md#evidence-presentation-and-live-run-progress).

## Retired native ARM acceptance checklist

Tasks 24-25 originally required native ARM hardware and real runtime evidence. The user removed ARM on 2026-10-13; those cases are historical and impose no current release prerequisite.

## Implementation history through 2026-10-14

The following dated records are condensed from the former work log. A completed local task means its named scope/checks passed, not full release acceptance. Exact commands, quantities, failures and original caveats remain in the pinned text.

### 2026-10-11 11:25:29 UTC+03:00 Bounded roadmap tasks

At the user's request, replaced the oversized original task 5 with separately checked tasks 5-35 in ROADMAP.md.

### 2026-10-11 Bounded writer queues

Completed selected task 7 only (F02/F07, T06/T15).

### 2026-10-11 Windows race instrumentation

User supplied successful terminal output from the current Windows x86-64 checkout after installing MSYS2 UCRT64 GCC 16.2.0 (Rev4) and setting session-local PATH, CC=gcc, CGO_ENABLED=1 and workspace caches. `go test -race ./internal/writer -count=20` passed (3.250 s); `go test -race ./...` passed every tested package with no race report.

### 2026-10-11 Protocol traffic tweak requirement

User requested protocol selection with related traffic tweaks, including fragmentation and header/payload sizes.

### 2026-10-11 Documentation state consistency

Corrected stale design-only/implementation-hold and execution-gate statements in `DESIGN.md` and `CHOICES.md` against current code, roadmap and recorded evidence.

### 2026-10-11 Archive created and active records shortened

Kept the current-state index and newest five prior entries in `WORK_LOG.md`; condensed `CHOICES.md` to all T01-T25 current answers, first-slice selection provenance and open details.

### 2026-10-11 Proposal suggestions at suitable milestones

Updated this index.

### 2026-10-11 Independent UDP workers

Completed selected task 8 only (F02/F07, T06/T15).

### 2026-10-11 UDP phase accounting

Completed selected task 9 only (F02/F07, T14/T15/T18), in three checked outcomes: multi-phase UDP workers, event/ticket-ledger integration, and metric/boundary fixtures.

### 2026-10-11 TCP integration

Completed selected task 10 only (F02/F07, T06/T14/T15/T18), in three checked outcomes: shared worker integration, complete/partial framing, and accounting/platform fixtures.

### 2026-10-11 Finite run lifecycle

Completed selected task 11 only (F01/F02/F06/F07/F09, T09/T11/T14/T15/T17/T22), in three checked outcomes: authenticated start/control ownership, finite phase/grace/cancellation engine, and CLI/platform fixtures.

### 2026-10-11 Current traffic smoke check

User asked whether traffic can run now.

### 2026-10-11 Windows-to-Ubuntu UDP enrollment and traffic

Diagnosed the user's SSH public-key rejection.

### 2026-10-11 Local evidence persistence

Completed selected task 12 only (F07/F09, T08/T14/T22).

### 2026-10-11 Local report replay and export

Completed selected task 13 only (F07/F09, T08/T14/T22).

### 2026-10-11 Authenticated server evidence transfer and reconciliation

Selected task 14 only (F07/F09, T08/T14/T22), split before implementation into authenticated bounded transfer, directional reconciliation, and persisted replay/export/fault validation.

### 2026-10-11 Repository privacy cleanup

User requested cleanup after a read-only audit confirmed the GitHub repository is public, its main branch matches the local commit, and 15 branch-history commits / 218 file versions contain no detected private-key markers, passwords or recognizable service/API tokens.

### 2026-10-11 Live SSH clock sampling

Completed selected task 15 only (F09, T22/T23), split into authenticated capture, bounded sampling/persistence and fault/platform fixtures.

### 2026-10-11 Uncertainty-gated one-way integration

Completed selected task 16 only (F09, T22/T23), split into causal model revisions, matched directional report/replay/export integration and timing/fault fixtures.

### 2026-10-11 Protected local run API

Completed selected task 17 only (F01/F08/F09, T06/T14/T22), in three checked outcomes: loopback request protection, finite-run ownership/idempotency, and security/lifecycle/platform fixtures.

### 2026-10-11 Saved configuration and report API

Completed selected task 18 only (F01/F07/F08/F09, T06/T14/T22), in three checked outcomes: saved profile revisions, public pairing/client-key operations, and report replay/import/export.

### 2026-10-11 Dashboard setup and run controls

Completed selected task 19 only (F01/F08/F10, T04/T06/T14/T25), in three bounded outcomes: embedded protected-origin shell/setup, pairing/profile controls, and frozen run controls/recovery fixtures.

### 2026-10-12 Example evidence budget repair

Investigated the user's dashboard failure before measurement.

### 2026-10-12 Diagnostic and distribution views

Completed selected task 20 only (F01/F07, T13/T14, basic S08/S09) in three separately checked outcomes: protected saved diagnostic projection, native SVG/paginated views, and explicit local-preflight controls.

### 2026-10-12 Matched saved-run comparisons

Completed selected task 21 only (F05/F07, T10/T18/T14) in three checked outcomes: bounded saved comparison/provenance, protected API/dashboard integration, and deterministic/file/script/real sequential endpoint fixtures.

### 2026-10-12 Current launch state and Linux build refresh

Checked the user's requested current state/run instructions against source, README, task 20/21 entries and their recorded validation.

### 2026-10-12 Adapter extension gate

Completed selected task 22 only (F08, T06/T14/T21): startup registry propagation and a common-workflow test adapter.

### 2026-10-12 Windows-to-Linux CLI validation

Completed selected task 23 only (F01/F07/F09, T08/T09/T12).

### 2026-10-12 S06 evidence presentation choice

User selected A in the S06 discussion: per-metric evidence labels with specific reasons, rather than a numeric confidence score.

### 2026-10-12 Current UDP/TCP release choice resolution

User instructed resolving all decisions, then explicitly limited scope to the current UDP/TCP release.

### 2026-10-12 Native ARM64 client prerequisite audit

Selected task 24 only (F01/F07/F09, T08/T09/T12).

### 2026-10-12 Dashboard evidence and live progress

User deferred ARM versions/validation and authorized the remaining dashboard work together.

### 2026-10-12 Dashboard section navigation

Paths: `internal/localapi/web/{index.html,dashboard.css,navigation.js,dashboard.js,diagnostics.js,compare.js,dashboard_test.cjs}`, `internal/localapi/dashboard{,_test}.go`; canonical operator/requirements/choice/proposal/roadmap/source-flow documents updated.

### 2026-10-12 Dashboard password SSH workflow

Automatic access: `POST /api/v1/dashboard-session` requires exact loopback authority/Origin, same-origin fetch metadata, a custom bootstrap header, safe request identity and no body/query.

### 2026-10-12 Copied helper startup repair

Checkpoint 23f fixes two independent causes: optional absolute **Server executable** selection resolves copies outside PATH; accepted host pins are now persisted immediately after successful SSH authentication, independently of helper initialization.

### 2026-10-12 Managed helper and top connection bar

Managed setup (23g): `build-dashboard.ps1` builds the matching unbundled Linux x86-64 server, compresses its artifact and embeds it into tagged Windows/Linux dashboards.

### 2026-10-12 Dashboard usability and automatic results

23i normal workflow: new `web/workspace.js` and dark console/cards/chart CSS; normal sidebar is Test/Saved results/Compare, with complete previous settings/run diagnostics and JSON behind Advanced.

### 2026-10-12 Single-page conditional dashboard settings

Removed both sidebar columns and replaced conflicting layered CSS with a single viewport layout.

### 2026-10-12 Test-only dashboard and sixty-second repair

23l: removed Saved results/Compare sections, activity navigation, comparison script loading and comparison setup controls.

### 2026-10-13 Numeric receipts and live latency

Replaced both main plot panels with one client-origin attempt-to-receipt RTT chart: elapsed seconds, explicit ms axis labels and completed-sample envelopes.

### 2026-10-13 Controlled validation and x86-64 release scope

Authorization/scope: user requested the remaining tasks together, autonomous routine choices, then explicitly removed ARM from the project.

### 2026-10-13 Reference runtime and rendered browser progress

User reaffirmed current UDP/TCP release only and confirmed no native server is available.

### 2026-10-14 Final release evidence review

Independent final review found no additional locally actionable implementation work.

### 2026-10-14 First GitHub prerelease (completed)

At the user's request, published [v0.1.0-alpha.1](https://github.com/rashidbestry/sneaky-port/releases/tag/v0.1.0-alpha.1), the repository's first release, as a prerelease targeting clean existing commit `5594182658b86bc1fbc49f0e787fabac4dc0217f`.

### 2026-10-14 GitHub Actions workflow files (completed)

Added `.github/workflows/ci.yml`: branch push/PR/manual and reusable exact-revision checks on Windows Server 2025/Ubuntu 24.04 x86-64.

### 2026-10-14 Windows CI checkout formatting repair (completed)

Inspected the user-supplied `Downloads/logs_104376436249` bundle for commit `87d37b7be391458c200d10591cad0a5f95de68fc`.

### 2026-10-14 Windows CI grace cancellation fixture repair (completed)

Inspected user-supplied `Downloads/logs_104380511001` for commit `760ff0130d6afbbe063e5b959098b1bd7fddfd68`.

### 2026-10-14 CI live clock and controlled delay fixture repair (completed)

User-supplied `Downloads/logs_104385833959` checks commit `5d130ca59e9dfce5f23afed38ddd50933abe1ef5`.

### 2026-10-14 CI controlled population fixture repair (completed)

User-supplied `Downloads/logs_104396312335` checks commit `b7d264eebe4383abcc1eaf682df06d77367577fd`.

### 2026-10-14 CI lifecycle and comparison deadline repair (completed)

`Downloads/logs_104460933501` checks clean commit `fbe694665f2cb5ca15f2e047018feafcebe0c8d3`.

### 2026-10-14 CI reconciliation and empty-RTT fixture repair (completed)

`Downloads/logs_104484007996` checks clean commit `357c25a450274cbff87033857059387e1d1ff3a5`.

## Full historical record

All nine pre-summary Markdown files are preserved in local Git revision **046eb60ce740064974ccc2983490de64552e678a**. Retrieve any original file with `git show REVISION:PATH`, for example:

```text
git show 046eb60ce740064974ccc2983490de64552e678a:docs/ARCHIVE.md
git show 046eb60ce740064974ccc2983490de64552e678a:docs/WORK_LOG.md
git show 046eb60ce740064974ccc2983490de64552e678a:docs/DESIGN.md
```

This preserves complete prior wording without duplicate snapshot files. It does not restore ignored raw reports/logs already deleted during authorized cleanup. Historical ARM plans, former dashboards, paths and superseded options are not current requirements.
