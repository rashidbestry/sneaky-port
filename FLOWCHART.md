# Code flow

Updated: 2026-10-11. This describes **current foundations, authenticated readiness and finite UDP/TCP CLI/accounting**. Bounded persistence, replay/export, authenticated reconciliation, SSH clock captures, one-way estimates and protected configuration/run/report API are implemented. Production clock bounds, dashboard/live views, comparisons and full acceptance remain pending. Each function has a purpose comment; `go test ./internal/quality -v` audits it. [Roadmap task index](ROADMAP.md#first-slice-task-index) tracks remaining work.

## Start reading here

```mermaid
flowchart TD
    A[main: install cancellation signal handler] --> B[run: select explicit command]
    B --> C[adapters: enumerate compile-time registry]
    B --> D[validate: open explicit profile file]
    B --> E[preflight: local monotonic timer samples]
    B --> F[keygen: new local Ed25519 identity]
    B --> P[coordination: explicit store and trust commands]
    B --> Q[server: Linux embedded SSH endpoint]
    B --> R[client-ready: verify and stop without measuring]
    B --> V[client-run: finite authenticated UDP/TCP measurement]
    B --> W[report-replay: validate local and transferred evidence]
    B --> X[report-export: exact JSON, JSONL or CSV]
    B --> Y[client-api: protected local run controls]
    B --> G[Unknown or unfinished command: error]
    C --> H[writeJSON: public results]
    D --> I[Decode: strict shared profile validation]
    I --> J[Factory.Validate: carrier limits and options]
    J --> K[Hash: canonical frozen-plan digest]
    K --> H
    E --> H
    F --> H
```

Entry points: [main.go](cmd/sneaky-port/main.go) (`main`, `run`, `writeJSON`), [coordination.go](cmd/sneaky-port/coordination.go) (`coordination`, `keyPassphrase`, `loadSigner`). `server` is Linux-only; `client-ready` uses direct owned UDP/TCP plus SSH control. `client-run` performs finite measurement; dashboard mode remains unsupported. Validation/preflight open no network sockets.

## Protected local run API

```mermaid
flowchart TD
    A[clientAPI: explicit store and launch profile; existing key or explicit later generation] --> B[ListenWithStore: validate saved ID and literal loopback; snapshot/hash fallback]
    B --> C[Generate process token; return private local bootstrap JSON]
    C --> D[Serve owned IPv4 or IPv6 HTTP listener]
    D --> E{Loopback peer and exact Host/Origin; valid bearer and request ID?}
    E -->|no| X[Structured rejection; no lifecycle mutation]
    E -->|yes| F{Versioned route and bounded request?}
    F -->|no| X
    F -->|adapters| G[Page compile-time descriptors]
    F -->|status| H[Small immutable lifecycle snapshot]
    F -->|saved configuration/reports| Q[One bounded saved operation; shared validators]
    F -->|start/cancel| I[Strict JSON with explicit identity and profile revision inputs]
    I --> J{Already accepted request ID?}
    J -->|same command| K[Original acknowledgment; no new operation]
    J -->|different command| X
    J -->|new| L{Current profile revision, history capacity and lifecycle admission?}
    L -->|no| X
    L -->|start| M[Select saved ID or launch fallback; match revision; private plan; execute control.Run]
    L -->|cancel| N[Cancel only the named active context]
    M --> O[Actual final identity/completeness; release reservation]
    N --> O
    D --> P[Process shutdown: stop admissions and HTTP; cancel and bounded join]
```

Sources: [launcher](cmd/sneaky-port/api.go) (`clientAPI`), [API](internal/localapi/server.go) (`Listen`, `ListenWithStore`, `handle`, `adapters`, `mutate`, `status`, `execute`, `Serve`, `Close`), [saved selection](internal/localapi/saved.go) (`selectedProfile`), [finite runner](internal/control/run.go) (`Run`), [security/lifecycle fixtures](internal/localapi/server_test.go), [real HTTP/SSH/UDP CLI fixture](cmd/sneaky-port/finite_test.go). HTTP contexts never schedule/cancel owned traffic. API handles differ from eventual measurement IDs; unknown readiness/completeness remain null until runner return. Tokens stay out of URLs/HTTP output. Active state has no fabricated phase/live samples. [Saved operations](#saved-configuration-and-report-api) are implemented; dashboard/live views and comparison remain pending. The [operator contract](README.md#protected-local-run-api) records exact bounds.

## Saved configuration and report API

```mermaid
flowchart TD
    A[Protected saved route; safe IDs; one disk/replay admission] --> B{Operation}
    B -->|catalog/read| C[Bounded sorted ID page; validated profile/public pairing/enrollment]
    B -->|profile validation| D[Shared profile.Decode; normalized fields/hash; no write]
    B -->|profile save| D
    D -->|explicit PUT| E[Store lock; expected canonical revision; private synced atomic replacement]
    B -->|pairing import| F[Bundle.Validate; explicit trust replacement; owned store write]
    B -->|client key generation| G[InitClient if absent; restrictive managed key; public enrollment only]
    B -->|report read/export| H[SavedReport; bounded ReplayReport; reconstruct raw accounting]
    B -->|report import| I[Fixed filename/base64 bytes; private exclusive staging]
    I --> J[ReplayReport; checksums/schema/cutoffs; validate every supplied file]
    J -->|valid new ID| K[Publish new directory; preserve source and existing reports]
    J -->|invalid| L[Reject; remove only owned staging]
    H --> M[Structured replay or capped JSON/JSONL/CSV attachment]
    E --> N[Future start selects saved revision; active snapshot stays private]
    A --> O[Shutdown closes admission and joins storage plus finite runs]
```

Sources: [routes](internal/localapi/saved.go) (`saved`, `catalog`, `savedBody`, `profileWrite`, `pairingImport`, `keyCreate`, `reportImport`, `reportExport`, `diskReply`), [store](internal/control/saved.go) (`ownedDir`, `SavedIDs`, `Profile`, `SaveProfile`, `SavedReport`, `ImportReport`), [trust/key store](internal/control/store.go) (`ImportPairing`, `Pairing`, `InitClient`), [replay](internal/control/replay.go) (`ReplayReport`), [exports](internal/control/export.go) (`ExportReport`), [store fixtures](internal/control/saved_test.go), [HTTP fixtures](internal/localapi/saved_test.go), [real HTTP/SSH report fixtures](cmd/sneaky-port/finite_test.go). Saved bodies cap at 1 MiB including base64 overhead; catalog inspection at 256 entries; replay reads at 8 MiB; export at 16 MiB. Busy saved requests reject with 503. Offline integrity cannot authenticate imported evidence or verify clock bounds; cached reconciliation is excluded. No import resumes traffic or remotely enrolls a client. Remaining UI/comparison paths are tasks 19-21.

## Persistent trust and endpoint readiness

```mermaid
flowchart TD
    A[OpenStore: restrict real owned directories] --> B{Explicit local operation}
    B --> C[InitClient or InitServer: local key generation or reference]
    C --> D[Export public enrollment or server bundle only]
    D --> E[Trusted manual transfer]
    E --> F[Enroll or ImportPairing: validate public identity]
    F --> G[Exclusive writer lock and restricted atomic replacement]
    B --> H[LoadKey: encrypted reference prompts only in memory]
    H --> I[Serve: bounded application-owned SSH endpoint]
    I --> J[CheckClient: enabled enrolled key and exact client ID]
    J --> K[control: single application channel]
    K --> L[Hello: contract and adapter capabilities]
    L --> M[Prepare: validate exact plan hash and server ceilings]
    M --> N{Server already reserved?}
    N -->|yes| X[Explicit busy error]
    N -->|no| O[Prepare owned exact data port]
    O --> P[Fresh session, directional MAC keys and challenge over SSH]
    P --> Q[acceptBind: authenticate candidate before any reply]
    Q -->|invalid| R[Reject candidate; preserve listener within finite budget]
    R --> Q
    Q -->|valid| S[Lock tuple and return authenticated equal-size bind ack]
    S --> T[markPlan: validate each role's requested DSCP]
    T --> U[Confirm plan hash and effective bindings and markings]
    U --> V[Ready: measurement_started false]
    V --> W[Heartbeat lease or explicit stop]
    I --> Y[watchAuthorization: current allowlist and expiry poll]
    W --> Z[Close owned data and control; release reservation]
    Y -->|revoked or expired or invalid store| Z
    W -->|control EOF or lease expiry or cancellation| Z
```

Sources: [store.go](internal/control/store.go) (`OpenStore`, `InitClient`, `InitServer`, `Enroll`, `ImportPairing`), [session.go](internal/control/session.go) (`NewServer`, `Serve`, `connection`, `control`, `Ready`, `acceptBind`, `checkBind`, `watchAuthorization`). Host pinning precedes session secrets. Shell/exec/forwarding and premature/extra channels are refused; one calibration channel is admitted during a finite run and one report channel after stop. Invalid UDP first tuples cannot elicit a reply or pin the listener; partial TCP frames cannot authenticate. Fresh identities/challenges/keys never enter public results.

Both roles retain actual local/remote bindings; NAT observations need not equal the client's local tuple. Marking API success leaves packet-observed values unknown. Readiness holds only a lease, counts client application-control bytes through stop, and performs no scheduler/ledger/calibration run. Readiness-only cleanup produces no measurement samples. [Tests](internal/control/session_test.go), [store tests](internal/control/store_test.go), [setup commands](README.md#pair-endpoints-and-check-readiness).

## Finite run lifecycle

```mermaid
flowchart TD
    A[Run: freeze explicit plan and reserve client store] --> B[clientSession: shared SSH trust, data bind and readiness]
    B --> C[Start: exact hash and run/session identities]
    C --> D[Server captures local anchor; client captures its own on acknowledgment]
    D --> E[NewFinite: phase writer hook and independent shared workers]
    E --> F[Writer selects current physical phase and applies owned-socket DSCP]
    F --> G[Retain API evidence; timestamp, encode and write frame]
    E --> H[Independent control heartbeats and server lifecycle status]
    E --> S[startClock: independent bounded calibration channel]
    S --> T[Persist captures; stop and join before raw finalization]
    E --> I[Finite half-open grace cutoff]
    H -->|failure, lease expiry or cancellation| J[StopGeneration fixes truncated generation boundary]
    J --> K[Drain at most grace and shutdown; preserve original identities]
    K --> L[Interrupt owned I/O and join worker accounting]
    I --> L
    L --> M[Server retains socket until stop or control cleanup]
    M --> N[Client stop receives server lifecycle status]
    N --> O[Preserve endpoint-only local snapshot]
    O --> R[Separate authenticated report channel and reconciliation]
    R --> P[Close owned transport and release reservations]
```

Sources: [run.go](internal/control/run.go) (`Run`, `executeRun`, `durationSum`), [session.go](internal/control/session.go) (`clientSession`, `control`), [finite.go](internal/engine/finite.go) (`NewFinite`, `phaseMarker`, `finish`, `Cancel`, `Wait`), [worker](internal/worker/endpoint.go) (`StopGeneration`, `write`, `receive`), [socket](internal/adapter/adapter.go) (`Interrupt`, `Mark`, `Close`). [Control fixtures](internal/control/run_test.go), [phase/cutoff fixtures](internal/engine/finite_test.go), [Linux marking fixtures](internal/control/run_linux_test.go), [CLI fixture](cmd/sneaky-port/finite_test.go).

Anchors/epochs are local to each run, with no shared-clock claim. Marking follows the current physical phase for queued data and receipts; original identities still determine measurement attribution. Quiet phases update on the next outgoing frame, and grace retains the last setting. Frames spanning boundaries and API success do not prove packet markings. A stopped generation cutoff is separate from observation/drain; late timer callbacks or read errors at/after the elapsed cutoff cannot relabel a full window as truncated. Early failures remain partial and server run heartbeats expose them. A server cutoff interrupts local I/O without early peer EOF; final socket cleanup belongs to control. The exclusive client lock is never stolen after a crash. The snapshot is also saved in task 12 local summaries; local replay/export and authenticated server reconciliation are implemented separately from endpoint-only snapshots. [Evidence](WORK_LOG.md#2026-10-11-finite-run-lifecycle).

## Profiles and planned slots

```mermaid
flowchart TD
    A[Decode: read at most 1 MiB] --> B[YAML node inspection]
    B --> C{Duplicate key, alias, tag, merge or extra document?}
    C -->|yes| X[Reject with reason]
    C -->|no| D[requiredFields: explicit inputs]
    D --> E[Strict JSON decoding and Decimal parsing]
    E --> F[Plan.Validate: identity, timing and resource bounds]
    F --> G[Pattern.Count: exact predicted slot demand]
    G --> H{Budgets and carrier constraints feasible?}
    H -->|no| X
    H -->|yes| I[Canonical: integer and string plan domain]
    I --> J[SHA-256 plan hash]
    G --> K[Pattern.Offset: absolute rational slot time]
    K --> L[Wait: cancelable monotonic anchor]
    L --> M[Pattern.Due: all due slots, latest admissible slot]
    M --> N[Earlier due slots skipped; no catch-up burst]
```

Sources: [profile.go](internal/profile/profile.go) (`Decode`, `requiredFields`, `yamlValue`, `Plan.Validate`), [canonical.go](internal/profile/canonical.go) (`Canonical`), [schedule.go](internal/schedule/schedule.go) (`Count`, `Offset`, `Due`, `Wait`), [preflight.go](internal/measure/preflight.go) (`Preflight`). The UDP/TCP workers/accounting below connect these patterns to I/O and the ledger across frozen windows. Half-open phases exclude the end, and quiet/burst gaps schedule no data.

## Bounded writer queues

```mermaid
flowchart TD
    A[New: validate explicit queue bounds] --> B[One owned FIFO worker]
    C[Submit: validate data or empty receipt] --> D{Total caps and data reserve allow admission?}
    D -->|no data capacity| E[skipped_local with message or byte reason]
    D -->|no receipt capacity| F[failed_local with message or byte reason]
    D -->|yes| G[Copy content and charge waiting plus active budgets]
    G --> H[Pop oldest work without freeing budget]
    B --> H
    H --> I[Write callback with cancellation and per-frame timeout]
    I -->|complete| J[Release budget and publish one result]
    J --> H
    I -->|error or timeout| K[Stop admission and cancel queued work without attempts]
    L[Parent cancellation or Close] --> K
    K --> M[Cancel active callback; retain returned byte evidence]
    M --> N[Release remaining budget and exit worker]
    N --> O[Wait joins within caller shutdown budget]
```

Sources: [queue.go](internal/writer/queue.go) (`New`, `Submit`, `Snapshot`, `run`, `stopLocked`, `popLocked`, `finishLocked`, `Close`, `Wait`), [fixtures](internal/writer/queue_test.go). Invalid/stopped submissions fail without tickets. Receipts share FIFO order and total caps; data cannot consume reserved message or 124-byte receipt capacity. Active work stays charged until the callback returns. One buffered result per accepted frame avoids blocking on report consumers; consumers own later result retention.

Callbacks own attempt timestamps, encoding, transport byte boundaries and cancellation of their owned I/O. They must return on cancellation/timeouts; arbitrary blocking code cannot be forcibly interrupted, and a bounded `Wait` exposes that violation. The queue performs no retry/drain, schedule accounting, transport writes or network-loss inference by itself. Callback fixtures and the following UDP/TCP integration are separate evidence.

<a id="independent-udp-workers"></a>

## Independent UDP/TCP workers

```mermaid
flowchart TD
    A[NewUDP/NewTCP: validate frozen windows and run-wide identity budgets] --> B[Own authenticated carrier transport and directional keys]
    B --> C[Scheduler: Wait on absolute local anchor]
    B --> D[Reader: timestamp complete datagram or TCP frame before Decode]
    B --> E[One shared FIFO writer]
    C --> F[Due: account overdue range and latest eligible slot]
    F --> G[Emit skipped range without catch-up]
    F --> H[admit: expire deadlines then check outstanding and queue caps]
    H --> I[Emit admission and completion ticket]
    I --> E
    E --> J[write: attempt timestamp then encode and authenticate]
    J --> K[Record socket-call boundary and bounded write completion]
    D --> L[Check session, direction, epoch, schedule, content and echoed attempt]
    L -->|invalid or duplicate| M[Bounded event; no new receipt]
    L -->|eligible receipt| N[Record arrival; release only still-active admission]
    L -->|eligible data| O[Record arrival and construct original-identity receipt]
    O --> H
    G --> P[Nonblocking bounded event channel]
    I --> P
    K --> P
    M --> P
    N --> P
    O --> P
    P -->|capacity exhausted| Q[Retain terminal error and cancel]
    K -->|partial write, error or timeout| Q
    R[Parent cancellation or Close] --> S[shutdown: close queue and owned socket]
    Q --> S
    S --> T[Join scheduler, reader and writer; close events]
    T --> U[Wait returns joined outcome within caller bound]
```

Sources: [UDP](internal/worker/udp.go) (`NewUDP`), [TCP](internal/worker/tcp.go) (`NewTCP`), [endpoint.go](internal/worker/endpoint.go) (`newEndpoint`, `validateSchedule`, `schedule`, `admit`, `write`, `receive`, `check`, `emit`, `shutdown`, `Wait`), [UDP fixtures](internal/worker/udp_test.go), [limit/correlation fixtures](internal/worker/udp_limits_test.go). Both endpoints run separate local schedules; neither reads, receipts nor a stalled writer advance their clocks. Data and reverse receipts share one queue/key per physical sender. Duplicate data produces no second receipt; duplicate receipts release outstanding capacity only once. Event content is omitted after frozen-pattern validation, while byte counts and all local timestamp boundaries remain explicit. A skipped-range event retains the first identity/count for later ledger expansion. Admission records precede their attempt events, but arrival may precede write completion.

Standalone constructors require prepared/authenticated transports and constant caller-applied markings; finite lifecycle supplies the phase writer hook. [windows.go](internal/worker/windows.go) (`PlanWindows`, `prepareWindows`, `windowFor`) freezes phase patterns/content and continuous origin sequences. [expiry.go](internal/worker/expiry.go) (`expireLocked`, `recordStop`) frees outstanding capacity strictly after deadlines before admission, retains late receipt matching and freezes the first observed stop. Interrupted scheduling accounts only slots strictly before that stop; quiet intervals add none. `Scheduled` closes at full generation end or stop while reads continue until cancellation. Joined timeline shutdown emits an observation cutoff. TCP uses the same workers with its larger authenticated-frame limit and four-byte prefix-inclusive write/arrival counts. Complete reads racing stop remain out-of-window evidence without fresh receipts. Adapter reads timestamp only complete frames; partial prefix/body/EOF/cancel evidence is a `read_error`, never an arrival. Failed/partial writes close the affected segment without reconnect. [TCP worker fixtures](internal/worker/tcp_test.go) cover actual stream blockage, prefix/body splitting and interrupted framing. Task 12 now persists local evidence; the finite lifecycle above connects start/marking/grace/CLI. [Timeline fixtures](internal/worker/windows_test.go), [evidence](WORK_LOG.md#2026-10-11-udp-phase-accounting).

<a id="udp-phase-accounting"></a>

## UDP/TCP phase accounting

```mermaid
flowchart TD
    A[NewCollector: freeze validated plan and warmup/main windows] --> B[engine.NewUDP/NewTCP: verify plan digest and prepared transport]
    B --> C[Independent worker schedules, reads and FIFO writes]
    C --> D[Collector.Apply: byte-charge every internal observation]
    D --> E[Expand reached/skipped ranges using original rational offsets]
    D --> F[Correlate admissions, attempts, writes and first/duplicate receipts]
    D --> G[Retain unique peer arrivals separately from origin receipts]
    D -->|byte/identity cap or transition failure| H[Stop owned worker and retain partial evidence]
    C --> I[Finite lifecycle or standalone caller stops observation; workers join]
    H --> I
    I --> J[Finish: reconcile all retained admission tickets without waiting]
    J --> K[Record cancelled unattempted work and partial bytes]
    K --> L[Fix actual generation and strict observation cutoffs]
    L --> M[Separate warmup, main and each phase summaries]
    M --> N[Deadline misses, RTT and scheduled-origin tails, variation]
    M --> O[Group original-sequence spikes and overdue completion stalls]
    G --> P[Local receiver evidence; remote delivery stays unknown]
```

Sources: [engine/endpoint.go](internal/engine/endpoint.go) (`NewCollector`, `Apply`, `keep`, `Finish`, `newEndpoint`, `collect`, `Wait`), [UDP](internal/engine/udp.go) (`NewUDP`), [TCP](internal/engine/tcp.go) (`NewTCP`), [UDP fixtures](internal/engine/udp_test.go), [TCP fixtures](internal/engine/tcp_test.go), [analysis.go](internal/measure/analysis.go) (`Analyze`, `completionStalls`), [metric fixtures](internal/measure/analysis_test.go). Raw internal observations retain exact decimal quantities without keys/content/channels and stop visibly at `event_bytes`; task 12 persists these exact records as versioned event details; task 13 now replays these records through shared accounting. Actual queue completion tickets distinguish callback invocation from serialization attempts. Records at/after cutoff remain retained but cannot revise that summary; quantized equality stays excluded. Local `Complete` requires full generation/grace and intact evidence, and does not claim successful delivery or remote compatibility. Standalone constructors reject changing DSCP without a writer hook; finite lifecycle supplies it. Windows/Ubuntu/CentOS IPv4/IPv6 loopback and injected local failures are distinct from native ARM64, calibrated one-way, independent impairment and live acceptance.

## Real sockets and authenticated frames

The following primitive exchange is exercised by adapter loopback tests; finite control orchestration is described above.

```mermaid
sequenceDiagram
    participant T as Adapter loopback test
    participant R as Registry and factory
    participant S as Owned server socket
    participant C as Owned client socket
    T->>R: Get adapter and validate explicit plan
    T->>R: Prepare requested destination port
    R->>S: Bind UDP or listen TCP
    T->>R: Dial requested source port and destination
    R->>C: Own connected transport
    C->>S: Encode + Write authenticated data
    S->>S: Read whole frame + Decode and verify HMAC
    S->>C: Encode + Write receipt preserving data identity
    C->>C: Read whole frame + Decode and verify HMAC
    T->>C: Close owned client socket
    T->>S: Close owned session and listener
```

Sources: [adapter.go](internal/adapter/adapter.go) (`Register`, `Describe`, `Prepare`, `Dial`, `Accept`, `Reject`, `SetReadDeadline`, `Read`, `Write`, `Close`), [wire.go](internal/probe/wire.go) (`Encode`, `Decode`, `ReadTCP`, `WriteTCP`), [adapter tests](internal/adapter/adapter_test.go). UDP sends whole datagrams; TCP adds a four-byte length prefix. Decode checks the exact 92-byte header/124-byte authentication overhead, flags, content length and plan/adapter tag. Partial TCP frames never become arrivals. Production readiness uses the bind/challenge path above; UDP scheduled component traffic is described separately, and shared UDP/TCP scheduled component traffic is described above; the finite CLI orchestration above connects both carriers.

`socket.Mark` delegates to [Linux](internal/platform/marking_linux.go) or [Windows qWave](internal/platform/marking_windows.go). Requested, API-applied and independently observed marking are distinct; nonzero application failures return errors. No packet-capture/ECN acceptance gate has passed.

## Local accounting

```mermaid
flowchart TD
    A[Schedule: record reached slot and original identity] --> B{Admitted by shared accounting?}
    B -->|no| C[Skip: local reason, retained denominator]
    B -->|yes| D[Attempt: record origin submission boundary]
    D --> E[Written: complete or partial/failed bytes]
    E --> F{Authenticated matching receipt?}
    F -->|first| G[ReceiveReceipt: retain arrival]
    F -->|duplicate| H[Retain duplicate time within evidence cap]
    F -->|none| I[Remain unresolved at cutoff]
    C --> J[SummarizePhase: lifecycle and phase filter]
    G --> J
    H --> J
    I --> J
    J --> K[Deadline equality on time; cutoff equality excluded]
    K --> L[Exact completed-sample percentiles and gap-aware variation]
    J --> M{Complete compatible receiver evidence?}
    M -->|no| N[Receiver delivery remains unknown]
    M -->|yes| O[Reconcile unique receiver arrivals and missing application data]
    O --> P[TCP network packet loss still unknown]
```

Source: [ledger.go](internal/measure/ledger.go). `Admit` and `Unattempted` retain local queue outcomes; `ReceiptProcessing` preserves a valid peer-local interval, with impossible RTT correction unavailable. `Summarize` selects main; `SummarizePhase` separates warmup/phase, and `SummarizePhases` partitions one snapshot without repeated full-ledger scans. Attempt/write/receipt and observed local-resolution boundaries obey strict cutoff filtering. `ReceiverEvidence` is caller-supplied: the engine exposes endpoint-only unknown delivery; the authenticated control report path now validates compatibility and supplies receiver evidence for separate reconciled snapshots. The ledger has one owner; concurrent mutation is unsupported.

## Trust and clock evidence

```mermaid
flowchart LR
    A[GenerateKey] --> B[Create new restricted file before writing secrets]
    B --> C[Public key and fingerprint export]
    D[PinnedHost] --> E[Exact imported host identity or reject]
    F[Authorize] --> G[Enabled enrolled key before explicit expiry]
    H[Read control frame] --> I[Bounded JSON, unique keys, supported envelope]
    J[Build calibration model from synthetic captures] --> K[Intersect all affine offset/drift constraints]
    K --> L[Delay: check epochs, validity and uncertainty]
    L --> M[Unavailable, estimate or independently verified calibrated interval]
```

Sources: [identity.go](internal/control/identity.go), [key Windows](internal/control/key_windows.go), [key Linux](internal/control/key_linux.go), [control frames](internal/control/frame.go), [calibration.go](internal/calibration/calibration.go). `DecodeObject` rejects duplicate/unknown/missing fields, invalid UTF-8 and excessive nesting; the coordinator separately checks lifecycle identities. Calibration tests inject known captures; no live sampler supplies them yet. `Policy.Verified` is only enabled by synthetic fixtures; production cannot claim calibrated timing until the independent evidence gates pass.

## Local evidence persistence

```mermaid
flowchart TD
    A[Authenticated frozen readiness/settings] --> B[newEvidence: exclusive restricted run directory]
    B --> C[Reserve file publication space; sync incomplete manifest and raw file]
    C --> D[NewFiniteWithEvidence: start independent socket workers]
    D --> E[Collector.keep: exact record to bounded storage admission]
    E --> F[One FIFO disk worker; active-inclusive message/byte accounting]
    E -->|admission or file cap exhausted| G[Stop endpoint; retain partial evidence]
    F -->|disk failure even in quiet phase| G
    D --> H[Finite cutoff or cancellation; drain and join workers]
    G --> H
    H --> I[Finalize: bounded disk-writer join and raw flush]
    I --> J[Reserved final cutoff event and raw checksum]
    J --> K[Synced summary staging; atomic publication]
    K --> L[Atomic final manifest; endpoint-only complete or partial]
    I -->|join timeout| M[Initial manifest remains incomplete]
    K -->|publication failure| M
    L --> N[Local replay/export available; remote transfer pending]
```

Sources: [evidence.go](internal/control/evidence.go) (`newEvidence`, `Append`, `writeLoop`, `Finalize`, `publishEvidence`), [engine collector](internal/engine/endpoint.go) (`Persistence`, `keep`, `collect`), [finite lifecycle](internal/engine/finite.go) (`NewFiniteWithEvidence`, `finish`), [client coordinator](internal/control/run.go) (`executeRun`), [server coordinator](internal/control/session.go) (`control`), [storage fixtures](internal/control/evidence_test.go), [failure integration](internal/engine/persistence_test.go). Both stores save their own evidence before measurement and finalize independently of the control connection. Raw files omit authentication objects, content and keys; decimal offsets stay host-local/uncalibrated. Manifest/settings are frozen public values. Existing run directories are not reused; failed or interrupted publication stays incomplete/partial. Actual filesystem calls cannot be forcibly cancelled portably; a storage join timeout leaves the owned disk worker responsible until its call returns. Power-loss/nonlocal-filesystem durability remains pending; authenticated receiver reconciliation is implemented in the separate path below.

## Local report replay and export

```mermaid
flowchart TD
    A[report-replay or report-export; explicit directory/input cap] --> B[Read fixed regular local files; validate schema and frozen plan]
    B --> C[Check finalized raw hash/bytes; validate ordered event envelopes]
    C --> D[ReplayRecord: shared Collector accounting without I/O tickets]
    D --> E[Final cutoff or conservative prefix bound; incomplete stays partial]
    E --> F[ReplayFinish: same ledger summaries and timing analysis]
    F --> G[Compare saved summary; saved mismatch rejects report]
    G --> R[Optional verified transfer receipt: recheck server files and replay]
    R --> S[Derive compatible delivery; preserve unknown evidence]
    S --> H[JSON summary or typed JSONL bundle or exact long-form CSV]
    C -->|unfinalized or failed torn tail| I[Exclude torn line; retain partial status and tail bytes]
    I --> D
    G -->|failed storage mismatch| J[Expose partial raw derivation and mismatch]
    J --> H
```

Sources: [replay.go](internal/control/replay.go) (`ReplayReport`, `reportFile`, `decodeReportObject`), [engine replay](internal/engine/replay.go) (`ReplayRecord`, `ReplayFinish`), [shared collector](internal/engine/endpoint.go) (`apply`, `Finish`), [export.go](internal/control/export.go) (`ExportReport`, `exportCSVValue`, `reportUnit`), [CLI](cmd/sneaky-port/main.go) (`run`), [report fixtures](internal/control/replay_test.go), [completion fault](internal/engine/replay_limits_test.go). No sockets or fresh remote authentication occur here; validated saved transfer evidence can now support independently rederived receiver reconciliation. Saved completion records alone can resolve unsent tickets; unsaved completions leave unresolved local evidence. Checksums establish local consistency. Units/decimal text/nulls survive exports; missing calibration/receiver evidence remains unknown. Native ARM64, power-loss durability and live-network gates stay open.

## Remaining connections

Raw clock captures replay independently of measurement accounting and feed [one-way derivation](#uncertainty-gated-one-way-results). Protected saved configuration/report API is implemented; dashboard/SVG, matched comparisons and full acceptance remain pending.

## Authenticated server evidence transfer and reconciliation

```mermaid
flowchart TD
    A[Finite stop joins and finalizes server evidence] --> B[One report channel on existing authenticated SSH connection]
    B --> C[Pin three fixed regular files and hash bounded aggregate]
    C --> D{Remaining request and per-endpoint file budgets fit?}
    D -->|no| X[Incomplete transfer; retain local reports and unknown delivery]
    D -->|yes| E[Offer file sizes and hashes with run, session and frozen plan]
    E --> F[Stream contiguous numbered 32 KiB chunks]
    F --> G[Validate identity, offset and chunk hash; save exclusive files]
    G --> H[Acknowledge exact next offset]
    H --> F
    F --> I[Verify final content and file hashes; sync transferred files]
    I --> J[Replay both endpoint raw records independently]
    J --> K{Complete compatible plans, settings, epochs and observations?}
    K -->|no| X
    K -->|yes| L[Match receiver identity, original attempt, direction and epoch]
    L --> M[Shared ledger derives warmup, main and phase delivery]
    M --> N[Preserve unknown packet loss and ambiguous writes; separately derive one-way intervals]
    N --> O[Publish separate transfer receipt and derived reconciliation]
    O --> P[Offline replay rechecks transferred bytes and rederives exports]
```

Sources: [transfer.go](internal/control/transfer.go) (`routeReportChannels`, `serveReportChannel`, `openReportSnapshot`, `serveReport`, `receiveServerReport`, `receiveReport`, `verifyTransferredFiles`), [reconcile.go](internal/control/reconcile.go) (`reconcileReports`, `reconcileOrigin`, `replayTransferred`), [replay.go](internal/control/replay.go), [exports](internal/control/export.go), [shared ledger](internal/measure/ledger.go) (`SummarizeSnapshot`), [fixtures](internal/control/transfer_test.go). Numbered frames/acks and final hashes bind exact ordered bytes to the existing enrolled-client/pinned-host session. Endpoint raw summaries remain unchanged; each observation cutoff belongs to its own clock. Lost receipts do not imply missing data; stream delivery never establishes zero network packet loss.

The per-endpoint file cap (maximum 1 GiB), remaining request budget after lifecycle/calibration reservations and whole network-transfer timeout bound work. Offline input budgets include both remote hash/replay passes; they are not heap caps. Interrupted transfer stays unknown; saved receipts are local provenance, not transferable attestations. Recovery/dashboard and native/live/impairment acceptance remain pending.

## Live SSH clock sampling

```mermaid
flowchart TD
    A[Authenticated finite start publishes both data epochs] --> B[One in-run calibration channel on pinned SSH]
    B --> C[Absolute cadence; skip overdue slots without catch-up]
    C --> D[t1 before serialization; retain client send costs]
    D --> E[t2 at complete request bytes before decode]
    E --> F[Validate frozen identity, epoch, unique ordered request and budget]
    F --> G[t3 before response serialization and submission]
    G --> H[t4 at complete response bytes before decode]
    G --> I[Separate follow-up server serialization and send costs]
    H --> J[Correlate captures and follow-up; preserve invalid or incomplete samples]
    I --> J
    J --> K[Reserved calibration file bytes; shared bounded disk queue]
    K --> L[Bounded recent window, counters, age and expiry]
    L --> C
    K -->|admission fails| M[Stop sampler; retain reason in reserved final cutoff]
    A --> N[Data/control keep independent schedules and RTT]
    N --> O[Finite completion or cancellation joins sampler before finalization]
    O --> P[Stop server calibration before post-stop report channel]
    P --> Q[Replay exact raw captures; matched endpoint records feed one-way derivation]
```

Sources: [clock.go](internal/control/clock.go) (`clockGate`, `startClock`, `run`, `clockRead`, `clockWrite`, `appendClock`, `clockWindow`), [lifecycle](internal/control/run.go) (`executeRun`, `drainJoinBudget`), [routing](internal/control/transfer.go) (`routeReportChannels`), [persistence](internal/control/evidence.go) (`Finalize`), [replay](internal/control/replay.go), [fixtures](internal/control/clock_test.go). Captures use the actual endpoint data epochs; t3 submission includes separately measured serialization/send costs, and t4 excludes follow-up arrival. Recent invalid samples remain visible and can displace older valid samples. Readiness-only sessions never sample.

Requests, 4 KiB frames/raw captures, recent attempts and event/file/queue bytes have explicit bounds. Calibration rejection, invalidity, expiry or response timeout does not cancel working data/control. If the shared SSH transport blocks channel accept/close/write past shutdown, the owned connection closes as control failure. The sole uncancellable SSH channel-open wait belongs to that connection and ends at teardown. Captures provide constraints to the separate derivation below; production clock-quality validation remains task 28.

## Uncertainty-gated one-way results

```mermaid
flowchart TD
    A[Replay authenticated transferred endpoint evidence] --> B[Reuse complete directional reconciliation gates]
    A --> C[Read exact client raw clock captures]
    B --> D[Match original attempts and receiver arrivals at independent cutoffs]
    D --> E[Order by client send upstream or client arrival downstream]
    C --> F[Advance only captures finished by that client time]
    E --> F
    F --> G[Freeze each bounded recent window, including invalid evictions]
    G --> H{Enough valid unexpired samples with consistent epochs?}
    H -->|no| X[Unavailable timing with null interval and reason]
    H -->|yes| I[Intersect all constraints with explicit offset drift and error policy]
    I --> J{Feasible envelope and both captures inside validity?}
    J -->|no| X
    J -->|yes| K[Propagate rate-scaled timestamp and model errors to signed interval]
    K --> L{Wholly negative envelope?}
    L -->|yes| X
    L -->|no| M{Independent verified bounds and uncertainty within policy?}
    M -->|yes: synthetic justified fixtures only today| N[Calibrated interval]
    M -->|no: production today| O[Estimate; excluded from directional timeliness claims]
    X --> P[Count all states; retain model and result details within byte cap]
    N --> P
    O --> P
    P --> Q[CLI reconciliation and deterministic JSON JSONL CSV exports]
```

Sources: [one_way.go](internal/control/one_way.go) (`productionOneWay`, `deriveOneWay`, `oneWayQueries`, `retainOneWay`), [calibration.go](internal/calibration/calibration.go) (`Build`, `Delay`), [reconciliation](internal/control/reconcile.go) (`reconcileReports`), [replay](internal/control/replay.go), [exports](internal/control/export.go), [synthetic fixtures](internal/control/one_way_test.go), [real SSH/UDP/TCP fixtures](internal/control/clock_test.go). Revisions are reconstructed from raw captures, with hashes/sample IDs, signed bounds, age and clock-local units; future samples never upgrade earlier results. Missing receiver captures stay null. Exact populations include omitted details under min(event budget, 256 KiB). Endpoint-only RTT/delivery and raw files remain unchanged. Production verification cannot be enabled by profile/source text; independent clock-bound, native clock-change/impairment/live acceptance remains pending.
