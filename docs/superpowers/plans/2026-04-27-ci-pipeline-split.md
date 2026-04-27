# CI Pipeline Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the monolithic `Pre-commit checks` job into four focused, parallelizable jobs (lint, test, coverage, build) defined once in a reusable workflow and called by both PR and Release workflows. Wire coverage to Codecov. Add a coverage badge to the README.

**Architecture:** A new reusable workflow `_validate.yml` defines `lint` / `test` / `coverage` running in parallel, gated by a final `build` job. The renamed `pr.yml` (formerly `ci.yml`) calls `_validate.yml` on `pull_request`. The existing `release.yml` calls the same `_validate.yml` on `push: master`, then chains `version → release → docker`. Every action stays SHA-pinned; every binary install stays SHA-256-verified. The Codecov v5 action is used (verified policy-compliant earlier in the session — its sole transitive dep `actions/github-script` is already SHA-pinned upstream).

**Tech Stack:** GitHub Actions (reusable workflows via `workflow_call`), Go 1.x via `actions/setup-go`, golangci-lint v2.11.4 + hadolint v2.12.0 + pre-commit 4.6.0 (all installed as pinned binaries, not as actions), `codecov/codecov-action@v5` (SHA `75cd1169…`).

**Branch context:** This plan executes on branch `feat/split-ci-pipeline`, which is stacked on `fix/sha-pinning-transitive-deps`. That parent branch already has the direct-shell-install pattern for golangci-lint and pre-commit; this plan reuses those install snippets verbatim inside `_validate.yml`.

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `.github/workflows/_validate.yml` | **create** | Reusable workflow defining the four gate jobs. The single source of truth for "what does it mean for code to be ready to merge / release." |
| `.github/workflows/pr.yml` | **create** (rename of `ci.yml`) | Triggered on `pull_request`. Thin caller that delegates to `_validate.yml`. |
| `.github/workflows/ci.yml` | **delete** | Replaced by `pr.yml`. |
| `.github/workflows/release.yml` | **modify** | Replace inline `pre-commit` and `build` jobs with a single `validate:` caller; rest of the file (`version`, `release`, `docker`) is unchanged. |
| `.pre-commit-config.yaml` | **modify** | Remove `go-build` and `go-unit-tests` local hooks (now redundant with CI test/build jobs). |
| `README.md` | **modify** | Add Codecov badge near the top of the file. |

Each task below produces a single self-contained commit. Frequent commits make bisecting easy if the gate misbehaves on the first PR run.

---

## Task 1: Create `_validate.yml` skeleton with the lint job

**Files:**
- Create: `.github/workflows/_validate.yml`

- [ ] **Step 1: Write the file**

Write `.github/workflows/_validate.yml` with this content (lint job only — other jobs added in subsequent tasks):

```yaml
name: Validate

on:
  workflow_call:

permissions:
  contents: read

jobs:
  lint:
    name: lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.0
        with:
          persist-credentials: false

      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.0.0
        with:
          go-version-file: go.mod

      - name: Install golangci-lint
        env:
          GOLANGCI_LINT_VERSION: v2.11.4
          GOLANGCI_LINT_SHA256: 200c5b7503f67b59a6743ccf32133026c174e272b930ee79aa2aa6f37aca7ef1
        run: |
          tarball="golangci-lint-${GOLANGCI_LINT_VERSION#v}-linux-amd64.tar.gz"
          curl -fsSL -o "$tarball" "https://github.com/golangci/golangci-lint/releases/download/${GOLANGCI_LINT_VERSION}/${tarball}"
          echo "${GOLANGCI_LINT_SHA256}  ${tarball}" | sha256sum -c -
          tar -xzf "$tarball"
          sudo install -m 0755 "${tarball%.tar.gz}/golangci-lint" /usr/local/bin/golangci-lint
          rm -rf "$tarball" "${tarball%.tar.gz}"

      - name: Install hadolint
        env:
          HADOLINT_VERSION: v2.12.0
          HADOLINT_SHA256: 56de6d5e5ec427e17b74fa48d51271c7fc0d61244bf5c90e828aab8362d55010
        run: |
          curl -fsSL -o hadolint "https://github.com/hadolint/hadolint/releases/download/${HADOLINT_VERSION}/hadolint-Linux-x86_64"
          echo "${HADOLINT_SHA256}  hadolint" | sha256sum -c -
          chmod +x hadolint
          sudo mv hadolint /usr/local/bin/

      - name: Run pre-commit
        env:
          PRE_COMMIT_VERSION: 4.6.0
        run: |
          python3 -m pip install --user "pre-commit==${PRE_COMMIT_VERSION}"
          python3 -m pre_commit run --all-files --show-diff-on-failure --color=always
```

