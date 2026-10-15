# sneaky-port

Measure UDP/TCP connection quality between Windows 11 and one Linux server, both x86-64. Reports cover application receipt deadlines, RTT/variation, spikes, stalls, ports and independent directional DSCP.

<a id="download-the-preview"></a>
<a id="start-the-standalone-linux-server"></a>

## Run

Download the [preview](https://github.com/rashidbestry/sneaky-port/releases), extract it and run:

```powershell
.\sneaky-port-dashboard.exe
```

Open the printed `api_url`, enter the Linux server's normal SSH details (usually port 22), connect and run a test. The dashboard uploads its matching temporary Linux helper. No Go installation or startup arguments are needed.

For the standalone Linux service, run `./sneaky-port-server-linux-amd64`. It uses app SSH **22022**, generated private state and client-supplied test settings; [key pairing](docs/USAGE.md#pair-endpoints-and-check-readiness) is required. Normal SSH and the managed helper are separate.

<a id="status"></a>
<a id="project-documents"></a>

## Status and documentation

UDP/TCP is implemented; full release acceptance is incomplete. Latest Linux startup checks are user-deferred; production one-way bounds and broader native/network qualification remain open. TLS, QUIC, WireGuard, Hysteria2 and DNS are future work.

[Usage](docs/USAGE.md) · [Design/decisions](docs/DESIGN.md) · [Roadmap/features](docs/ROADMAP.md) · [Validation](docs/WORK_LOG.md) · [History](docs/ARCHIVE.md) · [Notices](docs/THIRD_PARTY_NOTICES.md) · [Repository rules](docs/AGENTS.md)
