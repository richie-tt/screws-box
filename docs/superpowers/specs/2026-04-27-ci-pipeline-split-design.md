# CI pipeline split — design

**Status:** approved
**Date:** 2026-04-27
**Author:** Robert Tkocz (with Claude)

## Context

Today the project has two GitHub Actions workflows:

- `ci.yml` — runs on `pull_request` to `master`. One `pre-commit` job that runs lint + format + Dockerfile lint + go-build + go-test bundled inside the pre-commit framework, then a `build` job that runs `go build` and `go test ./...`.
- `release.yml` — runs on `push: master`. Same `pre-commit` and `build` jobs, then `version` (semantic-tag derivation), `release` (GH release), `docker` (GHCR push).

Both workflows duplicate the gate jobs verbatim. After PR #23 enabled SHA pinning across the board and PR #26 replaced `golangci-lint-action` and `pre-commit/action` with checksum-verified shell installs, the workflow YAML has grown — and the pre-commit job now mixes "real linting" (golangci-lint, hadolint) with operations that are better expressed as first-class CI jobs (`go build`, `go test`).

## Goals

1. Split the gate into focused, parallelizable jobs: **lint**, **test**, **coverage**, **build**.
2. Wire **coverage** to Codecov using the existing `CODECOV_TOKEN` secret.
3. Eliminate gate-job duplication between PR and Release workflows via a reusable workflow.
4. Preserve every property earned in PRs #23 and #26: every action SHA-pinned, every binary install SHA-256-verified, no transitive policy violations.
5. Rename the PR workflow from the generic `CI` to something more precise.

## Non-goals

- Coverage percentage gating. Codecov's PR comment will surface deltas; a hard floor can be added later via `codecov.yml` if desired.
- Artifact uploads from `build`. The `build` job exists purely to verify "this still compiles after lint/test/coverage pass."
- Replacing pre-commit. The `.pre-commit-config.yaml` continues to drive local-hook enforcement; CI just stops abusing it as a build-and-test runner.
- Splitting `release` or `docker` further. They are already correctly scoped after PR #23.

## Architecture

Three workflow files:

```
.github/workflows/
├── pr.yml          ← renamed from ci.yml, runs on pull_request
├── release.yml     ← unchanged path, runs on push: master
└── _validate.yml   ← new, workflow_call only — defines the four gate jobs
```

The leading underscore on `_validate.yml` is convention for "called, not triggered directly."

### Job graph in `_validate.yml`

```
       ┌─ lint  ─┐
       ├─ test  ─┼─→ build   (gate; only runs if all three pass)
       └─ cov   ─┘
```

The `build` job declares `needs: [lint, test, coverage]` so it runs only when the three parallel jobs all succeed. Each job does its own `actions/checkout` + `actions/setup-go` because GitHub Actions does not share workspaces across jobs.

### Per-job contracts

| Job | Command | Notable flags |
|---|---|---|
| **lint** | `pre-commit run --all-files` (drives golangci-lint v2.11.4 + hadolint v2.12.0 + format hooks via the existing `.pre-commit-config.yaml`) | `--show-diff-on-failure --color=always`. Tools installed via SHA-256-verified curl + `pip install --user pre-commit==4.6.0`, identical to PR #26. |
| **test** | `go test ./... -count=1 -race -shuffle=on` | `-race` enables the data-race detector; `-shuffle=on` randomizes test order to surface order dependencies. |
| **coverage** | `go test ./... -count=1 -coverprofile=coverage.txt -covermode=atomic`, then upload via `codecov/codecov-action@75cd11691c0faa626561e295848008c8a7dddffe # v5` with `token: ${{ secrets.CODECOV_TOKEN }}` and `files: coverage.txt`. | `covermode=atomic` is required when test execution involves goroutines. The upload step has `if: ${{ secrets.CODECOV_TOKEN != '' }}` so fork PRs (no token) succeed without uploading. |
| **build** | `go build -ldflags="-s -w" ./cmd/screwsbox` | `needs: [lint, test, coverage]`. No artifact published. |

### `pr.yml`

Triggers on `pull_request` to `master` with the existing `paths-ignore`. Single job:

```yaml
jobs:
  validate:
    uses: ./.github/workflows/_validate.yml
    secrets: inherit
```

`secrets: inherit` exposes `CODECOV_TOKEN` to same-repo PRs. Fork PRs do not receive secrets — the conditional upload step inside `_validate.yml` handles that gracefully.

### `release.yml`

Same trigger as today (`push: master`, paths-ignored). New job graph:

```
push to master
   └─ validate (uses ./.github/workflows/_validate.yml)
        └─ version (semantic-tag derivation, unchanged)
             └─ release (needs: [validate, version], if: !skip)
                  └─ docker (needs: [version, release], if: !skip)
```

`release` and `docker` keep the job-scoped `permissions:` blocks (`contents: write` and `packages: write` respectively) introduced by PR #23.

### `.pre-commit-config.yaml` cleanup

Remove these two local hooks:

```yaml
- id: go-unit-tests   # ran `go test ./...` with pass_filenames: false → slow, redundant with CI test job
- id: go-build        # likewise for go build
```

Keep all of: `go-fmt`, `golangci-lint`, `go-mod-tidy`, `go-mod-verify`, `hadolint`, and the four `pre-commit/pre-commit-hooks` formatters.

### Codecov uploader

`codecov/codecov-action@v5` was inspected against the SHA-pinning policy and is **safe**: its only transitive `uses:` is `actions/github-script@60a0d83039c74a4aee543508d2ffcb1c3799cdea` — already SHA-pinned upstream. v5 contains no `actions/cache` reference, so it does not trigger the `sha_pinning_required` block that hit `golangci-lint-action` and `pre-commit/action`.

Pin: `codecov/codecov-action@75cd11691c0faa626561e295848008c8a7dddffe # v5` with trailing version comment, exactly as every other action in the repo.

Inputs:

```yaml
- uses: codecov/codecov-action@75cd11691c0faa626561e295848008c8a7dddffe # v5
  if: ${{ secrets.CODECOV_TOKEN != '' }}
  with:
    token: ${{ secrets.CODECOV_TOKEN }}
    files: coverage.txt
    fail_ci_if_error: true
```

`fail_ci_if_error: true` keeps the strict-upload contract from the risk register: a real upstream outage fails the gate; we re-run.

### Coverage badge in README

Add a Codecov badge to the top of `README.md`, immediately under the `# Screws Box` heading and before the existing `assets/main.png` screenshot, so coverage is visible at a glance:

```markdown
[![codecov](https://codecov.io/gh/richie-tt/screws-box/branch/master/graph/badge.svg)](https://codecov.io/gh/richie-tt/screws-box)
```

The badge URL is the standard public Codecov URL — no token needed for the badge itself; Codecov serves it from the public report on master. (If the repo were private, a `?token=…` query parameter would be required, but `richie-tt/screws-box` is public.)

## Ruleset implications

The default-branch ruleset currently requires status checks named `Pre-commit checks` and `Build and test`. After this change those names disappear. New required checks should be:

```
validate / lint
validate / test
validate / coverage
validate / build
```

GitHub surfaces reusable-workflow jobs to branch protection as `<caller-job-id> / <callee-job-id>` — caller job-id is `validate`, callee job-ids are `lint`, `test`, `coverage`, `build`.

**Implementation order to avoid a broken gate:**

1. Open the PR with the new workflows. Old required checks (`Pre-commit checks`, `Build and test`) will appear as missing on this PR (because the new workflows don't emit them) and the ruleset will block merge.
2. Manually merge once via admin override OR temporarily relax the ruleset to require the new check names.
3. After merge, the new checks run on master at least once so GitHub registers their names; then update the ruleset to require the new names.

The PR description must call this out explicitly.

## Open items at implementation time

- Verify the reusable-workflow caller-id surfacing (`validate / lint` etc.) by inspecting the first PR's checks page before recommending the ruleset update.
- Decide whether to keep or drop the `Pre-commit checks` / `Build and test` job names entirely (probably drop — they're misnomers now).

## Risk register

| Risk | Mitigation |
|---|---|
| Adding `-race` surfaces a real race that has been silent until now | Land the four-job split first with plain `go test`; introduce `-race` in a follow-up PR if it complicates this one. |
| Reusable workflow surfaces unfamiliar status-check names, breaking the ruleset gate at merge time | Land in two steps as documented in "Ruleset implications" above. |
| Transient codecov.io outage causes the upload step to fail | Token is verified correct, so a failure means a real upstream outage. Step is *strict* (no `continue-on-error`) — the gate fails and the run is re-run when Codecov is back. Accepts brief upstream-coupled flakiness in exchange for guaranteed coverage tracking on master. |
| Forks cannot run coverage upload | Conditional `if: ${{ secrets.CODECOV_TOKEN != '' }}` short-circuits the upload step; the test+coverage measurement still runs. |