- [ ] **Step 2: Validate YAML parses**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/_validate.yml'))" && echo OK`
Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/_validate.yml
git commit -m "feat(ci): add reusable validate workflow with lint job

First commit of the four-job validate split. Subsequent commits add
test, coverage, and build jobs. Identical install pattern (curl + sha256
verify) as the parent branch's fix for transitive-deps SHA-pinning."
```

---

## Task 2: Add the `test` job to `_validate.yml`

**Files:**
- Modify: `.github/workflows/_validate.yml`

- [ ] **Step 1: Append the test job**

After the closing of the `lint:` job (so `test:` is a sibling under `jobs:`), insert:

```yaml
  test:
    name: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.0
        with:
          persist-credentials: false

      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.0.0
        with:
          go-version-file: go.mod

      - name: Run tests
        run: go test ./... -count=1 -race -shuffle=on
```

- [ ] **Step 2: Validate YAML parses**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/_validate.yml'))" && echo OK`
Expected: `OK`

- [ ] **Step 3: Verify locally that tests still pass with -race**

Run: `go test ./... -count=1 -race -shuffle=on`
Expected: `ok` for every package, no `DATA RACE` output. If a real race surfaces, **stop and report** — that is a separate fix that does not belong in this PR.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/_validate.yml
git commit -m "feat(ci): add test job to validate workflow

Runs go test with -race -shuffle=on for stronger CI signal than the
plain go test the workflow ran before."
```

---

## Task 3: Add the `coverage` job to `_validate.yml`

**Files:**
- Modify: `.github/workflows/_validate.yml`

- [ ] **Step 1: Append the coverage job**

After the closing of the `test:` job, insert:

```yaml
  coverage:
    name: coverage
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.0
        with:
          persist-credentials: false

      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.0.0
        with:
          go-version-file: go.mod

      - name: Generate coverage profile
        run: go test ./... -count=1 -coverprofile=coverage.txt -covermode=atomic

      - name: Upload coverage to Codecov
        if: ${{ secrets.CODECOV_TOKEN != '' }}
        uses: codecov/codecov-action@75cd11691c0faa626561e295848008c8a7dddffe # v5
        with:
          token: ${{ secrets.CODECOV_TOKEN }}
          files: coverage.txt
          fail_ci_if_error: true
```

- [ ] **Step 2: Validate YAML parses**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/_validate.yml'))" && echo OK`
Expected: `OK`

- [ ] **Step 3: Verify locally that coverage profile is produced**

Run: `go test ./... -count=1 -coverprofile=/tmp/coverage.txt -covermode=atomic && head -1 /tmp/coverage.txt`
Expected: First line begins with `mode: atomic`. Then: `rm /tmp/coverage.txt`

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/_validate.yml
git commit -m "feat(ci): add coverage job with Codecov upload

Runs go test with -coverprofile=coverage.txt -covermode=atomic and
uploads via codecov/codecov-action@v5 (SHA-pinned, verified
policy-compliant — its sole transitive dep is the already-pinned
actions/github-script). Upload is conditional on CODECOV_TOKEN being
present so fork PRs do not break.

