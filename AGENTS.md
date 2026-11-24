# AGENTS.md

These instructions apply to the entire repository. They are written for AI coding agents and should be treated as the project-specific operating contract.

## Project mission

Constàtra (**CONnection STAbility TRAcker**) is a small, cross-platform, real-time network quality monitor. Its primary use case is practical: a user leaves a laptop visible, moves a phone acting as a hotspot around a room, and reads two large color-first indicators from a distance.

The product must answer two questions immediately:

1. Is the connection good **right now**?
2. Has it **stayed good** long enough to trust this position?

The application is a single Go executable with an embedded web dashboard. It must work on Linux, Windows, and Linux under WSL with the dashboard viewed in a Windows browser.

## Non-negotiable product behavior

- All software-facing text, CLI help, UI labels, log output, documentation, commit messages, workflow names, and release notes must be in English.
- The UI is color-first and must remain readable from across a room.
- Keep the page background dark, neutral, low-contrast, and non-invasive. Bright color belongs primarily to the two large quality indicators.
- Do not rely on color alone: every state also needs a visible English label and meaningful technical values.
- Preserve the two decision horizons:
  - **RIGHT NOW**: default 3-second rolling window.
  - **STABILITY**: default 30-second rolling window with an initial warm-up period.
- Preserve the state vocabulary and visual ordering:
  - `EXCELLENT`: green.
  - `GOOD`: yellow.
  - `POOR`: orange.
  - `CRITICAL`: red.
  - `OFFLINE`: dark gray.
  - `CHECKING`: neutral gray.
- Default settings must work without tuning.
- The normal update cadence is one second. User-visible disconnection feedback should arrive within approximately one to two seconds.
- Missing probes must materially degrade quality.
- The dashboard must show technical evidence without competing with the dominant color indicators.
- The application must print the dashboard URL, try to open it automatically, and continue running if browser launch fails.
- Port `0` is the default: the operating system atomically selects a free port. Do not return to a fixed default such as 8080.
- Bind only to loopback addresses. Do not add LAN/public listening without an explicit security design and owner approval.
- Both the dashboard Stop button and `Ctrl-C` must shut down the process cleanly.
- Keep runtime state and history in memory unless persistence is explicitly requested.
- Do not add telemetry, accounts, remote JavaScript, remote fonts, analytics, or a cloud backend.

## Measurement terminology

The current engine performs concurrent TCP handshakes rather than privileged ICMP echo requests.

Default targets:

- `1.1.1.1:443`
- `8.8.8.8:443`
- `9.9.9.9:443`

Use the term **probe loss**, not packet loss. A failed TCP probe can result from actual packet loss, retransmission, filtering, endpoint failure, routing failure, or timeout. Do not claim that Constàtra directly measures raw IP packet loss.

The quality model combines:

- successful and total probes;
- probe loss;
- tick availability;
- median latency;
- p90 latency;
- temporal jitter per target;
- a 0–100 score;
- explicit severity guardrails.

Temporal jitter is the mean absolute latency change between consecutive successful probes to the same target. Do not compute jitter across different endpoints; stable endpoints with different baseline latencies must not appear jittery.

RIGHT NOW uses time-based fast-offline behavior. STABILITY deliberately retains recent history. See `docs/quality-model.md` before changing thresholds, formulas, warm-up, or category overrides. Any scoring change requires table-driven tests and matching documentation.

## Repository layout

- `cmd/constatra/`: CLI entry point, option validation, listener creation, browser launch, signals, and process lifecycle.
- `internal/probe/`: concurrent portable TCP probing and per-tick summaries.
- `internal/quality/`: pure rolling-window statistics, scoring, labels, warm-up, and offline logic.
- `internal/app/`: history ownership, snapshots, console output, HTTP API, SSE fan-out, shutdown, and security headers.
- `internal/webui/`: embedded HTML, CSS, and dependency-free JavaScript dashboard.
- `scripts/`: local toolchain, versioning, build, run, and release automation.
- `.github/workflows/release.yml`: tag and GitHub Release automation driven by committed `VERSION` changes.
- `docs/`: architecture, scoring, scripts, troubleshooting, and version/release contracts.
- `VERSION`: tracked `major.minor.patch.build` application version.

