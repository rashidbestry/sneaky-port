# sneaky-port

A planned client/server measurement tool for finding the best connection settings for FPS online games on your network.

It will compare real TCP, UDP, TLS, QUIC, WireGuard, Hysteria2, and DNS data paths against one Linux server, using Windows 11 clients. Both operating systems target 64-bit ARM64 and x86-64, with broad Linux server coverage including CentOS, Ubuntu, Fedora, Arch, and other major distributions; exact version limits remain open. Selected ports and independent packet priority markings in both directions are part of the comparison. Protocol sessions are handled by the tool itself.

Connection quality means timely delivery, low p95/p99 latency, low jitter, and few spikes or stalls under sustained game-like traffic. Preferential treatment of DNS is a hypothesis to measure, not an assumed advantage.

The protocol architecture must be extensible: later releases should add probe adapters through shared measurement, configuration, dashboard, and reporting contracts.

**Current status: design only. No application has been implemented.** A future gaming proxy, traffic shaping, and automatic route switching are separate from this repository's scope.

## Project documents

- [AGENTS.md](AGENTS.md): working instructions and current authorization.
- [DESIGN.md](DESIGN.md): confirmed requirements, protocol/DNS coverage, ports, packet priority, measurement rules, and validation.
- [CHOICES.md](CHOICES.md): technical questions, options, recommendations, and confirmed decisions.
- [ROADMAP.md](ROADMAP.md): the first few actions, adjusted as we learn.
- [WORK_LOG.md](WORK_LOG.md): backward tracking of completed changes, current implementation, and validation.
- [PROPOSALS.md](PROPOSALS.md): forward tracking of future features, optional ideas, and their decisions/status.
