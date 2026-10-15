# sneaky-port @@VERSION@@ (@@PLATFORM@@ x86-64)

UDP/TCP measurement preview with a browser dashboard and matching Linux helper. Go is not needed to run it. Full platform/live-network acceptance is incomplete.

<a id="start-the-dashboard"></a>

## Start

Extract into a writable folder and run:

```text
@@START_COMMAND@@
```

Open the printed `api_url`. Windows needs no arguments and stores private state in `%APPDATA%\sneaky-port\client`. Optional `-data-dir FOLDER` selects an existing store; `-profile FILE` supplies a complete YAML/JSON override.

Enter the Linux server's normal SSH IP/port/username/password, connect, review settings and run. The account needs commands/forwarding plus `mktemp`, `head`, `sha256sum`, `chmod`, `rm` and `rmdir`; the chosen UDP/TCP data port must be reachable. First fingerprints are trusted after authentication; changed pins require replacement. Passwords are transient. Stop cancels a test; Ctrl+C closes the app and owned helper.

The standalone `./sneaky-port-server-linux-amd64` starts app SSH on **22022**, with generated private state and built-in limits. It requires [trusted key pairing](https://github.com/@@REPOSITORY@@/blob/@@REVISION@@/docs/USAGE.md#pair-endpoints-and-check-readiness); normal SSH 22 is separate.

<a id="cli-and-evidence"></a>

## Results and notices

Receipts measure application outcomes; RTT is shown in ms over elapsed seconds. Live results are provisional; terminal replay is authoritative. Production one-way bounds/outer-priority claims remain unverified. TLS, QUIC, WireGuard, Hysteria2 and DNS are future work.

`BUILD_INFO.json` records revision/dirty state, toolchain, binary settings/hashes and packaging checks. See [remaining acceptance](https://github.com/@@REPOSITORY@@/blob/@@REVISION@@/docs/ROADMAP.md#task-35-current-acceptance-review), [dependency notices](docs/THIRD_PARTY_NOTICES.md) and `licenses/`. Keep private state/reports private; reference tools are not bundled and dependency licenses do not imply an application license.