## Runtime architecture and API

The application is a single process:

```text
measurement loop -> rolling in-memory history -> quality model
                                              -> console
                                              -> JSON/SSE API -> embedded browser UI
```

Important local routes:

- `GET /`: embedded dashboard with the per-process shutdown token.
- `GET /app.css`: embedded styles.
- `GET /app.js`: embedded dashboard logic.
- `GET /api/state`: complete current snapshot and retained history.
- `GET /api/events`: bounded Server-Sent Events stream.
- `POST /api/shutdown`: token-protected graceful shutdown.

Preserve these server properties:

- slow browsers must never block measurements;
- subscriber channels remain bounded and publish non-blockingly;
- SSE handlers send heartbeats and exit on request or application cancellation;
- shared slices are copied before publication;
- shutdown replies are flushed before process cancellation;
- the shutdown token is random per process and compared safely;
- security headers and the restrictive Content Security Policy remain in place;
- do not configure a finite server-wide write timeout that kills long-lived SSE connections.

## Toolchain and first-time setup

The project pins Go `1.27.4` for its self-contained workflow. `go.mod` currently declares the minimum language/toolchain compatibility expected by the source.

Recommended first command on Linux or WSL:

```bash
./scripts/setup.sh
```

This downloads the official architecture-specific Go archive, verifies the pinned SHA-256 checksum, and installs it under ignored `.tools/`. It requires no `sudo` and does not modify global Go configuration.

Shared script configuration lives in `scripts/lib.sh`. After setup, use the repository-local toolchain in automation:

```bash
source scripts/lib.sh
"${CONSTATRA_GO_BIN}" version
```

Generated content belongs only in ignored locations:

- `.tools/`: Go SDK, module cache, build cache, and version lock.
- `build/`: user-facing local builds.
- `dist/`: optional packaging output.
- `/tmp/...`: disposable validation builds and downloaded release checks.

Do not commit generated binaries, caches, SDK files, or temporary output.

## Critical version side effect

The supported build commands increment `VERSION` exactly once per invocation:

```bash
./scripts/build.sh
./scripts/build.sh --all
./scripts/run.sh
make build
make build-all
```

`run.sh` rebuilds before execution, so it also increments the version.

This side effect is intentional, but AI agents must not increment `VERSION` merely to compile or test an unrelated change. For ordinary validation, use the local Go toolchain directly and write binaries to `/tmp`:

```bash
source scripts/lib.sh
"${CONSTATRA_GO_BIN}" test ./...

CONSTATRA_VALIDATION_VERSION="$(./scripts/version.sh current)"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "${CONSTATRA_GO_BIN}" build \
  -trimpath -ldflags "-s -w -X main.version=${CONSTATRA_VALIDATION_VERSION}" \
  -o /tmp/constatra-linux-amd64 ./cmd/constatra
```

Use `scripts/build.sh` only when an increment is intended. Never silently revert a user-created `VERSION` change. Inspect it and determine whether it is part of an intended build or release.

Version commands:

```bash
./scripts/version.sh current
./scripts/version.sh next
./scripts/version.sh increment
```

`increment` uses `.tools/version.lock` and an atomic file replacement. If a stale lock exists, verify that no build is running before removing only that lock directory.

## Development workflow

1. Read `README.md` and the relevant document under `docs/` before changing behavior.
2. Start with `git status --short --branch`. Preserve unrelated or user-authored changes.
3. Make the smallest coherent change that fully solves the request.
4. Keep the Go backend standard-library-only unless a dependency has a clear, documented benefit.
5. Keep the frontend dependency-free unless the owner explicitly approves a build pipeline.
6. Update tests and documentation in the same logical step as behavior changes.
7. Run proportional verification, including platform builds when lifecycle or release behavior changes.
8. Check `git diff --check` and review the staged diff before committing.
9. Commit each coherent step separately with a descriptive English subject and an explanatory body.
10. Push verified work to `main` when publication is part of the request. Do not rewrite published history unless explicitly requested.

