# Work log

Current state and recent dated summaries. [ARCHIVE.md](ARCHIVE.md) summarizes older records and pins the full pre-summary text; [DESIGN.md](DESIGN.md) owns the contract/decisions and [ROADMAP.md](ROADMAP.md) actions/features. Verify old claims against code; record actual checks and gaps, not inferred acceptance.

## Current state

**Current UDP/TCP implementation is complete in its recorded local scopes; full release acceptance remains incomplete.** Shipping is Windows 11/Linux x86-64 only. Latest Linux startup checks are user-deferred. Production one-way bounds, selected native minimum/compiler/race, remaining outer observations, valid reference bands and live/ISP/external-load trials remain open.

| Area | Implemented / recorded evidence | Limit |
| --- | --- | --- |
| Engine/trust | Strict plans, authenticated SSH/data binding, independent UDP/TCP phases/queues/accounting and bounded finite lifecycle | Named component/loopback/WSL/native cases only |
| Reports | Endpoint-local JSONL/manifests, exact replay/exports, authenticated transfer/reconciliation, causal one-way models | Missing receiver evidence stays unknown; production one-way estimate/unavailable |
| Client/API | Protected loopback saved configuration/run/report controls, exact comparisons, Test-only dashboard/live RTT/S06/preflight | No automatic winner or network-loss/ISP attribution |
| Browser | Current Test dashboard connected-Chrome QA passes: exact edits/layout/keyboard/results/cancellation/recovery/preflight/conflicts/cleanup | Browser fixtures model Linux upload/shell; broader native/live gates separate |
| Native VMware | Ubuntu 24.04.4 OpenSSH/helper/direct-data/report/cleanup, ordinary package processes and isolated kernel/marking cases pass | Cross-built binaries; not Ubuntu 22.04.5 minimum or native compiler/race |
| Port/priority | Three alternating port pairs/carrier and named endpoint captures pass; native Linux IPv4/IPv6 marking cases pass | Windows required DSCP 46 rejects at qWave; NAT/global IPv6/remaining capture qualifications open |
| References | Pinned external builds/smokes and exploratory compatible overlaps recorded | iperf2 pacing warnings and incomplete independent bands/held-out acceptance |
| Release/CI | First preview published; pinned CI/release workflows and local repairs/checks exist | Local checks do not prove a later tagged hosted release is repaired |
| Startup | Windows no-argument dashboard/per-user store; optional strict profile; Linux22022 compiled defaults/generated state | Windows checks/builds pass; latest Linux startup runtime is deferred |
| Documentation/cleanup | Root README plus eight docs; direct source links, concise history and post-test scratch cleanup | Full older Markdown pinned in Git; raw ignored test artifacts were removed |

## Named native and reference evidence

The 2026-10-15 VMware case used Ubuntu **24.04.4**, kernel **6.17.0-14-generic**, x86-64 and source revision **36edcdd**, with Windows 11 on-link IPv4. There were **14 completed/two cancelled** runs. Completed client main populations totaled **420 planned, 418 written/on-time and two local skips**. Cancellations stayed partial; missing authenticated remote delivery remained unknown.

Guest capture had **2,768 packets**; Windows Npcap had **5,536**, including **2,768 identical IP/transport records**. Both reported zero capture drops; **1,084 unique UDP payload/endpoint tuples** matched. Named main runs observed server DSCP 12/client DSCP 0, ECN 0 and 30 distinct identities per direction. This is not radio-wire/NAT/global-IPv6/offload/checksum or underlying packet-loss qualification.

All **16 cross-built ordinary package binaries** ran successfully as Linux processes; control took **27.15 s**, source-purpose audit covered **1,170 functions**. The **12 isolated kernel cases** passed in **29.92 s** and **four AF_PACKET IPv4/IPv6 marking cases** in **0.08 s**. Guarded namespace evidence enabled reviewed execution after an initial isolation rejection; no guest real-interface/route/firewall/QoS settings were changed. These tests predate the latest deferred server-startup path.

Native Windows iperf2 retained timer-handle pacing warnings. Four Crusader scenarios completed in isolated Linux WSL2. IRTT's supported 127.0.0.2 bind avoided its Windows MTU-sentinel buffer failure without a source patch; ten exploratory pairs recorded **200 product/199 reference completions and one reference skip**. Valid compatible baseline/held-out bands remain task 33.

## Recording rules

Keep this index and recent dated results short. Record known Europe/Istanbul timestamps, what/why, paths, actual checks/failures and evidence boundaries. Mark completed roadmap outcomes only after their checks. Preserve IDs/provenance and pinned full-history revisions when summarizing; no read-only log entry is needed. Remove agent-owned test scratch after validation, preserving user data/current deliverables/dependencies.