fail_ci_if_error: true keeps the strict-upload contract."
```

---

## Task 4: Add the `build` gate job to `_validate.yml`

**Files:**
- Modify: `.github/workflows/_validate.yml`

- [ ] **Step 1: Append the build job**

After the closing of the `coverage:` job, insert:

```yaml
  build:
    name: build
    runs-on: ubuntu-latest
    needs: [lint, test, coverage]
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.0
        with:
          persist-credentials: false

      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.0.0
        with:
          go-version-file: go.mod

      - name: Build binary
        run: go build -ldflags="-s -w" ./cmd/screwsbox
```

- [ ] **Step 2: Validate YAML parses**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/_validate.yml'))" && echo OK`
Expected: `OK`

- [ ] **Step 3: Verify the four jobs are wired correctly**

Run:
```bash
python3 -c "
import yaml
d = yaml.safe_load(open('.github/workflows/_validate.yml'))
jobs = d['jobs']
assert set(jobs.keys()) == {'lint', 'test', 'coverage', 'build'}, jobs.keys()
assert jobs['build']['needs'] == ['lint', 'test', 'coverage'], jobs['build']['needs']
print('OK — 4 jobs, build gates the other three')
"
```
Expected: `OK — 4 jobs, build gates the other three`

- [ ] **Step 4: Verify locally that the build command works**

Run: `go build -ldflags="-s -w" -o /tmp/screwsbox ./cmd/screwsbox && rm /tmp/screwsbox && echo OK`
Expected: `OK`

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/_validate.yml
git commit -m "feat(ci): add build gate job to validate workflow

Final job — declares needs: [lint, test, coverage] so it only runs
if all three parallel jobs pass. Pure compile-only verification, no
artifact published."
```

---

## Task 5: Replace `ci.yml` with `pr.yml` calling the reusable workflow

**Files:**
- Create: `.github/workflows/pr.yml`
- Delete: `.github/workflows/ci.yml`

- [ ] **Step 1: Create `pr.yml`**

Write `.github/workflows/pr.yml` with this content:

```yaml
name: PR

on:
  pull_request:
    branches: [master]
    paths-ignore:
      - 'LICENSE'
      - '*.md'
      - 'assets/**'
      - '.vscode/**'

permissions:
  contents: read

concurrency:
  group: pr-${{ github.ref }}
  cancel-in-progress: true

jobs:
  validate:
    uses: ./.github/workflows/_validate.yml
    secrets: inherit
```

- [ ] **Step 2: Delete the old `ci.yml`**

Run: `git rm .github/workflows/ci.yml`
Expected: `rm '.github/workflows/ci.yml'`

- [ ] **Step 3: Validate YAML parses**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/pr.yml'))" && echo OK`
Expected: `OK`

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/pr.yml
git commit -m "refactor(ci): rename ci.yml to pr.yml as a thin caller

pr.yml triggers on pull_request and delegates to the new reusable
_validate.yml. secrets: inherit exposes CODECOV_TOKEN to same-repo PRs;
fork PRs work too because the upload step inside _validate.yml is
conditional on the token being present."
```

---

## Task 6: Refactor `release.yml` to use `_validate.yml`

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Read the current file**

Run: `cat .github/workflows/release.yml`
Note the existing `pre-commit:`, `build:`, `version:`, `release:`, `docker:` jobs and their `needs:` chains.

- [ ] **Step 2: Replace the file**

Overwrite `.github/workflows/release.yml` with this content. The `version`, `release`, and `docker` jobs are kept verbatim from current master except their `needs:` chains are updated to depend on the new `validate` caller:

```yaml
name: Release

on:
  push:
    branches: [master]
    paths-ignore:
      - 'LICENSE'
      - '*.md'
      - 'assets/**'
      - '.vscode/**'

permissions:
  contents: read