The repository owner prefers exhaustive commit bodies that explain behavior, safety properties, tests, and documentation—not vague messages such as `fix` or `updates`.

## Test and validation matrix

### Go changes

```bash
source scripts/lib.sh
"${CONSTATRA_GO_ROOT}/bin/gofmt" -w <changed-go-files>
"${CONSTATRA_GO_BIN}" vet ./...
"${CONSTATRA_GO_BIN}" test ./...
"${CONSTATRA_GO_BIN}" test -race ./...
```

Required focused coverage includes:

- scoring threshold boundaries;
- loss degradation;
- time-based offline behavior;
- stability warm-up;
- temporal jitter separated by target;
- successful and failed local TCP probes;
- CLI default validation;
- state API startup behavior;
- shutdown token rejection/acceptance;
- SSE cleanup and application stop.

### Shell changes

```bash
bash -n scripts/*.sh
shellcheck scripts/*.sh
```

Keep scripts idempotent where documented, use `set -euo pipefail`, quote expansions, and never delete unexpected tool directories automatically.

### Frontend changes

```bash
node --check internal/webui/app.js
```

Also smoke-test the embedded page and API from a real built binary. Confirm that the dominant indicator colors remain legible on the neutral background at desktop and narrow widths.

### Workflow changes

- Parse `.github/workflows/release.yml` with a YAML parser.
- Prefer `actionlint` when available.
- Confirm referenced official action major versions are current before changing them.
- Validate the same build commands locally without incrementing `VERSION`.
- For a real workflow test, follow the publication verification procedure below and do not stop after a green workflow status.

### Cross-platform validation

Use direct Go commands for non-versioning validation:

```bash
source scripts/lib.sh
CONSTATRA_VALIDATION_VERSION="$(./scripts/version.sh current)"
mkdir -p /tmp/constatra-validation

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "${CONSTATRA_GO_BIN}" build \
  -trimpath -ldflags "-s -w -X main.version=${CONSTATRA_VALIDATION_VERSION}" \
  -o /tmp/constatra-validation/constatra-linux-amd64 ./cmd/constatra

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 "${CONSTATRA_GO_BIN}" build \
  -trimpath -ldflags "-s -w -X main.version=${CONSTATRA_VALIDATION_VERSION}" \
  -o /tmp/constatra-validation/constatra-windows-amd64.exe ./cmd/constatra
```

Verify file types and run the Linux binary with `--version`. When browser behavior changes, manually test WSL-to-Windows opening and the printed-URL fallback.

## Version and publication contract

`VERSION` uses:

```text
major.minor.patch.build
```

Normal project builds advance the final component. Major, minor, and patch changes require deliberate review.

Every committed `VERSION` change pushed to `main` triggers `.github/workflows/release.yml`. The workflow:

1. validates `VERSION`;
2. runs `go vet` and race-enabled tests;
3. builds standalone Linux AMD64 and Windows AMD64 binaries;
4. creates `SHA256SUMS`;
5. creates or validates annotated tag `v<VERSION>` at the exact triggering commit;
6. creates a GitHub Release, or safely replaces assets on a rerun.

The workflow has only `contents: write` permission. Keep permissions minimal.

A commit that does not change `VERSION` must not create a release.

### Standard release command

Preview:

```bash
./scripts/release.sh --dry-run
```

Publish:

```bash
./scripts/release.sh
```

The local release script validates the tree and remote, performs a versioned verification build, runs race tests, commits only `VERSION`, and pushes `main`. GitHub Actions owns tag and Release creation. Local GitHub CLI authentication is not required for publishing.

Do not manually create or move release tags during the normal flow. Manual tag work is a recovery operation and requires explicit owner approval.

### Release preconditions

- Current branch is `main`.
- `main` exactly matches `origin/main` before release preparation.
- The index is clean.
- No working-tree change exists except an optional pre-existing `VERSION` modification from a local build.
- The planned tag does not already exist locally or remotely.
- Tests and both cross-builds pass.

