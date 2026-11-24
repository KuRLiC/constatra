# Versioning and GitHub releases

Constàtra uses an explicit, monotonically increasing application version and a tag-driven GitHub Actions publication workflow.

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
- local `main` exactly synchronized with `origin/main`;
- no staged files;
- no tracked or untracked changes other than `VERSION`;
- permission to push commits to `main`.

Local Git authentication only needs to push `main` to `origin`. GitHub CLI is not required: tag and release creation run inside GitHub Actions with the repository's short-lived `GITHUB_TOKEN`.

## Dry-run preflight

```bash
./scripts/release.sh --dry-run
```

The dry run validates local commands, branch, index, and working-tree scope, then reports the current version, next version, and planned tag. It does not build, modify files, create commits or tags, contact GitHub, or push anything.

## Publish

```bash
./scripts/release.sh
```

The local script performs these steps in order:

1. validates local prerequisites and working-tree scope;
2. fetches `origin/main` and requires an exact local match;
3. checks the planned tag does not exist locally or remotely;
4. increments `VERSION` once and cross-builds Linux AMD64 and Windows AMD64;
5. runs the race-enabled Go test suite;
6. creates a local `build/SHA256SUMS` verification manifest;
7. commits only `VERSION` with a detailed release commit;
8. pushes `main`.

The pushed `VERSION` change triggers `.github/workflows/release.yml`. GitHub Actions then:

1. checks out the exact tagged source;
2. validates the four-component `VERSION` value;
3. runs `go vet` and the race-enabled tests;
4. independently builds Linux AMD64 and Windows AMD64 with the tagged version;
5. creates a fresh SHA-256 manifest;
6. creates or validates the annotated `v<version>` tag at that exact commit;
7. creates the GitHub Release with generated notes, or replaces its assets on a safe workflow rerun.

The uploaded files are:

```text
constatra-linux-amd64
constatra-windows-amd64.exe
SHA256SUMS
```

## Failure behavior

Failures before the release commit leave only normal local build outputs and the incremented `VERSION`; rerun the dry-run after fixing the cause.

If the Git push fails, the version commit remains local and no tag or release is created. Inspect the error before retrying.

If GitHub Actions fails after the push, the version commit is already public, but the workflow creates the tag only after tests and builds succeed. Do not rebuild or create another version merely to retry. Inspect the failed workflow, fix it when necessary, and rerun the job from GitHub. Reruns are idempotent: an existing tag must point to the same commit, and existing release assets are replaced.

## Repository permission

The workflow declares only:

```yaml
permissions:
  contents: write
```

That permission allows the tag-triggered job to create the release and upload assets. If the repository globally restricts workflow tokens to read-only access, enable read/write workflow permissions in the repository's Actions settings or explicitly permit the workflow to request `contents: write`.
