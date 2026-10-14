# sneaky-port @@VERSION@@ (@@PLATFORM@@ x86-64)

UDP/TCP connection measurement preview with a browser dashboard and matching Linux helper. Go is not required to run the binaries. Full platform and live-network acceptance remains incomplete.

## Start the dashboard

Extract the archive into a writable folder and run this command from that folder:

```text
@@START_COMMAND@@
```

Open the exact `api_url` printed in the terminal. Enter your Linux server's IP, SSH port, username and password, then Connect. The dashboard uploads, verifies and starts its matching Linux x86-64 helper. The SSH account must allow commands and forwarding, with `mktemp`, `head`, `sha256sum`, `chmod`, `rm` and `rmdir` available. The selected UDP/TCP data port must be reachable between the endpoints.

The first SSH fingerprint is trusted after successful authentication; a changed saved fingerprint requires explicit replacement. Passwords are cleared and are not saved. Keep your application store and reports private.

Choose UDP/TCP and edit the server, workload, duration, deadline, rates, payload and ports before Run test. The included profile contains editable example values. Stop cancels the owned test; Ctrl+C shuts down the application.

Receipt outcomes are application observations. The chart shows client attempt-to-receipt RTT in milliseconds over elapsed seconds. Live results are provisional; terminal replay supplies final or partial results. Production clock calibration and on-wire priority claims remain unverified.

## CLI and evidence

The dashboard executable also provides the CLI commands. The Linux archive includes `sneaky-port-server-linux-amd64` for the standalone server/CLI workflow. Replace the executable name in the [operator documentation](https://github.com/@@REPOSITORY@@/blob/@@REVISION@@/README.md) with the binary from this archive. Standalone pairing/server examples use `examples/readiness-policy.json`.

`BUILD_INFO.json` records actual source revision and dirty state, toolchain, compiled build settings, binary sizes and SHA-256 hashes. [Remaining acceptance gates](https://github.com/@@REPOSITORY@@/blob/@@REVISION@@/ROADMAP.md#task-35-current-acceptance-review) distinguish package checks from platform qualification and live-network acceptance. TLS, QUIC, WireGuard, Hysteria2 and DNS are future work.

Dependency notices are in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) and `licenses/`. No application license is inferred from dependency licenses. Reference tools are not bundled.