concurrency:
  group: release-${{ github.ref }}
  cancel-in-progress: false

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  validate:
    uses: ./.github/workflows/_validate.yml
    secrets: inherit

  version:
    needs: validate
    runs-on: ubuntu-latest
    outputs:
      tag: ${{ steps.version.outputs.tag }}
      skip: ${{ steps.version.outputs.skip }}
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.0
        with:
          fetch-depth: 0
          persist-credentials: false

      - name: Determine version from commits
        id: version
        run: |
          LATEST_TAG=$(git tag -l 'v*' --sort=-v:refname | head -n1)
          if [ -z "$LATEST_TAG" ]; then
            LATEST_TAG="v0.0.0"
          fi

          MAJOR=$(echo "$LATEST_TAG" | sed 's/v//' | cut -d. -f1)
          MINOR=$(echo "$LATEST_TAG" | sed 's/v//' | cut -d. -f2)
          PATCH=$(echo "$LATEST_TAG" | sed 's/v//' | cut -d. -f3)

          if [ "$LATEST_TAG" = "v0.0.0" ]; then
            COMMITS=$(git log --oneline --format="%s")
          else
            COMMITS=$(git log --oneline --format="%s" "${LATEST_TAG}..HEAD")
          fi

          BUMP="none"
          while IFS= read -r msg; do
            if echo "$msg" | grep -qiE '^feat(\(.+\))?:'; then
              BUMP="minor"
            elif echo "$msg" | grep -qiE '^fix(\(.+\))?:' && [ "$BUMP" != "minor" ]; then
              BUMP="patch"
            fi
          done <<< "$COMMITS"

          if [ "$BUMP" = "minor" ]; then
            MINOR=$((MINOR + 1))
            PATCH=0
          elif [ "$BUMP" = "patch" ]; then
            PATCH=$((PATCH + 1))
          else
            echo "No fix: or feat: commits found — skipping release."
            echo "skip=true" >> "$GITHUB_OUTPUT"
            exit 0
          fi

          NEW_TAG="v${MAJOR}.${MINOR}.${PATCH}"
          echo "tag=$NEW_TAG" >> "$GITHUB_OUTPUT"
          echo "skip=false" >> "$GITHUB_OUTPUT"
          echo "New version: $NEW_TAG (bump: $BUMP)"

  release:
    runs-on: ubuntu-latest
    needs: [version]
    if: needs.version.outputs.skip != 'true'
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.0
        with:
          fetch-depth: 0

      - name: Create and push tag
        env:
          TAG: ${{ needs.version.outputs.tag }}
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git tag "$TAG"
          git push origin "$TAG"

      - name: Generate changelog
        id: changelog
        run: |
          LATEST_TAG=$(git tag -l 'v*' --sort=-v:refname | head -n1)
          PREV_TAG=$(git tag -l 'v*' --sort=-v:refname | sed -n '2p')
          if [ -z "$PREV_TAG" ]; then
            LOG=$(git log --oneline)
          else
            LOG=$(git log --oneline "${PREV_TAG}..${LATEST_TAG}")
          fi
          {
            echo "changelog<<EOF"
            echo "$LOG"
            echo "EOF"
          } >> "$GITHUB_OUTPUT"

      - name: Create GitHub Release
        uses: softprops/action-gh-release@b4309332981a82ec1c5618f44dd2e27cc8bfbfda # v3.0.0
        with:
          tag_name: ${{ needs.version.outputs.tag }}
          name: Release ${{ needs.version.outputs.tag }}
          body: |
            ## Changes
            ${{ steps.changelog.outputs.changelog }}
          generate_release_notes: true

  docker:
    runs-on: ubuntu-latest
    needs: [version, release]
    if: needs.version.outputs.skip != 'true'
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.0
        with:
          persist-credentials: false

      - uses: docker/login-action@4907a6ddec9925e35a0a9e82d7399ccc52663121 # v4.0.0
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - uses: docker/metadata-action@030e881283bb7a6894de51c315a6bfe6a94e05cf # v6.0.1
        id: meta
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=raw,value=${{ needs.version.outputs.tag }}
            type=raw,value=latest

      - uses: docker/build-push-action@bcafcacb16a39f128d818304e6c9c0c18556b85f # v7.0.0
        with:
          context: .
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          build-args: |
            VERSION=${{ needs.version.outputs.tag }}
```

- [ ] **Step 3: Validate YAML parses**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))" && echo OK`
Expected: `OK`