<a id="2026-10-11-112529-utc0300-bounded-roadmap-tasks"></a>
<a id="2026-10-11-bounded-writer-queues"></a>
<a id="2026-10-11-windows-race-instrumentation"></a>
<a id="2026-10-11-protocol-traffic-tweak-requirement"></a>
<a id="2026-10-11-documentation-state-consistency"></a>
<a id="2026-10-11-archive-created-and-active-records-shortened"></a>
<a id="2026-10-11-proposal-suggestions-at-suitable-milestones"></a>
<a id="2026-10-11-independent-udp-workers"></a>
<a id="2026-10-11-udp-phase-accounting"></a>
<a id="2026-10-11-tcp-integration"></a>
<a id="2026-10-11-finite-run-lifecycle"></a>
<a id="2026-10-11-current-traffic-smoke-check"></a>
<a id="2026-10-11-windows-to-ubuntu-udp-enrollment-and-traffic"></a>
<a id="2026-10-11-local-evidence-persistence"></a>
<a id="2026-10-11-local-report-replay-and-export"></a>
<a id="2026-10-11-authenticated-server-evidence-transfer-and-reconciliation"></a>
<a id="2026-10-11-repository-privacy-cleanup"></a>
<a id="2026-10-11-live-ssh-clock-sampling"></a>
<a id="2026-10-11-uncertainty-gated-one-way-integration"></a>
<a id="2026-10-11-protected-local-run-api"></a>
<a id="2026-10-11-saved-configuration-and-report-api"></a>
<a id="2026-10-11-dashboard-setup-and-run-controls"></a>
<a id="2026-10-12-example-evidence-budget-repair"></a>
<a id="2026-10-12-diagnostic-and-distribution-views"></a>
<a id="2026-10-12-matched-saved-run-comparisons"></a>
<a id="2026-10-12-current-launch-state-and-linux-build-refresh"></a>
<a id="2026-10-12-adapter-extension-gate"></a>
<a id="2026-10-12-windows-to-linux-cli-validation"></a>
<a id="2026-10-12-s06-evidence-presentation-choice"></a>
<a id="2026-10-12-current-udptcp-release-choice-resolution"></a>
<a id="2026-10-12-native-arm64-client-prerequisite-audit"></a>
<a id="2026-10-12-dashboard-evidence-and-live-progress"></a>
<a id="2026-10-12-dashboard-section-navigation"></a>
<a id="2026-10-12-dashboard-password-ssh-workflow"></a>
<a id="2026-10-12-copied-helper-startup-repair"></a>
<a id="2026-10-12-managed-helper-and-top-connection-bar"></a>
<a id="2026-10-12-dashboard-usability-and-automatic-results"></a>
<a id="2026-10-12-single-page-conditional-dashboard-settings"></a>
<a id="2026-10-12-test-only-dashboard-and-sixty-second-repair"></a>
<a id="2026-10-13-numeric-receipts-and-live-latency"></a>
<a id="2026-10-13-controlled-validation-and-x86-64-release-scope"></a>
<a id="2026-10-13-reference-runtime-and-rendered-browser-progress"></a>
<a id="2026-10-14-final-release-evidence-review"></a>
<a id="2026-10-14-first-github-prerelease-completed"></a>
<a id="2026-10-14-github-actions-workflow-files-completed"></a>
<a id="2026-10-14-windows-ci-checkout-formatting-repair-completed"></a>
<a id="2026-10-14-windows-ci-grace-cancellation-fixture-repair-completed"></a>
<a id="2026-10-14-ci-live-clock-and-controlled-delay-fixture-repair-completed"></a>
<a id="2026-10-14-ci-controlled-population-fixture-repair-completed"></a>
<a id="2026-10-14-ci-lifecycle-and-comparison-deadline-repair-completed"></a>
<a id="2026-10-14-ci-reconciliation-and-empty-rtt-fixture-repair-completed"></a>

## History

