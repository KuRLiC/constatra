# Constàtra

**CONnection STAbility TRAcker** is a small, real-time connection monitor designed to answer two questions at a glance:

1. Is this location good **right now**?
2. Has the connection **stayed good** long enough to trust it?

The original use case is deliberately practical: leave a laptop where it is visible, move a phone acting as a hotspot around a room, and use large color-coded indicators to find and validate the best position. Constàtra also works as a general foreground monitor during calls, remote sessions, uploads, and other latency-sensitive work.

Constàtra runs a local web dashboard, opens it in the default browser, and updates it once per second. The browser UI and console both report immediate and sustained quality. No application runtime is required after compilation: releases are single Go executables for Linux and Windows.

## What you see

The dashboard is intentionally color-first and readable from across a room:

| State | Color | Meaning |
|---|---|---|
| **EXCELLENT** | Green | Low latency, low variation, and effectively no missed probes |
| **GOOD** | Yellow | Usable connection with a measurable warning signal |
| **POOR** | Orange | Noticeable delay, variation, or probe loss |
| **CRITICAL** | Red | Severe degradation; real work is likely to be interrupted |
| **OFFLINE** | Dark gray | No successful probe, or the last success is at least 2 seconds old |
| **CHECKING** | Neutral gray | Not enough data has been collected yet |

Two large indicators serve different decisions:

- **RIGHT NOW** summarizes the last 3 seconds and reacts quickly while the hotspot is being moved.
- **STABILITY** summarizes the last 30 seconds and answers whether the new position has remained reliable.

Below them, a 2-, 5-, or 15-minute chart shows latency and missed probes. Technical details include p90 round-trip time, jitter, probe loss, availability, score, per-target results, update interval, and process uptime.

## How measurement works

Constàtra makes concurrent TCP connection attempts to several independent, highly available endpoints. A successful TCP handshake supplies a latency observation; a timeout or failed connection counts as a missed probe and degrades the score.

The default endpoints are:

- `1.1.1.1:443` — Cloudflare
- `8.8.8.8:443` — Google
- `9.9.9.9:443` — Quad9

TCP was chosen instead of requiring ICMP echo requests because it:

- works without administrator/root privileges or Linux capabilities;
- does not depend on an operating-system `ping` command or localized command output;
- cross-compiles cleanly into self-contained Linux and Windows executables;
- measures the ability to establish the kind of connection real applications need.

This metric is called **probe loss**, not raw IP packet loss. A dropped packet, retransmission, filtering rule, unreachable endpoint, or timeout can all affect a TCP probe. Multiple targets reduce the chance of mistaking one remote service problem for a local connection problem. See [Measurement and quality model](docs/quality-model.md) for the complete algorithm and thresholds.

## Quick start

### Recommended: setup, build, and run scripts

On Linux or WSL, the shortest path is:

```bash
./scripts/run.sh
```

On the first run, this downloads the pinned Go toolchain into the ignored local `.tools/` directory, verifies its official SHA-256 checksum, builds Constàtra, and starts it. Later runs reuse the toolchain and Go build cache. Nothing is installed globally and `sudo` is not required.

The individual steps are also available:

```bash
# Download and verify the local build toolchain
./scripts/setup.sh

# Build the native Linux/WSL executable
./scripts/build.sh

# Build Linux AMD64 and Windows AMD64 executables with one shared version
./scripts/build.sh --all

# Rebuild if necessary and run; options are passed to Constàtra
./scripts/run.sh --no-browser
```

Generated executables are written under `build/`. The downloaded toolchain and caches remain under `.tools/`; both directories are ignored by Git. See [Setup, build, and run scripts](docs/scripts.md) for the complete contract and recovery instructions.

Every project build increments the final component of the tracked `VERSION` value. Both binaries produced by one `--all` invocation receive the same version, which is visible in the dashboard, startup console, and `--version` output.

### Manual build with an existing Go installation

Building requires Go 1.22 or newer. Running the resulting executable does not require Go or another runtime.

```bash
mkdir -p build
go build -o build/constatra ./cmd/constatra
./build/constatra
```

The repository also provides the equivalent build target:

```bash
make build
```

Constàtra selects an available local port, prints the complete URL, tries to open it in the default browser, and starts measuring immediately. If browser launch fails, copy or click the URL shown in the console.

Stop it with the **Stop** button in the dashboard or `Ctrl-C` in the terminal. Both paths perform a graceful HTTP server shutdown.

### Cross-compile Windows and Linux binaries from Linux

Constàtra uses the Go standard library and does not require CGO. Both target executables can be produced from Linux:

```bash
mkdir -p build

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -o build/constatra-linux-amd64 ./cmd/constatra

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -o build/constatra-windows-amd64.exe ./cmd/constatra
```

For ARM64 targets, replace `GOARCH=amd64` with `GOARCH=arm64`.

The checked-in build target creates both AMD64 executables and advances the tracked version:

```bash
make build-all
```

The self-contained script equivalent is:

```bash
./scripts/build.sh --all
```

### Publish a GitHub Release

Preview the next release without changing files or GitHub:

```bash
./scripts/release.sh --dry-run
```

Publish after reviewing the preflight:

```bash
./scripts/release.sh
```