- [ ] **Step 4: Verify the chain wires correctly**

Run:
```bash
python3 -c "
import yaml
d = yaml.safe_load(open('.github/workflows/release.yml'))
jobs = d['jobs']
assert set(jobs.keys()) == {'validate', 'version', 'release', 'docker'}, jobs.keys()
assert 'uses' in jobs['validate'] and jobs['validate']['uses'].endswith('_validate.yml')
assert jobs['version']['needs'] == 'validate'
assert jobs['release']['needs'] == ['version']
assert jobs['docker']['needs'] == ['version', 'release']
print('OK — validate gates version; release/docker chain unchanged')
"
```
Expected: `OK — validate gates version; release/docker chain unchanged`

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "refactor(ci): release.yml uses reusable _validate.yml

Replaces the duplicated pre-commit and build jobs with a single
validate caller. version/release/docker chain unchanged in behavior;
their needs: chains are updated so version waits on validate.
release and docker keep their job-scoped permissions blocks."
```

---

## Task 7: Remove redundant pre-commit hooks

**Files:**
- Modify: `.pre-commit-config.yaml`

- [ ] **Step 1: Read current config**

Run: `cat .pre-commit-config.yaml`

- [ ] **Step 2: Remove the `go-build` and `go-unit-tests` hook entries**

Edit `.pre-commit-config.yaml`. Find this section:

```yaml
  - repo: local
    hooks:
      - id: go-unit-tests
        name: go unit tests
        entry: go test ./...
        pass_filenames: false
        language: golang
        types: [go]
      - id: go-mod-verify
        name: go mod verify
        entry: go mod verify
        pass_filenames: false
        language: golang
        types: [go]
      - id: hadolint
        name: Lint Dockerfiles
        description: Runs hadolint to lint Dockerfiles
        language: system
        types: ["dockerfile"]
        entry: hadolint
```

Remove the **`go-unit-tests`** hook (the first entry under `repo: local`). The `go-mod-verify` and `hadolint` entries stay. Also check the upstream `dnephin/pre-commit-golang` block — if it still contains a `go-build` hook id, remove that line:

```yaml
  - repo: https://github.com/dnephin/pre-commit-golang
    rev: v0.5.1
    hooks:
      - id: go-fmt
      - id: golangci-lint
      - id: go-build       # ← remove this line
      - id: go-mod-tidy
```

After the edit the file should no longer contain `go-build` or `go-unit-tests` strings.

- [ ] **Step 3: Validate YAML parses and the strings are gone**

Run:
```bash
python3 -c "import yaml; yaml.safe_load(open('.pre-commit-config.yaml'))" && echo "YAML OK"
! grep -q "go-build\|go-unit-tests" .pre-commit-config.yaml && echo "hooks removed" || echo "FAIL: still present"
```
Expected:
```
YAML OK
hooks removed
```

- [ ] **Step 4: Verify pre-commit still runs locally**

Run: `pre-commit run --all-files`
Expected: All remaining hooks pass (or skip with no files matched). No reference to `go-build` or `go-unit-tests` in the output.

- [ ] **Step 5: Commit**

```bash
git add .pre-commit-config.yaml
git commit -m "chore: remove go-build and go-unit-tests pre-commit hooks

Both hooks ran with pass_filenames: false meaning they re-ran the full
build/test on every commit regardless of which files changed. The new
CI test and build jobs run the same commands properly. Keeping them
in pre-commit just slowed down local commits with no extra signal."
```

---

## Task 8: Add Codecov badge to README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Read the current README header**

Run: `head -10 README.md`
Expected output (verify exact text before editing):

```
# Screws Box

A web application for managing hardware organizer boxes. Quickly find which container holds your screws, bolts, washers, and other small parts.

![Grid View Screenshot](assets/main.png)
```

- [ ] **Step 2: Insert the badge between the title and the description**

Edit `README.md`. Replace:

```
# Screws Box

A web application for managing hardware organizer boxes.
```

with:

```
# Screws Box