### Post-publication verification

Do not report a release complete until all of these pass:

```bash
gh run list --repo KuRLiC/constatra --workflow "Publish release" --limit 5
gh run watch <run-id> --repo KuRLiC/constatra --exit-status
gh release view "v$(./scripts/version.sh current)" --repo KuRLiC/constatra
```

Download the published assets into a new `/tmp` directory, run `sha256sum --check SHA256SUMS`, execute the Linux binary with `--version`, and inspect both binary formats. Fetch the generated tag locally and confirm its peeled commit equals `HEAD`.

Expected release assets:

- `constatra-linux-amd64`
- `constatra-windows-amd64.exe`
- `SHA256SUMS`

### Release failure handling

- Before the version commit: fix the problem and rerun; `VERSION` may remain incremented.
- Push failure: the version commit remains local and no tag/release is generated.
- Workflow failure before tag creation: fix the workflow or source and rerun the job; do not create a new version solely to retry.
- Workflow rerun after tag creation: the tag must point to the same commit; existing release assets are replaced idempotently.
- Never move an already published version tag to a different commit.

## Git and branch policy

- `main` is the canonical and release branch.
- `origin` is `KuRLiC/constatra` through the configured SSH alias.
- Use the configured identity `KuRLiC <kurlic@hotmail.com>` for project commits.
- Do not push a duplicate `master` branch.
- Do not force-push `main` unless the owner explicitly requests a history rewrite and the remote lease has been verified.
- Do not amend or squash already published commits without explicit approval.
- Never commit secrets, GitHub tokens, private network configuration, local toolchains, caches, or release downloads.

## Documentation responsibilities

Behavior and documentation must remain synchronized:

- `README.md`: user-facing overview, quick start, CLI, release entry points, and limitations.
- `docs/architecture.md`: packages, lifecycle, concurrency, API, security, and build design.
- `docs/quality-model.md`: formulas, thresholds, warm-up, offline semantics, and interpretation cautions.
- `docs/scripts.md`: setup/build/run behavior and generated files.
- `docs/versioning.md`: version mutation, CI tags, release automation, and recovery.
- `docs/troubleshooting.md`: user-observable failures and safe diagnostics.
- `AGENTS.md`: AI operating rules and project invariants.

When changing a flag, default, endpoint, route, score, asset name, toolchain version, script side effect, workflow trigger, or release step, search all documentation for the old contract and update every affected location.

## Security and privacy invariants

- Listener host validation accepts loopback IPs only.
- Random local port selection uses the actual listener bound to port `0`; never probe a free port and reopen it later.
- Browser launch is best-effort and must not stop measurement.
- WSL detection must occur before the generic Linux browser path.
- The Stop endpoint remains POST-only and token-protected.
- Keep CSP, `nosniff`, frame denial, and no-referrer headers.
- No persistent user network history by default.
- No automatic uploads of measurements or environment information.
- Release workflows must use the repository token with minimal permissions and official actions.

## Known limitations

- TCP handshake latency differs from ICMP ping latency.
- Probe loss is not raw packet loss.
- One selectively blocked endpoint can degrade the score.
- The application cannot read phone radio strength, band, tower, or modem state.
- It does not yet separate laptop-to-hotspot Wi-Fi quality from hotspot-to-cellular quality.
- It does not continuously measure throughput, DNS health, or loaded latency.
- History is in-memory only.

Do not silently describe a roadmap item as an implemented feature.

## Definition of done

A change is complete only when:

- requested behavior is implemented;
- relevant product and security invariants are preserved;
- tests cover new logic and regressions;
- formatting, static checks, race tests, and applicable cross-builds pass;
- UI or lifecycle changes receive a smoke test;
- documentation matches the implementation;
- `git diff --check` is clean;
- the change is committed with an explanatory message;
- requested publication is pushed and verified;
- release requests include workflow, tag, asset, checksum, and binary-version verification;
- the final working tree is clean except for explicitly acknowledged user changes.
