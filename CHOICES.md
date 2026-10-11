# Technical choices

Use this file for technical questions, options, recommendations, and confirmed decisions. [DESIGN.md](DESIGN.md) contains product requirements, [PROPOSALS.md](PROPOSALS.md) tracks future features, and [WORK_LOG.md](WORK_LOG.md) records completed work and decision history.

A recommendation is not user acceptance, and a confirmed technology choice is not permission to implement. Preserve stable T identifiers. Implementation remains on hold.

## Technical decisions

Updated: 2026-10-11. Choosing technologies does not authorize implementation. T01-T04 and T06-T09 are confirmed; T12's roles/architectures/broad distribution scope and T13's frontend stack are confirmed. Version limits and remaining details are open; no integrations are validated.

| ID | Area | Choice / recommendation | State |
| --- | --- | --- | --- |
| T01 | Backend/application language | Go | Confirmed by user |
| T02 | First-release interface | Local browser dashboard | Confirmed by user |
| T03 | Packaging and roles | One executable per OS/CPU with client and server modes; supported deployments are Windows 11 client and Linux server | Confirmed by user, 2026-10-11 |
| T04 | Dashboard implementation | Embedded plain HTML/CSS/JavaScript assets, locally bundled with no runtime CDN dependency | Confirmed by user, 2026-10-11; chart library/layout open under T13 |
| T05 | Protocol integrations | Standard Go networking/TLS, quic-go, wireguard-go userspace netstack, Hysteria2 core, and a DNS message library with dedicated encrypted-DNS adapters | Recommended; feasibility validation required |
| T06 | Internal boundaries | One shared measurement engine; separate protocol adapters, platform port/priority support, and dashboard/API modules; UI does not drive probe scheduling | Confirmed by user, 2026-10-11 |
| T07 | Configuration | Versioned YAML profiles; file loading and dashboard edits share validation rules; dashboard edits require an explicit save | Confirmed by user, 2026-10-11; parser/schema open |
| T08 | Results | Per-run JSONL raw observations plus JSON summary/settings snapshot; CSV export for analysis | Confirmed by user, 2026-10-11; schemas open |
| T09 | Trust and pairing | Tool-generated pairing setup with secure transfer/import into the client dashboard; also support user-provided TLS certificates with explicit identity verification | Confirmed by user, 2026-10-11; credential handling details open |
| T10 | Experiment execution | Sequential profile comparisons with repeated/alternating runs and coordinated port lifecycle | Recommended; automatic screening remains optional S01 |
| T11 | Priority implementation | Platform-specific Linux and Windows marking backends; verify outer packets where observable; never equate API success with applied wire priority | Recommended; per-adapter/platform validation required |

Go's [embed package](https://pkg.go.dev/embed) supports bundling dashboard files. Recommend a localhost-only HTTP API for actions and an event stream for live summaries; keep UI updates bounded so rendering cannot drive probe scheduling. Protect local state-changing requests with session/origin controls. Exact endpoints, update rates, chart library, layout, and any optional asset build tooling remain open; plain HTML/CSS/JavaScript is confirmed.

For T05, the current preferred candidates are:

