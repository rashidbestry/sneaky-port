# sneaky-port

A client/server measurement tool under development for comparing FPS connection quality on your network.

It will compare real TCP, UDP, TLS, QUIC, WireGuard, Hysteria2, and DNS data paths against one Linux server, using Windows 11 clients. Both operating systems target 64-bit ARM64 and x86-64, with broad Linux server coverage including CentOS, Ubuntu, Fedora, Arch, and other major distributions; exact version limits remain open. Selected ports and independent packet priority markings in both directions are part of the comparison. Protocol sessions are handled by the tool itself.

Connection quality means timely delivery, low p95/p99 latency, low jitter, and few spikes or stalls under sustained game-like traffic. Preferential treatment of DNS is a hypothesis to measure, not an assumed advantage.

The protocol architecture must be extensible: later releases should add probe adapters through shared measurement, configuration, dashboard, and reporting contracts.

**Current status: first-slice foundations implemented; complete measurement workflow pending.** A future gaming proxy, traffic shaping, and automatic route switching are separate from this repository's scope.

Implementation was authorized on 2026-10-11. Current code provides strict profiles/plan hashes, rational scheduling, authenticated UDP/TCP framing and socket adapters, local ledgers, clock-envelope/SSH identity primitives and local timer preflight. Embedded SSH orchestration, the network run engine, persisted reports/API/dashboard and full acceptance evidence remain pending. [Code flow](FLOWCHART.md), [contract](DESIGN.md#first-slice-implementation-contract), [actual checks](WORK_LOG.md#2026-10-11-first-implementation-foundations).

## Run the foundation tools

Requires Go 1.27 or later. Commands below run only the currently implemented operations:

```text
go test ./...
go run ./cmd/sneaky-port adapters
go run ./cmd/sneaky-port validate -profile YOUR_PROFILE.yaml
go run ./cmd/sneaky-port preflight -duration-ns YOUR_DURATION -interval-ns YOUR_INTERVAL
go run ./cmd/sneaky-port keygen -output NEW_PRIVATE_KEY_FILE
```

All workload/timeout/budget settings are explicit; there is no automatically selected FPS profile. Preflight is local timer evidence. Key generation writes a new restricted file and prints only public enrollment material. `client`/`server` commands are not available yet. Dependencies and [license notices](THIRD_PARTY_NOTICES.md) are pinned; native ARM64/live-network support remains unvalidated.

## Project documents

- [AGENTS.md](AGENTS.md): working instructions and current authorization.
- [DESIGN.md](DESIGN.md): confirmed requirements, protocol/DNS coverage, ports, packet priority, measurement rules, and validation.
- [CHOICES.md](CHOICES.md): technical questions, options, recommendations, and confirmed decisions.
- [ROADMAP.md](ROADMAP.md): the first few actions, adjusted as we learn.
- [WORK_LOG.md](WORK_LOG.md): backward tracking of completed changes, current implementation, and validation.
- [PROPOSALS.md](PROPOSALS.md): forward tracking of future features, optional ideas, and their decisions/status.
- [FLOWCHART.md](FLOWCHART.md): implemented function paths and remaining connections, linked to source.
