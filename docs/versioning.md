# Versioning and GitHub releases

Constàtra uses an explicit, monotonically increasing application version and a guarded GitHub publication workflow.

## Version format

The tracked root file `VERSION` contains four numeric components:

```text
major.minor.patch.build
```

The initial line is `0.1.0.0`. Project builds increment only the final component:

```text
0.1.0.0 → 0.1.0.1 → 0.1.0.2
```

Major, minor, and patch changes are deliberate product decisions and may be edited in `VERSION` as part of a reviewed change. The next build still increments the build component.

The exact value is embedded into every executable and appears in:

- `constatra --version`;
- the startup console message;
- the dashboard technical panel;
- `/api/state` as the `version` field.

## What counts as a build

The supported build entry points all route through `scripts/build.sh` and increment once:

```bash
./scripts/build.sh
./scripts/build.sh --all
./scripts/run.sh
make build
make build-all
```

`--all` produces two artifacts but represents one logical build, so Linux and Windows share one version. `run.sh` rebuilds before execution and therefore increments as well.

Direct ad hoc use of `go build` bypasses project version tracking and is intended only for low-level development experiments.

## Atomic increment

`scripts/version.sh increment` validates the current format, acquires `.tools/version.lock`, calculates the next build component, writes a temporary value, and atomically moves it over `VERSION`. A second concurrent build fails instead of reusing a version.

If a build process is forcibly killed while holding the lock, verify no build is running and remove only:

```text
.tools/version.lock/
```

The `VERSION` change is intentionally visible in Git. It records the latest local build and is the only pre-existing tracked modification accepted by the release workflow.

## Release prerequisites

Publishing requires:

- the `main` branch;
- the `origin` remote pointing to the intended repository;
- GitHub CLI `gh` installed and authenticated;
- local `main` exactly synchronized with `origin/main`;
- no staged files;
- no tracked or untracked changes other than `VERSION`;
- permission to push commits and tags and create releases.

Check GitHub authentication with:

```bash
gh auth status
```

## Dry-run preflight

```bash
./scripts/release.sh --dry-run
```

The dry run validates local commands, branch, index, and working-tree scope, then reports the current version, next version, and planned tag. It does not build, modify files, create commits or tags, contact GitHub, or push anything.

## Publish

```bash
./scripts/release.sh
```

The script performs these steps in order:

1. validates local prerequisites and working-tree scope;
2. verifies GitHub authentication;
3. fetches `origin/main` and requires an exact local match;
4. checks the planned tag does not exist locally or remotely;
5. increments `VERSION` once and cross-builds Linux AMD64 and Windows AMD64;
6. runs the race-enabled Go test suite;
7. creates `build/SHA256SUMS` for both executables;
8. commits only `VERSION` with a detailed release commit;
9. creates an annotated `v<version>` tag;
10. atomically pushes `main` and the tag;
11. creates a GitHub Release with generated notes and all three assets.

The uploaded files are:

```text
constatra-linux-amd64
constatra-windows-amd64.exe
SHA256SUMS
```

## Failure behavior

Failures before the release commit leave only normal local build outputs and the incremented `VERSION`; rerun the dry-run after fixing the cause.

If the atomic Git push fails, the commit and tag remain local and nothing is partially updated on the remote. Inspect the error before retrying.

If GitHub Release creation fails after the atomic push, the version commit and tag are already public. Do not rebuild or create another version merely to retry the upload. Use the printed tag and run an equivalent `gh release create` command with the existing files, or remove the remote tag only after deliberately deciding to abandon that release.