- TCP/UDP/TLS: Go's standard networking and TLS packages.
- QUIC: [quic-go](https://quic-go.net/docs/quic/datagrams/), with streams and datagrams reported separately and app-supplied socket integration assessed for priority/source-port control.
- WireGuard: [wireguard-go's userspace netstack](https://git.zx2c4.com/wireguard-go/tree/tun/netstack/examples/http_client.go), using real WireGuard without making separately configured OS tunnel services part of the normal workflow. This is a userspace implementation profile; do not claim it reproduces kernel WireGuard overhead. Outer binding/priority integration and both-platform behavior still need validation.
- Hysteria2: its Go [client configuration](https://raw.githubusercontent.com/HyNetworks/hysteria/master/core/client/config.go) and [server configuration](https://raw.githubusercontent.com/HyNetworks/hysteria/master/core/server/config.go) expose connection integration hooks. Prefer embedded core APIs; verify dependency compatibility, supported modes, cleanup, and markings before adoption. Hysteria2's QUIC implementation must not be conflated with the separate baseline QUIC adapter.
- DNS: evaluate the maintainer's [new Codeberg implementation](https://pkg.go.dev/codeberg.org/miekg/dns) alongside compatibility needs of [the original GitHub implementation](https://github.com/miekg/dns). The original repository now states it receives only specific fixes and may be archived; library selection is therefore explicitly unresolved. Verify the new API/version stability and socket integration before adopting it. DoH/DoQ transport support and project-specific data-tunnel framing must be assessed separately; a DNS library does not supply the full tunnel.

For T11, [Windows QOSSetFlow](https://learn.microsoft.com/en-us/windows/win32/api/qos2/nf-qos2-qossetflow) and its [DSCP operation](https://learn.microsoft.com/en-us/windows/win32/api/qos2/ne-qos2-qos_set_flow) are candidate mechanisms, not proven support across every adapter. Inspect actual socket ownership, OS permissions, and DSCP/ECN behavior before finalizing the backend. Preserve explicit unavailable/unknown outcomes and the existing requirement for independent directions.

Do not install dependencies or create manifests during this design stage. Pin compatible versions and review integration/license requirements when selection and implementation are authorized; the source inspection above is not a build or runtime test.

## Foundation discussion

Started and completed: 2026-10-11. Roadmap step 1's foundation answers are recorded: packaging, frontend stack, OS roles/architectures, broad Linux distribution scope, shared-engine boundaries, profiles, results, and pairing approach. Exact Linux version limits, chart/layout choices, schemas, and credential handling remain open for the following contract/integration discussions.

| Choice | Recommended starting point | Alternative / tradeoff | Answer status |
| --- | --- | --- | --- |
| T03 | One executable per OS/CPU with client/server modes | Separate role executables were considered. A single executable is a packaging target, subject to protocol integration constraints; it does not promise privilege-free or dependency-free operation. Supported roles follow T12. | Confirmed, 2026-10-11 |
| T04/T13 | Embedded plain HTML/CSS/JavaScript for the initial dashboard, with locally bundled assets and no runtime CDN | A compiled frontend framework was considered. Chart library, layout, and optional asset build tooling remain open; a frontend framework is not selected. | Stack confirmed, 2026-10-11; remaining T13 details open |
| T12 | Windows 11 client only; Linux server only; 64-bit ARM64 and x86-64 on both; broad major Linux distributions including CentOS, Ubuntu, Fedora, Arch, and other mainstream distributions | Supersedes the previous Windows/Linux client scope. User clarified 64-bit only: Go targets `windows/arm64`, `windows/amd64`, `linux/arm64`, and `linux/amd64`. Exact distribution/version support matrix, minimum kernel/userspace requirements, and actual server environment remain unspecified; review during integration work. No runtime support is validated. | Roles/architectures/distribution scope confirmed, 2026-10-11; version limits open |

Source check: Go's [embed documentation](https://pkg.go.dev/embed) supports embedding static web assets and serving them through `net/http`. This supports the bundling approach, not protocol integration or platform-support claims. Consult Go's [minimum requirements](https://go.dev/wiki/MinimumRequirements) and each selected integration when assessing requested platform targets; compiler support alone is insufficient.

Confirmed on 2026-10-11: T06 uses one measurement engine with protocol adapters, platform port/priority support, and separate dashboard/API modules. This module boundary does not adopt S03's separate coordination-path proposal. T07 uses versioned YAML profiles with explicit save and a single validation model shared by file loading and dashboard edits; JSON was considered as an alternative. YAML supports comments in its [format specification](https://yaml.org/spec/1.2.2/); Go provides [JSON encoding/decoding](https://pkg.go.dev/encoding/json). No parser dependency or exact schema has been selected.

Confirmed on 2026-10-11: T08 saves JSONL raw observations and a JSON summary/settings snapshot per run, with CSV export. Exact schemas and file lifecycle belong to T14. This does not adopt S05's future-proxy recommendation export.

T09 initially needed a clearer explanation; no acceptance was inferred from that response. After clarification, the user chose generated pairing setup **plus support for TLS certificates they provide**. Intended workflow: generate protocol-appropriate server identity/client authorization material, transfer pairing information through an already trusted channel such as the user's SSH session, and import it into the client dashboard. Retain explicit server identity verification; never silently accept an unknown identity or bypass verification. TLS-capable owned-server endpoints also accept user-supplied certificates with an explicit trust/identity configuration. A server TLS certificate does not itself authorize a client. Protocols such as WireGuard retain their native credentials; encrypted recursive-resolver endpoint authentication remains independent.

Detailed pairing format, client enrollment, secret storage, certificate loading/renewal, and credential rotation/revocation remain open for the contract/integration review. Pairing transfer must not export the server's private identity key; reports exclude credentials/private keys. Go's [TLS verification documentation](https://pkg.go.dev/crypto/tls#Config) provides certificate-verification mechanisms, not a complete pairing workflow or runtime validation.

T12 requires broad mainstream Linux server coverage, with CentOS, Ubuntu, Fedora, and Arch explicitly named by the user as examples, not an exhaustive list. Do not narrow the product to the development host's distribution. Exact versions and compatibility requirements remain unresolved; intended broad coverage is not a claim of support for every historical release. This discussion does not adopt optional S01-S06 or authorize implementation.

## Remaining technical choices

T05 and T10-T11 await decisions. Also unresolved: Linux version/support matrix, dashboard layout/chart library and optional asset build tooling, exact configuration/API/wire schemas, detailed pairing/credential handling, integration/privileges, numerical defaults, DNS records/framing/polling/limits, QoS presets/observation capabilities, and ranking/confidence thresholds. Consult current primary documentation where capabilities matter. Incremental implementation must preserve the current confirmed coverage.

## Questions for the next discussion

- **T13:** Which bundled chart library and dashboard layout suit the measurement contract? Plain HTML/CSS/JavaScript is confirmed; optional asset build tooling remains open.
- **T05:** Which embedded protocol integrations and exact compatible dependency versions should we adopt? Investigate socket/source-port/outer-priority control before final selection. DNS library selection is still open.
- **T09/T14:** Define the selected generated pairing transfer/import and user-provided TLS certificate flows: enrollment, identity verification, secret storage, rotation/revocation, and certificate lifecycle. The high-level approach is confirmed.
- **T10:** Sequential comparisons or controlled concurrent groups? Decide how to avoid unintended contention and port collisions. Automatic screening remains optional S01.
- **T11:** Which Linux/Windows APIs and observation methods can support priority on every adapter's actual outer socket? Record capability limits; do not infer support from a library hook alone.

## Additional open choices

T12 and T13 have partial answers recorded below; other answers and numerical defaults remain open. These expand the unresolved areas above without creating new product features.

| ID | Question | State |
| --- | --- | --- |
| T12 | Windows 11 client only; Linux server only; 64-bit ARM64/x86-64 on both; broad mainstream Linux coverage including CentOS/Ubuntu/Fedora/Arch. Which exact version/support matrix? | Roles/architectures/distribution scope confirmed by user, 2026-10-11; version limits open |
| T13 | Plain HTML/CSS/JavaScript confirmed. Which dashboard layout, chart library, and optional asset build tooling? | Stack confirmed by user, 2026-10-11; remaining details open |
| T14 | What versioned API, configuration validation, probe wire format, and stored-result schemas should be defined? | Open; related to T06-T08 |
| T15 | What initial payload sizes, bidirectional rates, duration, warmup, deadline, and grace period should be configurable defaults? | Open |
| T16 | Which DNS records, encodings, framing, polling, fragmentation, and transport-specific limits should the tunnel use? | Open; related to T05 |
| T17 | What priority presets and sender/receiver observation capabilities should be exposed on each platform? | Open; related to T11 |
| T18 | What FPS decision criteria, confidence rules, and minimum evidence should qualify a recommendation? | Open; S06 remains optional |

## Maintaining choices

- Record future technical questions here with stable identifiers, meaningful options, a recommendation when justified, and unresolved dependencies.
- Keep states explicit: open, recommended, confirmed, deferred, or superseded. Include the user's answer and decision date when confirmed; do not infer acceptance from a recommendation or silence.
- When a choice changes, retain a short reference to the previous choice and log the reason in `WORK_LOG.md`. A change to technology does not automatically change product scope.
- Update affected design text when a confirmed choice changes product behavior. Keep detailed alternatives and research here rather than repeating them in `DESIGN.md` or `PROPOSALS.md`.
- Reference relevant F/S feature identifiers when useful. Record actual implementation/validation in `WORK_LOG.md`; choosing a dependency does not mark its feature implemented.
