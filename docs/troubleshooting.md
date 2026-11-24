# Troubleshooting

## The browser did not open

Constàtra continues running even when automatic browser launch fails. Use the complete URL printed near startup:

```text
Dashboard: http://127.0.0.1:49152
```

Copy or click it. When using `--no-browser`, this is expected behavior.

On a minimal Linux installation, automatic launch generally needs `xdg-open` and a graphical session. This is not required when opening the URL manually.

## WSL is running, but Windows cannot reach the page

Try these checks in order:

1. Confirm the Constàtra process is still running in WSL.
2. Use the exact URL and port printed by that process.
3. Use `127.0.0.1`, not a guessed WSL address.
4. Check whether custom `.wslconfig`, VPN, proxy, or firewall settings have disabled or interfered with localhost forwarding.
5. Start with a fixed port temporarily, such as `--port 8765`, to simplify firewall diagnosis.
6. If Windows browser launch itself failed, run `cmd.exe /c start http://127.0.0.1:<port>` from WSL to verify that Windows integration is available.

Restarting WSL can restore a broken localhost-forwarding state, but do not treat that as the first diagnostic step if other WSL services are active.

## The selected port is already occupied

Do not specify a port unless another tool requires it. The default `--port 0` asks the operating system for a free port and avoids collisions, including common use of port 8080.

If a requested fixed port is unavailable, Constàtra exits with an error instead of silently using a different port. Remove `--port` or choose another value.

## Everything shows OFFLINE, but browsing works

Likely causes include outbound TCP filtering, a VPN policy, or all configured targets being inaccessible from the current network.

Check the per-target indicators. Then try a known reachable address that accepts TCP connections on the selected probe port:

```bash
constatra --targets 1.1.1.1,8.8.8.8 --probe-port 443
```

Hostnames are supported, but their resolution adds DNS behavior to the measurement. Direct IP addresses keep each timing observation independent of DNS. Do not use an arbitrary server that you do not operate or know permits this traffic.

Captive portals may block or redirect Internet access until their sign-in page has been completed.

## One target is always red

The network or destination may selectively block that endpoint. A consistently failed target increases probe loss even when the other targets work, because partial reachability is meaningful diagnostic information.

To distinguish a local policy from an endpoint problem:

1. compare behavior with and without a VPN;
2. try the same target from another network;
3. temporarily supply a different independent target;
4. return to multiple targets after the test.

Avoid configuring several addresses owned by the same service; diversity is what makes the default set useful.

## Colors change too often

Brief transitions near a threshold are real measurements, but there are several ways to interpret them:

- Look at **RIGHT NOW** while moving the hotspot.
- Wait for **STABILITY** to fill before deciding that a location is reliable.
- Inspect loss and p90 latency rather than focusing on median latency alone.
- Use the five-minute chart to determine whether the change is a single spike or a repeating pattern.

Changing the sample interval or rolling windows also changes the meaning of the defaults. Prefer the standard one-second, 3-second, and 30-second combination unless diagnosing a specific behavior.

## Latency differs from the `ping` command

This is expected. Constàtra measures a TCP handshake to port 443, while `ping` measures an ICMP echo exchange. Firewalls, routing, destination processing, and protocol handling can differ. Constàtra's measurement was chosen because it works without special privileges and exercises application-relevant reachability.

Compare trends rather than expecting identical millisecond values.

## Probe loss is not the same as packet loss

Probe loss is the proportion of TCP connection attempts that did not complete before the deadline. Actual IP packet loss can contribute, but so can filtering, endpoint failure, routing failure, or a timeout caused by extreme delay.

The label is intentionally specific. For raw packet diagnostics, use an ICMP or network-capture tool alongside Constàtra.

## The dashboard says GOOD while the connection feels bad

Constàtra currently emphasizes reachability, connection setup latency, variation, and loss. An application can still perform poorly because of:

- insufficient download or upload throughput;
- bufferbloat only visible under load;
- DNS problems after a connection is established;
- application-server problems;
- VPN or proxy behavior;
- long-lived connection issues not represented by new handshakes.

Periodic HTTPS and throughput checks are roadmap items. Until then, use an application-specific test alongside Constàtra when capacity is the suspected bottleneck.

## The dashboard says POOR, but work seems fine

Open the technical target list. A single blocked target can create 33.3% loss with three targets while availability remains 100%. That partial reachability may or may not affect the applications you use.

Also check whether a VPN makes the selected public destinations unusually slow. If the condition is stable and understood, select appropriate independent targets rather than weakening loss penalties globally.

## The Stop button does not work

The shutdown request requires the token from the page served by the current process. A stale browser tab from a previous Constàtra run cannot stop a newer process, even if a port was reused.

Refresh the current URL. If the server is unresponsive, return to the terminal and press `Ctrl-C`. Graceful signal handling is the authoritative fallback.

## Tests do not run because `go` is missing

Go is needed to build and test, but not to run a published executable. Install Go 1.22 or newer in the build environment, then verify:

```bash
go version
go test ./...
```

No Node.js installation is required.

Alternatively, use the repository-local setup without installing Go globally:

```bash
./scripts/setup.sh
./scripts/build.sh
```

If setup reports an invalid existing tool directory, move `.tools/go-<version>-linux-<architecture>` out of the repository and run setup again. Setup deliberately does not delete an unexpected directory automatically.

## Setup cannot download or verify Go

The bootstrap requires `curl`, `tar`, `sha256sum`, and `mktemp`, which are normally present in Linux and WSL environments. It stops before extraction when any command is missing, the download fails, or the pinned checksum does not match.

Corporate proxies and TLS inspection can prevent access to `https://go.dev/dl/`. Configure the environment's normal HTTPS proxy settings or install a compatible Go toolchain manually; do not disable checksum verification.

## Cross-compiled Windows binary does not start

Confirm the output architecture matches Windows:

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -o build/constatra-windows-amd64.exe ./cmd/constatra
```

Common mistakes are trying to execute the Windows binary inside Linux, transferring it without the `.exe` suffix, or building ARM64 for an AMD64-only environment. Run the `.exe` from Windows or invoke it through Windows integration from WSL.

## Reporting a reproducible problem

Include:

- operating system and whether WSL is involved;
- Constàtra version;
- exact command-line options, with private internal addresses redacted if necessary;
- console output around startup or failure;
- which targets succeed and fail;
- whether a VPN, proxy, captive portal, or firewall is active;
- whether the problem also occurs with default options.

Do not publish unrelated network configuration, credentials, private URLs, or full firewall exports.