[![codecov](https://codecov.io/gh/richie-tt/screws-box/branch/master/graph/badge.svg)](https://codecov.io/gh/richie-tt/screws-box)

A web application for managing hardware organizer boxes.
```

(Only the line containing the badge is new. The blank line above the description is preserved.)

- [ ] **Step 3: Verify the change**

Run: `head -5 README.md`
Expected:
```
# Screws Box

[![codecov](https://codecov.io/gh/richie-tt/screws-box/branch/master/graph/badge.svg)](https://codecov.io/gh/richie-tt/screws-box)

A web application for managing hardware organizer boxes. Quickly find which container holds your screws, bolts, washers, and other small parts.
```

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add Codecov coverage badge to README

Standard public Codecov badge URL — no token needed. Visible at the
top of the README so coverage status is immediately apparent."
```

---

## Task 9: Push branch and open PR with ruleset-update guidance

**Files:** none — this task only touches the remote.

- [ ] **Step 1: Sanity check the local branch**

Run:
```bash
git status
git log --oneline fix/sha-pinning-transitive-deps..HEAD
```
Expected: working tree clean, exactly 8 new commits beyond the parent branch (one per Task 1–8).

- [ ] **Step 2: Push the branch**

Run: `git push -u origin feat/split-ci-pipeline`
Expected: branch is pushed and a `Create a pull request` URL is printed.

- [ ] **Step 3: Open the PR**

Run:
```bash
gh pr create --title "feat(ci): split pre-commit job into lint/test/coverage/build + Codecov" --body "$(cat <<'EOF'
## Summary
Splits the monolithic `Pre-commit checks` job into four focused jobs (`lint`, `test`, `coverage`, `build`) defined once in a new reusable workflow `_validate.yml` and called by both PR and Release workflows. Wires coverage to Codecov via `codecov/codecov-action@v5` (SHA-pinned, verified policy-compliant). Adds a Codecov badge to the README.

Stacked on #26 — please merge that one first.

Design doc: `docs/superpowers/specs/2026-04-27-ci-pipeline-split-design.md`

## ⚠️ Required ruleset update before this PR can merge

The default-branch ruleset currently requires status checks named `Pre-commit checks` and `Build and test`. After this PR those names disappear and the gate would block this PR forever. Update the ruleset to require these check names instead:

```
validate / lint
validate / test
validate / coverage
validate / build
```

Process: let the PR's CI run once so GitHub registers the new check names, then update the ruleset, then merge.

## Test plan
- [ ] All four `validate / *` checks pass on this PR
- [ ] Codecov uploads successfully and posts a coverage comment on the PR
- [ ] Codecov badge resolves in the README preview
- [ ] After merge, master Release run reaches `version` → `release` → `docker` (or skips with the no-feat/fix message)
- [ ] `pre-commit run --all-files` locally still succeeds with the slimmer hook list

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Expected: PR URL is printed.

- [ ] **Step 4: Watch the first PR run**

Run: `gh pr checks --watch`
Expected: all four `validate / *` checks pass. If `validate / coverage` fails on the Codecov upload step, capture the error from `gh run view <run-id> --log-failed` and report it — the token might need its scope or slug verified.

---

## Self-review checklist

- [x] **Spec coverage:** Every section of the spec maps to a task — `_validate.yml` (Tasks 1–4), `pr.yml` (Task 5), `release.yml` refactor (Task 6), pre-commit cleanup (Task 7), README badge (Task 8), ruleset-update guidance (Task 9 PR body).
- [x] **No placeholders:** Every step contains the exact code/command/expected output.
- [x] **Type consistency:** All four `_validate.yml` jobs are referenced by name in `build`'s `needs:` (`[lint, test, coverage]`) and in the PR description's required-checks list (`validate / lint`, etc.). Caller jobs in `pr.yml` and `release.yml` both use `validate` as the job-id, matching the documented status-check names.
- [x] **TDD-equivalent for YAML:** Each task has YAML-parse validation and, where the change has runnable behavior (test job, coverage job, build job, pre-commit cleanup), a local verification step before commit.
