# Architecture

Constàtra is a single-process local application with an embedded browser interface. It deliberately avoids external services, a database, a frontend build pipeline, and runtime dependencies.

## System view

```text
                         concurrent TCP handshakes
                    ┌──────────────┬──────────────┐
                    ▼              ▼              ▼
              1.1.1.1:443    8.8.8.8:443    9.9.9.9:443
                    ▲              ▲              ▲
                    └───────┬──────┴──────┬───────┘
                            │ probe results
┌───────────────────────────┴─────────────────────────────────┐
│                     Constàtra process                       │
│                                                             │
│  scheduler → probe engine → rolling history → quality model │
│                                         │                   │
│                                         ├─ console renderer │
│                                         └─ JSON/SSE API     │
│                                                │            │
│  embedded HTML/CSS/JS ← local HTTP server ─────┘            │
└────────────────────────────┬────────────────────────────────┘
                             │ 127.0.0.1:<random port>
                             ▼
                    Linux or Windows browser
```

The measurement loop and HTTP server share a cancellation context. `Ctrl-C`, normal process signals, and an authenticated dashboard shutdown request all converge on the same graceful shutdown path.

## Packages

### `cmd/constatra`

The executable entry point is responsible for:

- parsing and validating command-line options;
- selecting the local listen port;
- wiring the measurement engine, rolling windows, and web server;
- printing the dashboard URL and live console summary;
- launching the default browser on Linux, Windows, or Windows-from-WSL;
- handling signals and graceful shutdown.

Platform-specific behavior is confined to browser launch. The measurement engine itself uses portable Go networking APIs.

### `internal/probe`

The probe engine takes a list of IP addresses, TCP port, and timeout. At every tick it starts one connection attempt per target concurrently. Each result records:

- target;
- success or failure;
- elapsed milliseconds;
- a compact error description on failure.

After all attempts finish, the sample stores the median latency of successful targets and the percentage of missed target probes. Parallel execution prevents a slow target from serially delaying all other observations.

### `internal/quality`

The quality package is a pure rolling-window calculation. Given timestamped samples, a current time, and a window duration, it computes:

- successful and total probes;
- probe loss percentage;
- tick availability percentage;
- median and p90 latency;
- mean absolute latency change between consecutive successful probes to the same target, presented as jitter;
- a 0–100 score;
- a categorical level and display label.

The immediate 3-second calculation enables a time-based fast-offline rule: a last successful tick at least two seconds old is enough to report `OFFLINE`. This remains correct if the measurement interval changes or a tick is delayed. The sustained calculation retains the history of degradation rather than immediately discarding it.

Because this calculation does not perform I/O, threshold behavior is deterministic and straightforward to unit test.

### `internal/webui`

The web package embeds HTML, CSS, and plain JavaScript using Go's `embed` support. It has no Node.js dependency and no generated bundle.

The initial page receives a random shutdown token. JavaScript then:

1. loads the current snapshot and retained history;
2. opens a Server-Sent Events stream;
3. updates quality cards and technical values on each sample;
4. appends chart points and trims browser-side history;
5. redraws the selected 2-, 5-, or 15-minute canvas range;
6. sends the token when the user selects **Stop**.

Server-Sent Events fit this application because updates are one-way, ordered, small, and infrequent. They also reconnect automatically in browsers.

## Runtime sequence

```text
start
  │
  ├─ validate configuration
  ├─ listen on 127.0.0.1:0
  ├─ print resolved URL
  ├─ start HTTP server
  ├─ try to open browser
  ├─ take first measurement
  │
  └─ every 1 second
       ├─ connect to all targets concurrently
       ├─ append one sample to bounded history
       ├─ calculate RIGHT NOW (3 s)
       ├─ calculate STABILITY (30 s)
       ├─ render console summary
       └─ publish browser event

Ctrl-C / Stop / termination signal
  │
  ├─ cancel measurement work
  ├─ close SSE subscribers
  ├─ gracefully stop HTTP server
  └─ exit
```

The scheduler targets a one-second cadence. Configuration validation requires the measurement timeout to remain strictly below the interval so an unreachable network does not create an unbounded backlog. Measurements are serialized rather than overlapped. The implementation keeps at most the configured history duration in memory.

## Local HTTP contract

The browser integration uses a small same-origin API:

| Route | Purpose |
|---|---|
| `GET /` | Embedded dashboard with the per-process shutdown token |
| `GET /app.css` | Embedded visual styling |
| `GET /app.js` | Embedded dashboard logic |
| `GET /api/state` | Current configuration, snapshot, and retained history |
| `GET /api/events` | Live snapshot stream using Server-Sent Events |
| `POST /api/shutdown` | Token-protected graceful process shutdown |

API responses are intended for the bundled dashboard, but stable JSON field names also make them inspectable with browser developer tools or `curl`.

## Concurrency and ownership

The measurement loop is the only producer of samples. Shared state is protected while snapshots are copied for HTTP responses and subscribers. Slow or disconnected browsers must not block measurement: event delivery is bounded, and a subscriber can always recover the latest complete state through `/api/state`.

Shutdown is idempotent. Multiple signals, closing browser connections, or repeated Stop requests cannot close the same channel twice or terminate the server before the Stop response is delivered.

## Binding and browser launch

The default listener requests port `0`, allowing the operating system to atomically select an available port. This avoids a check-then-bind race and common collisions such as port 8080.

Browser launch strategies are:

- Windows: invoke the system URL handler;
- WSL: invoke Windows `cmd.exe` so the page appears in the Windows browser;
- Linux desktop: use `xdg-open` when available.

Browser launch is best-effort. Its failure never stops measurement; the resolved URL remains visible and usable in the terminal.

## Build properties

The code uses the Go standard library and embedded static assets. With `CGO_ENABLED=0`, the build output is a self-contained native executable suitable for cross-compilation. No JavaScript compilation is involved.

The scripts under `scripts/` provide a reproducible, non-root build environment. `setup.sh` downloads a pinned official Go archive into the Git-ignored `.tools/` directory and verifies the architecture-specific SHA-256 checksum before extraction. `build.sh` atomically advances the tracked four-component version and always uses that exact toolchain with `GOTOOLCHAIN=local`, while `run.sh` rebuilds incrementally before executing the native Linux binary. Toolchain and module caches also stay below `.tools/`.

Version metadata is read from the tracked `VERSION` file and supplied at link time. The underlying linker mechanism is equivalent to:

```bash
go build -ldflags "-X main.version=0.1.0.42" -o build/constatra ./cmd/constatra
```

## Security boundaries

The default design assumes the computer user is trusted and the network is not:

- listen only on IPv4 loopback;
- make no inbound public connection;
- require a random token for the state-changing shutdown endpoint;
- keep history in process memory;
- serve no user-provided file paths;
- use fixed embedded assets;
- avoid remote JavaScript, fonts, analytics, and content delivery networks.

Configuration validation rejects non-loopback listen addresses. The built-in server is intentionally not a remote monitoring service and does not replace authenticated monitoring infrastructure.
