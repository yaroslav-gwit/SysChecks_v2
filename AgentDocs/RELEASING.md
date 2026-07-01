# SysChecks Release Playbook

Step-by-step guide for cutting a SysChecks release. Written for humans and LLM
agents. The release is driven from `main` with `./release.sh`.

> Companion docs: `AgentDocs/E2E_TESTING.md` (validation runbook) and the
> repo-root `RELEASES.md` (GitHub CLI setup, troubleshooting, friendly intros).

## Before you release

1. **Run the E2E tests.** Work through `AgentDocs/E2E_TESTING.md` against the
   container matrix (and a VM where the runbook calls for one). Do not release
   with failing checks unless the failure is documented and understood. See
   [Container gotchas](E2E_TESTING.md#container-gotchas) for known false
   failures (banner needs a TTY; minimal EL images need `/etc/cron.d` and
   `/boot`; use `sudo docker`).
2. **`go test ./...` and `go vet ./...` must pass.**
3. **Land the code first.** `release.sh` only tags `HEAD` and pushes the tag —
   it does **not** commit or push the branch. Commit your changes and
   `git push origin main` before releasing, so the tag points at pushed code.
4. **Pick the version** (SemVer): a new feature → minor bump; a bug-fix-only
   release → patch bump.
5. **Update `CHANGELOG.md`.** Move the `[Unreleased]` items into a dated
   `## [X.Y.Z] - YYYY-MM-DD` section and add the compare links at the bottom
   (`[X.Y.Z]: .../compare/vPREV...vX.Y.Z`). `release.sh` extracts this section
   into the release body.
6. **Write a friendly intro.** Every release body opens with a warm, human,
   2-4 sentence summary — what users get and why it matters. Do **not** ship the
   generic fallback. Provide it, in priority order:
   - `export RELEASE_INTRO="In this release we're excited to ..."` (preferred), or
   - create `release-notes/vX.Y.Z.md` with the intro text.

## Cutting the release

```bash
# Dry run first — review the summary and generated notes
./release.sh -n X.Y.Z

# Real run (-f: untracked .lastplane/ files make the tree look "dirty",
# which would otherwise block the clean-tree check)
export RELEASE_INTRO="In this release we're excited to ..."
./release.sh -f X.Y.Z
```

`release.sh` then:

- runs `go test ./...`;
- builds `syschecks-linux-amd64` via `sudo -E ./build-advanced.sh docker ubuntu18`
  (Ubuntu 18.04 base for broad glibc compatibility; the artifact is renamed from
  `syschecks-ubuntu18`);
- cross-compiles `syschecks-linux-arm64`;
- packs the self-extracting offline installers `syschecks-installer-amd64.run`
  and `syschecks-installer-arm64.run` (via `create-run-installer.sh`) for
  air-gapped hosts;
- generates `checksums-sha256.txt`;
- creates the GitHub release and pushes an annotated tag `vX.Y.Z`.

### Prerequisites

- `gh` authenticated (`gh auth status`).
- Non-interactive `sudo docker`. Plain `docker` may be permission-denied; the
  script uses `sudo` for the Docker build. Confirm `sudo -n docker version` works.
- Go 1.21+.

> **Asset name contract:** the amd64 asset MUST stay named
> `syschecks-linux-amd64`. `syschecks self-update` downloads it by that exact
> name (`syschecks-<os>-<arch>`); renaming it breaks self-update.

## After releasing

- **Verify the release body** on GitHub: it should open with the friendly intro,
  contain no stray log lines, and end with a correct
  `compare/vPREV...vX.Y.Z` link. (Log output goes to stderr so it can't leak into
  the captured notes; regenerate with `gh release edit vX.Y.Z --notes-file ...`
  if you need to fix it.)
- **Confirm assets and checksums** are attached (2 binaries, 2 `.run`, checksums).
- **Smoke-test self-update** against a real host — e.g. the
  [reference test host](E2E_TESTING.md#reference-test-host): run
  `syschecks self-update` and confirm it fetches the freshly published version,
  the command still works afterward, and `syschecks updates` looks right.

## Quick checklist

- [ ] E2E runbook passed
- [ ] `go test ./...` / `go vet ./...` green
- [ ] Code committed and pushed to `main`
- [ ] Version chosen (SemVer)
- [ ] `CHANGELOG.md` updated (dated section + compare links)
- [ ] Friendly intro set (`RELEASE_INTRO` or `release-notes/vX.Y.Z.md`)
- [ ] `./release.sh -n X.Y.Z` reviewed, then `./release.sh -f X.Y.Z`
- [ ] Release body, assets, and checksums verified on GitHub
- [ ] `self-update` smoke-tested on a real host