The release workflow requires GitHub CLI authentication. It verifies `main` is synchronized and free of unrelated changes, increments the version, builds and tests Linux/Windows artifacts, writes SHA-256 checksums, commits `VERSION`, creates an annotated tag, pushes atomically, and creates a GitHub Release. See [Versioning and GitHub releases](docs/versioning.md).

### Run the Windows build

From PowerShell or Command Prompt:

```powershell
.\constatra-windows-amd64.exe
```

Windows Defender Firewall normally permits outbound TCP connections without a prompt. Constàtra listens only on the loopback interface by default, so it does not expose the dashboard to the LAN.

## WSL and a Windows browser

Running the Linux binary inside WSL while viewing the dashboard in a Windows browser is a supported workflow:

```bash
./build/constatra
```

The server binds to `127.0.0.1` and uses a random available port. Constàtra detects WSL and asks Windows to open the URL using `cmd.exe`. With normal WSL localhost forwarding, the same `http://127.0.0.1:<port>` URL is reachable from Windows.

If the page does not open automatically:

1. copy the URL printed by Constàtra;
2. paste it into the Windows browser;
3. confirm that WSL localhost forwarding has not been disabled by custom networking configuration.

Using a random port avoids collisions with development servers commonly occupying port 8080.

## Command-line options

Defaults are meant to work without tuning. Options are available for diagnostics and unusual network environments:

```text
constatra [options]

  --host string                 Loopback listen IP (default "127.0.0.1")
  --port int                    Local HTTP port; 0 chooses a free port (default 0)
  --no-browser                  Do not open the dashboard automatically
  --targets string              Comma-separated probe targets
  --probe-port int              TCP port used by probes (default 443)
  --interval duration           Time between samples (default 1s)
  --timeout duration            Timeout for each target (default 800ms)
  --immediate-window duration   RIGHT NOW window (default 3s)
  --stability-window duration   STABILITY window (default 30s)
  --history duration            In-memory history retention (default 15m)
  --version                     Print version information and exit
```

Examples:

```bash
# Print the URL but leave browser choice to the user
constatra --no-browser

# Use internal or provider-specific endpoints
constatra --targets 10.0.0.1,203.0.113.10 --probe-port 443

# Request a fixed port when integrating with another local tool
constatra --port 8765
```

Durations use Go syntax such as `750ms`, `3s`, and `2m`. Avoid changing the default windows before observing the standard behavior; their thresholds are designed as a coherent set.

## Console output

The terminal remains useful when the browser is on another display. A live line reports both decisions and the most important supporting values, for example:

```text
NOW EXCELLENT   58 ms  loss   0.0% | STABILITY GOOD       92 ms  loss   3.3%  available 100.0%
```

At startup the console always prints the dashboard URL and selected probe targets. Startup, browser-launch, and shutdown errors are printed as normal log lines.

## Security and privacy

- The HTTP server binds to `127.0.0.1` by default. It is not accessible from other devices on the network.
- The dashboard Stop action requires a per-process random token embedded in the locally served page.
- Probe samples and chart history remain in memory and disappear when Constàtra exits.
- Constàtra has no account, telemetry, analytics, database, or cloud backend.
- Normal operation sends only TCP connection handshakes to the configured targets and serves the dashboard locally.
- Endpoint IPs, timing, and error strings are visible in the local browser and console.

For safety, `--host` accepts loopback IP addresses only. Constàtra deliberately does not provide a public/LAN listening mode.

## Development

Run the complete test suite:

```bash
go test ./...
```

Additional useful checks:

```bash
go vet ./...
go test -race ./...
```

The dashboard has no Node.js toolchain or third-party frontend dependencies. HTML, CSS, and JavaScript are embedded into the executable at build time.

Project documentation:

- [Architecture](docs/architecture.md)
- [Measurement and quality model](docs/quality-model.md)
- [Setup, build, and run scripts](docs/scripts.md)
- [Versioning and GitHub releases](docs/versioning.md)
- [Troubleshooting](docs/troubleshooting.md)

## Current limitations

- TCP handshake latency is not identical to ICMP ping latency and does not expose raw packet-level loss.
- An endpoint-specific outage or policy block can look like probe loss; multiple independent targets mitigate but cannot eliminate this.
- Constàtra measures end-to-end reachability from the computer. It cannot read the phone's cellular signal strength, radio band, tower, or modem statistics.
- The initial implementation does not separate laptop-to-hotspot Wi-Fi quality from hotspot-to-cellular quality by probing the local gateway.
- The score uses practical fixed defaults, not an adaptive baseline for a particular provider or location.
- History is intentionally in-memory only; there is no persistence or export yet.

## Roadmap

Potential follow-up work, in approximate value order:

1. Optional local-gateway probe to distinguish Wi-Fi trouble from cellular uplink trouble.
2. Lightweight periodic HTTPS and throughput probes for application-layer validation and capacity context.
3. Best-recent-position marker and audible accessibility cues.
4. CSV/JSON export and opt-in session persistence.
5. Adaptive baseline mode alongside the predictable fixed-threshold mode.
6. Signed release archives and automated multi-platform builds.

The primary design rule remains unchanged: the color must be understandable immediately, while the supporting measurements must be available when the user wants to investigate why.
