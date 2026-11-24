# Setup, build, and run scripts

Constàtra provides a local, reproducible toolchain workflow for Linux and WSL. It is designed for the common case where Go is not installed globally and the user wants one command that starts the application.

## Quickest command

```bash
./scripts/run.sh
```

This command:

1. detects Linux AMD64 or ARM64;
2. installs the pinned Go toolchain locally when absent;
3. increments the tracked application build version;
4. builds the native Linux executable using the incremental cache;
5. replaces the script process with Constàtra;
6. forwards all command-line arguments to the application.

For example:

```bash
./scripts/run.sh --no-browser
./scripts/run.sh --port 8765
./scripts/run.sh --targets 1.1.1.1,8.8.8.8
```

Stop the running application with `Ctrl-C` or its dashboard **Stop** button.

## `setup.sh`

```bash
./scripts/setup.sh
```

Setup downloads Go 1.27.4 from `https://go.dev/dl/` into:

```text
.tools/go-1.27.4-linux-<architecture>/
```

Before extraction it verifies the complete archive against the architecture-specific SHA-256 checksum pinned in `scripts/lib.sh`. A failed download or mismatch stops setup without installing the archive.

The following commands must already exist:

- `curl`;
- `tar`;
- `sha256sum`;
- `mktemp`.

Setup is idempotent. When the expected Go binary is already present and reports the pinned version, it returns immediately. It never writes to `/usr/local`, the user's global Go configuration, or another system directory, and it never needs `sudo`.

If the target tool directory exists but is incomplete or contains a different toolchain, setup stops and asks the user to move or remove it. This avoids silently deleting local data.

## `build.sh`

Native Linux or WSL build:

```bash
./scripts/build.sh
```

Output:

```text
build/constatra
```

Linux AMD64 and Windows AMD64 cross-build:

```bash
./scripts/build.sh --all
```

Outputs:

```text
build/constatra-linux-amd64
build/constatra-windows-amd64.exe
```

If setup has not run yet, `build.sh` invokes it automatically. Builds use:

- `CGO_ENABLED=0` for standalone executables;
- `-trimpath` for fewer machine-specific build paths;
- linker stripping for smaller binaries;
- the repository-local toolchain and caches.

Every invocation increments the fourth component in the tracked `VERSION` file before compiling. For example, `0.1.0.17` becomes `0.1.0.18`. A single `--all` invocation increments once and embeds the same value in both operating-system binaries.

## `run.sh`

```bash
./scripts/run.sh [Constàtra options]
```

`run.sh` always invokes the native build first. Go's incremental cache makes unchanged builds fast, while this policy guarantees the executable matches the checked-out source. Since this is a real build, it also advances `VERSION`. The script then uses `exec`, so signals such as `Ctrl-C` reach Constàtra directly and graceful shutdown behaves exactly as it does when the binary is launched manually.

`run.sh` is intended for Linux and WSL. To run the Windows executable, build it with `--all` and start `build/constatra-windows-amd64.exe` from Windows, PowerShell, Command Prompt, or WSL Windows integration.

## Local files and caches

The scripts keep all generated content in two ignored directories:

```text
.tools/   pinned Go SDK, build cache, and module cache
build/    generated Constàtra executables
```

Removing `build/` only forces an executable rebuild. Removing `.tools/` forces the toolchain to be downloaded and verified again on the next setup or build.

The project currently uses only the Go standard library, but the local module cache is configured in advance so future dependencies do not modify an unrelated global cache.

## Make targets versus scripts

The Makefile remains useful in environments where Go is already managed externally:

```bash
make build
make build-all
make test
make check
```

The scripts are the recommended first-run experience because they do not assume `go` is installed or available on `PATH`. Both workflows compile the same `./cmd/constatra` entry point with equivalent standalone-build settings.

Release automation and the exact version contract are documented in [Versioning and GitHub releases](versioning.md).
