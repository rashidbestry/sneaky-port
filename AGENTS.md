# Repository instructions

## Scope and authorization

sneaky-port measures FPS connection quality between Windows 11 clients and one Linux server. The gaming proxy is a separate project.

**First-slice implementation authorized, 2026-10-11.** The user requested starting roadmap step 5: implement and validate the agreed UDP/TCP slice in `DESIGN.md`. Other carriers and later features retain their existing scope/status. Preserve user-owned services and settings.

## Task workflow

- Optimize context and tokens: use targeted searches/reads and concise updates, batch independent checks, and reuse verified results. Expand context only when needed; never skip required work or validation to save tokens.

- Read [WORK_LOG.md](WORK_LOG.md)'s current index and recent/relevant history; inspect Git status and affected files. Verify existing behavior from code before duplicating or fixing it; log claims can be stale.
- Consult relevant sections of [DESIGN.md](DESIGN.md) (requirements), [CHOICES.md](CHOICES.md) (technical answers), [PROPOSALS.md](PROPOSALS.md) (feature status), and [ROADMAP.md](ROADMAP.md) (next actions). Latest user instructions win; plans/recommendations do not imply acceptance or execution authorization.
- After meaningful changes, update affected documents/statuses and the work-log index; append dated evidence with what/why, paths, actual state, checks/results and gaps. Use known Europe/Istanbul timestamps; never invent results. Preserve stable F/S/T identifiers, decision provenance and historical log entries.
- Automatically mark finished roadmap outcomes complete and append ` (completed)` to their headings after required checks. Keep partial/unvalidated work explicit; planning completion is not implementation.

## Essential constraints

- Preserve real, tool-managed TCP/UDP/TLS/QUIC/WireGuard/Hysteria2/DNS coverage and extensible adapters using shared measurement/UI/configuration/reporting contracts.
- Include direct/resolver-mediated DNS and port controls; DNS prioritization is an ISP-specific hypothesis. Independent directional priority applies to actual outer traffic, with requested/applied/observed evidence. No product shaping, queue management or automatic routing.
- Preserve independent scheduling, local monotonic timing, honest loss/late/stream-stall accounting and evidence-based comparisons. One-way delay needs valid clock calibration and uncertainty; eventual stream delivery does not prove zero network loss. Report unsupported capabilities without silent substitutions.
- Preserve unrelated files/settings; leave changes uncommitted unless requested. Check documentation links, whitespace and scope. Later code uses applicable `DESIGN.md` acceptance gates; distinguish build, loopback, native platform, impairment and live-network evidence.
- Comment every function, including test/helper functions, explaining its purpose and any important timing/accounting limits. Maintain `FLOWCHART.md` against actual code with source links; distinguish implemented paths from remaining work.

## Keep documentation concise

Review affected Markdown guidance during meaningful changes. Summarize oversized sections, remove repetition/stale instructions and link to the canonical document instead of copying details. Keep active instructions and next actions short; do not create redundant instruction files or skills.

Preserve requirements, stable identifiers, decisions, open questions and evidence when shortening. Keep dated history in `WORK_LOG.md`; read it selectively rather than loading it all. Log significant compaction and any remaining documentation cleanup there.
