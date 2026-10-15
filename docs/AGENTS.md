# Repository instructions

<a id="scope-and-authorization"></a>

## Scope

sneaky-port measures FPS connection quality; the gaming proxy is separate. The user authorized the UDP/TCP first slice on 2026-10-11. Shipping targets are Windows 11/Linux x86-64; ARM was removed on 2026-10-13. Future protocols retain their [scope/status](ROADMAP.md#future-features-and-proposals). Latest user instructions win.

## Task workflow

- Inspect Git status, affected files and [WORK_LOG.md](WORK_LOG.md). Verify behavior in code; old log claims may be stale. Consult [DESIGN.md](DESIGN.md) for requirements/decisions and [ROADMAP.md](ROADMAP.md) for tasks/features. Search [ARCHIVE.md](ARCHIVE.md) only for relevant older context.
- Work on the selected task. Split large work into checkable outcomes; do not absorb adjacent work without authorization. Make routine choices autonomously within the authorized scope.
- Batch independent reads/checks and keep updates concise. Never skip required validation to save time or tokens.
- After meaningful changes, update affected docs/statuses and append dated Europe/Istanbul evidence: what/why, paths, checks/results, failures and gaps. Mark a finished roadmap heading with exact ` (completed)` only after its required checks; report the next step.
- At relevant milestones, suggest eligible F/S features with benefit, prerequisites and remaining decisions. Proposed/accepted scope does not itself authorize implementation; avoid repeated suggestions without new evidence.
- Preserve unrelated files, settings, services and uncommitted work. Leave changes uncommitted unless requested.
- Comment every function, including test/helper functions, with purpose and timing/accounting limits. Link useful implementation notes directly to source; no separate flowchart is maintained.
- Run checks appropriate to the change. Distinguish build, component/loopback, WSL, native, impairment, rendered-browser and live evidence. Current Linux runtime testing is user-deferred.
- After validation, remove task-owned temporary binaries, logs, fixture stores/profiles/reports and smoke staging/archives. Keep concise results in the log; preserve user data, current deliverables and reusable dependencies. Prefer automatically cleaned test directories.

<a id="essential-constraints"></a>

## Measurement constraints

Preserve authentic adapters and shared engine/UI/config/report contracts, direct/resolver DNS and port controls, independent outer-traffic priority, monotonic timing and honest skipped/late/loss/stall accounting. Require calibrated bounds/uncertainty for one-way claims and relevant capture evidence for priority claims. Unsupported requests reject visibly. No silent carrier/port substitutions, shaping, queue management, automatic routing, automatic winner or numeric confidence score.

<a id="keep-documentation-concise"></a>

## Documentation

Keep every Markdown file short and canonical. Preserve stable F/S/T IDs, decisions/provenance, open questions, evidence limits and dates. Current summaries belong in `WORK_LOG.md`, older summaries in `ARCHIVE.md`, decisions in `DESIGN.md` and features/actions in `ROADMAP.md`. When summarizing committed history, record the exact Git revision for retrieving the full text. Check links/anchors, scope and whitespace; do not create duplicate snapshot documents.
