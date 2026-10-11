# Repository instructions

## Scope and authorization

sneaky-port measures which real protocol, DNS mode, ports, and directional packet priority settings suit FPS traffic between Windows 11 clients and one Linux server. The future gaming proxy is a separate project.

**Implementation is on hold.** Current authorization covers documentation and workspace cleanup only. Do not add application code, runtime helpers, dependency manifests, runnable configuration, deploy services, or execute network experiments until the user authorizes that work. A later explicit implementation request supersedes this hold for its stated scope; do not seek redundant permission.

## Before each task

1. Read the current-state index and recent entries in [WORK_LOG.md](WORK_LOG.md). Search its history for the feature or area being changed instead of rereading an ever-growing log in full.
2. Inspect repository status and relevant current files. Verify prior work exists and still behaves as recorded before deciding to implement it again. The code is the source of truth for implementation; the log can be stale or incomplete.
3. Consult the relevant sections of [DESIGN.md](DESIGN.md), the single product/design reference. Latest direct user instructions take precedence; update affected documentation when intent changes.
4. Consult [PROPOSALS.md](PROPOSALS.md) to track forward: accepted future features, optional ideas, decisions, and progress. Distinguish confirmed scope from unapproved suggestions and execution authorization. Keep future features and suggestions there with stable identifiers.
5. Consult [CHOICES.md](CHOICES.md) for technical questions, options, recommendations, and confirmed answers. Keep technical choice discussions there with stable T identifiers; do not infer acceptance or execution authorization from recommendations.
6. Consult [ROADMAP.md](ROADMAP.md) for the next few actions. Update its step status as work progresses. After completing a task's stated outcome and required checks, automatically append ` (completed)` to its heading (for example, `## 1. Settle the immediate foundation choices (completed)`) and mark its state complete in `ROADMAP.md` before the final report; never wait for a user reminder. Keep incomplete or partial tasks accurately marked. Expand the plan only as needed, without treating planned work as authorization.

An existing feature may still need a fix or an explicitly requested change. Do not duplicate it, skip required work because an old entry says completed, or promote a documented plan to implemented without evidence.

## Essential constraints

- Preserve authentic tool-managed TCP, UDP, TLS, QUIC, WireGuard, Hysteria2, and DNS coverage. Manual setup of separate tunnel/proxy services is not the intended product workflow.
- Include direct and resolver-mediated DNS data paths plus port controls. Treat DNS priority as an ISP-specific hypothesis, not a guarantee.
- Packet priority is independently configurable in both directions; apply it to actual outer traffic and distinguish requested, applied, and observed settings. No product shaping, bandwidth control, queue management, or automatic route switching here.
- Preserve independent probe scheduling, monotonic local timing, honest loss/late-delivery accounting, and evidence-based FPS comparisons. Do not invent one-way delay or infer zero network loss from eventual stream delivery.
- Preserve unrelated user files/settings. Leave changes uncommitted unless requested. Report unsupported capabilities; never silently substitute protocols, ports, or priority settings.

## After meaningful changes

Update the current-state index and append a dated entry to [WORK_LOG.md](WORK_LOG.md): what changed and why, affected paths/symbols, actual implementation status, checks performed/results, and remaining gaps. Use Europe/Istanbul timestamps (UTC+03:00) when the time is known. Do not fabricate times or test results.

Keep historical entries; record fixes, extensions, or reversions as later entries. Mark partial or unvalidated work accurately. Update the matching feature/proposal status in `PROPOSALS.md` and link it to the work-log evidence, preserving identifiers and decisions. Record completed behavior backward in `WORK_LOG.md`; do not treat a forward plan as code already implemented.

Record technical answers and decision dates in `CHOICES.md`, and append significant decision changes to `WORK_LOG.md`. Keep unresolved alternatives in the choices file; update `DESIGN.md` when a confirmed decision affects product behavior.

For documentation, check links, whitespace, and scope consistency. For later authorized code, use the applicable acceptance checks in `DESIGN.md`; distinguish build, loopback, platform, impairment, and live-network evidence. Run checks appropriate to the change without claiming untested coverage.

Keep this instruction file concise. Product details belong in `DESIGN.md`, technical questions/answers in `CHOICES.md`, near-term actions in `ROADMAP.md`, forward feature tracking in `PROPOSALS.md`, and backward tracking/current implementation evidence in `WORK_LOG.md`. Avoid creating another instruction layer or redundant skill without a concrete need.