Earlier 2026-10-10/14 records are summarized in [ARCHIVE.md](ARCHIVE.md#implementation-history-through-2026-10-14). Those records include tasks 5-23/S06, dashboard evolution, QPC timing repairs, controlled cases, reference/browser work and CI fixture repairs. The first preview **v0.1.0-alpha.1** was recorded as published on **2026-10-14 07:00:34 +03:00** from **5594182**; this is historical, not a claim about a newer tagged job.

## Recent changes

### 2026-10-15 VMware Ubuntu access check

The supplied guest was checked through normal OpenSSH before deployment. Connection/topology evidence did not establish the selected Ubuntu 22.04.5 baseline.

### 2026-10-15 VMware deployment and network validation

Normal OpenSSH, matching helper upload/control forwarding and direct UDP/TCP trials completed on the named Ubuntu 24.04.4 on-link path. Exact populations/capture/native-process/kernel limits are summarized above; no native compiler/race or Internet/ISP claim.

### 2026-10-15 Built-in server defaults and port 22022

Compiled service ceilings, generated private state, app SSH 22022 and client-supplied plans were implemented. Windows ordinary/vet/focused race and Linux cross-build pass; unreachable/failed Linux probes are not runtime success. The user deferred Linux validation; checkpoint 23o remains incomplete.

### 2026-10-15 Short README and operator guide

README was reduced from 286 to 35 content lines and detailed commands moved to USAGE.md; requirements/evidence boundaries were retained.

### 2026-10-15 Markdown files moved into docs

All non-root-README source Markdown moved into docs, with rebased links and release template/notices paths. Local Windows package inventory/helper/hash/extraction/native read-only smokes passed; no publication or Linux runtime claim.

### 2026-10-15 Generated artifact cleanup

Removed 44,111 generated files totaling 5,529,223,551 bytes after archive/protected-hash checks. Current source/binaries/configuration/reports and published evidence outside named targets were retained at that stage.

### 2026-10-15 Test-output artifact cleanup

Removed 36 obsolete binaries/launchers/checkers totaling 246,507,184 bytes; 390 protected hashes matched. At that point 211 readable files/72,323,079 bytes plus seven inaccessible stores remained; those are historical counts.

### 2026-10-15 Optional profile and built-in client settings

Added fresh compiled starter plans and shared strict optional-profile loading. Full Windows owned-source ordinary/vet, focused race, actual no-profile startup, both builds and helper equality passed. A native probe JSON parser was corrected; no Linux runtime claim.

### 2026-10-15 Zero-argument Windows client startup

Windows empty/leading-flag invocation selects client-api and the per-user store; explicit stores/commands are preserved without migration. Protected isolated startup/ordinary/vet/race and packaged/build/helper checks passed; Linux retains server role.

### 2026-10-15 Sample configuration folder removed

Deleted examples and moved fixture/packaging dependencies to typed defaults. Windows affected tests/package/JSON override/docs checks passed; an existing 100-ms second-start lease test first returned EOF during concurrent builds, then the full serial control suite passed in 45.392 s without a fixture change. Linux tests were compile-checked only.

### 2026-10-15 Cleanup after validation

Added the standing scratch-cleanup rule. Removed 8,916 files/965,633,945 bytes; 183 protected hashes matched. Current deliverables/user state/dependencies remained; concise outcomes replace retained scratch artifacts.

### 2026-10-15 Old cache artifacts removed

Removed 3,083 old cache files/1,762,982,718 bytes, including two protected fixture stores after reviewed broader access. 3,310 protected hashes matched. Remaining reusable cache was 3,127 files/97,655,467 bytes in go-mod, acceptance-references and workflow-tools; no ACL/service settings changed.

### 2026-10-15 Markdown documentation consolidated

Deleted FLOWCHART.md; merged CHOICES.md into DESIGN.md and PROPOSALS.md into ROADMAP.md, removed their separate files and fixed links/rules. Nine Markdown files/482 local links, all 47 IDs, decisions/status/history and unchanged release inputs passed documentation checks.

### 2026-10-15 Markdown files summarized

Recorded: 2026-10-15 18:00 UTC+03:00 (Europe/Istanbul).

- User requested summaries of every repository Markdown file. Shortened all nine files; removed duplicated descriptions and retained contracts/commands, all 47 F/S/T IDs, decisions/provenance, open questions and explicit acceptance gaps. Dated historical titles remain indexed; complete pre-summary Markdown is pinned at `046eb60ce740064974ccc2983490de64552e678a` in [ARCHIVE.md](ARCHIVE.md#full-historical-record).
- Legacy bookmarks are retained as compact anchors. License texts remain unchanged; release placeholders and operator/legal links remain valid. Product source/current binaries/user data and task completion states are unchanged.
- Validation passed: nine files/80 local links, 127 legacy anchors, 47 IDs, 52 unchanged task headings, 96 historical titles, 33 source/license references, five release placeholders and seven parsed PowerShell examples; JSON examples/tables/fences/whitespace are valid. Source Markdown reduced from 700,809 to 103,954 bytes. No Go/Linux test execution, publication, commit, memory update or retained scratch files.
