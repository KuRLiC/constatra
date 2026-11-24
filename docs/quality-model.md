# Measurement and quality model

Constàtra turns several low-level observations into one large, stable color. This document makes that conversion explicit so defaults can be reviewed, tested, and changed deliberately.

## Probe sample

Once per second, the engine attempts a TCP connection to every configured target in parallel. With three default targets, each tick produces three results.

A target result contains:

- `success`: whether the TCP handshake completed before its deadline;
- `latencyMs`: time from the start of the connection attempt until success or failure;
- `error`: a compact reason when the attempt failed.

The tick-level latency is the median of successful target latencies. The tick-level probe loss is:

```text
failed target attempts / all target attempts × 100
```

If every target fails, latency is left unavailable rather than recording the timeout duration as a successful latency measurement.

## Rolling windows

The same samples feed two views:

| View | Default window | Purpose |
|---|---:|---|
| RIGHT NOW | 3 seconds | Fast feedback while changing hotspot position |
| STABILITY | 30 seconds | Confidence that a position has remained usable |

History retained for the chart does not change these calculations. A 15-minute chart still displays the current 3- and 30-second classifications.

## Window statistics

For every window, Constàtra calculates:

### Probe loss

The percentage of individual target attempts that failed. This is deliberately penalized strongly: intermittent failure can disrupt interactive work even when successful attempts are fast.

### Availability

A tick is available when at least one target succeeds. Availability is:

```text
available ticks / all ticks in the window × 100
```

Probe loss and availability describe different conditions. If one of three targets is consistently unreachable, probe loss is 33.3% while tick availability can remain 100%. If all targets disappear for one tick, both metrics degrade.

### Median latency

The 50th percentile of all successful target latencies. It describes the typical connection handshake.

### P90 latency

The 90th percentile of all successful target latencies. The main color uses p90 rather than only the mean or median so recurring slow observations remain visible.

### Jitter

For each target independently, Constàtra takes the absolute latency difference between consecutive successful probes. Jitter is the mean of those temporal differences across the window. A failed probe breaks that target's sequence until its next success.

Keeping target sequences separate is important: three perfectly stable endpoints at 30, 80, and 130 ms produce zero jitter instead of being mistaken for one highly variable endpoint. This remains a practical connection-stability indicator, not RTP interarrival jitter as defined for a media stream.

## Score

The score starts at 100 and subtracts four penalties:

```text
score = clamp(
    100
    - loss penalty
    - latency penalty
    - jitter penalty
    - availability penalty,
    0, 100)
```

### Loss penalty

```text
min(probe loss % × 1.5, 65)
```

Examples:

| Probe loss | Penalty |
|---:|---:|
| 1% | 1.5 |
| 5% | 7.5 |
| 20% | 30 |
| 33.3% | 50 |
| 50% or more | 65 maximum |

### P90 latency penalty

The latency penalty is continuous within these ranges:

| P90 latency | Penalty range |
|---:|---:|
| 0–80 ms | 0–5 |
| 80–160 ms | 5–17 |
| 160–350 ms | 17–37 |
| 350–700 ms | 37–55 |
| Over 700 ms | 60 |

### Jitter penalty

- 0–20 ms maps linearly to a penalty of 0–3.
- Above 20 ms, the penalty grows more quickly.
- The penalty is capped at 25.

### Availability penalty

```text
min((100 - availability %) × 0.4, 30)
```

## Category overrides and thresholds

After scoring, explicit guardrails prevent one severe metric from being hidden by good values elsewhere:

| Level | Score/guardrail conditions |
|---|---|
| **CRITICAL** | Score below 35, loss at least 50%, availability below 70%, or p90 at least 700 ms |
| **POOR** | Score below 60, loss at least 20%, p90 at least 350 ms, or jitter at least 150 ms |
| **GOOD** | Score below 80, loss at least 5%, p90 at least 160 ms, or jitter at least 60 ms |
| **EXCELLENT** | None of the above |

Conditions are evaluated from most severe to least severe.

## Offline behavior

If a window contains no successful probes, its level is `OFFLINE`.

RIGHT NOW also uses a time-based fast-offline rule. When the most recent successful tick is at least two seconds old, it shows `OFFLINE` even when that older success is still inside the 3-second window. With no successful observation at all, it reports `OFFLINE` as soon as the first failed measurement completes. The time-based rule keeps the intended behavior if a user changes the sampling interval or a measurement is delayed.

STABILITY does not use that shortcut. It continues to describe the rolling 30-second record, so an immediate outage can coexist with a previously good sustained indicator. This is intentional:

- RIGHT NOW answers “is it working now?”
- STABILITY answers “how trustworthy has this position been?”

## Warm-up behavior

Before the first completed sample, both views are `CHECKING`. RIGHT NOW can classify after its first sample. STABILITY deliberately remains `CHECKING` for its first 10 observed seconds; partial statistics are still shown during this warm-up. After that it classifies the observations collected so far, while confidence continues to grow until the full 30-second window has filled.

No synthetic old samples are added. A newly started 30-second window therefore summarizes only the observations actually collected.

## Why multiple targets

One endpoint can be slow, filtered, or unavailable independently of the user's connection. Concurrent targets provide useful redundancy and expose partial reachability. Using direct IP addresses also avoids making DNS resolution a prerequisite for each timing observation.

Multiple targets are not a perfect voting system: a provider can route them differently, and some networks selectively block public services. The technical target display exists so a consistently failing endpoint is visible rather than silently hidden.

## Interpretation cautions

TCP handshake time includes more than radio latency. It can reflect:

- Wi-Fi contention between computer and hotspot;
- cellular radio conditions and retransmission;
- carrier scheduling, congestion, and routing;
- Internet path changes;
- destination-side behavior;
- local firewall or VPN processing.

This is useful for the primary decision—whether the end-to-end connection is usable—but does not identify the failing network layer on its own.

The defaults are practical starting points for interactive remote work, not service-level guarantees. Change them only with tests that preserve the intended visual behavior and document the new rationale.
