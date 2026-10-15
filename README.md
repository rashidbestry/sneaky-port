# sneaky-port

Measure FPS connection quality between a Windows 11 x86-64 client and a Linux x86-64 server. Compare UDP/TCP ports and independent directional packet priority using receipt deadlines, RTT, jitter, spikes and stalls. Runs save replayable reports with JSON, JSONL and CSV exports.

## Download the preview

Get the [Windows or Linux x86-64 preview](https://github.com/rashidbestry/sneaky-port/releases). Go is not required for packaged binaries. Use matching client/server builds; current source may be newer than published previews.

From the extracted Windows archive:

```powershell
.\sneaky-port-dashboard.exe client-api -data-dir .\client-data -profile .\examples\readiness-udp.yaml
```

Open the printed `api_url`. Enter your Linux server's normal SSH details (usually port **22**) and select **Connect**; the dashboard uploads its matching temporary helper. Choose UDP/TCP, review the test settings, then **Run test**. The selected data port must be reachable.

## Start the standalone Linux server

```sh
chmod +x ./sneaky-port-server-linux-amd64
./sneaky-port-server-linux-amd64
```

Current source starts app SSH on **22022** with built-in limits and automatically generated private state. No server policy file is required; authenticated clients supply test settings. Follow [key pairing](docs/USAGE.md#pair-endpoints-and-check-readiness) for standalone use. This listener is separate from normal SSH and the dashboard's helper workflow.

## Status

UDP/TCP is implemented; full release acceptance remains incomplete. Production one-way clock bounds and broader platform/network qualification are pending. Linux runtime checks for the latest startup change are deferred. TLS, QUIC, WireGuard, Hysteria2 and DNS remain future work. See the [remaining checks](docs/ROADMAP.md#task-35-current-acceptance-review) and [validation record](docs/WORK_LOG.md).

## Project documents

- [Operator guide](docs/USAGE.md): source builds, pairing, CLI, API, reports and measurement limits.
- [Design](docs/DESIGN.md), [technical choices](docs/CHOICES.md) and [code flow](docs/FLOWCHART.md).
- [Roadmap](docs/ROADMAP.md), [feature proposals](docs/PROPOSALS.md) and [history](docs/ARCHIVE.md).
- [Dependency notices](docs/THIRD_PARTY_NOTICES.md) and [repository instructions](docs/AGENTS.md).
