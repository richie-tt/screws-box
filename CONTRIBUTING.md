# Contributing to Screws Box

Thanks for considering a contribution. Screws Box is a small Go web app with a
short dev loop — run the commands below, open a PR against `master`, and CI takes
it from there.

## Prerequisites

- **Go 1.26.5+** (matches the `go` directive in [`go.mod`](./go.mod)).
- [`pre-commit`](https://pre-commit.com) for the format/lint hooks.
- Optional: Docker / Docker Compose to exercise the container path.

## Dev Loop

- `go build ./cmd/screwsbox` (or `make build`) — compile the server binary.
- `go test ./... -race` — unit tests with the race detector. CI runs
  `go test ./... -count=1 -race -shuffle=on`.
- `go test ./... -coverprofile=coverage.txt -covermode=atomic` — coverage profile
  (uploaded to Codecov in CI).
- `pre-commit run --all-files` — runs `gofmt`, `golangci-lint` (config in
  [`.golangci.yml`](./.golangci.yml)), `go mod tidy`, `hadolint`, and `actionlint`.
- `govulncheck ./...` — scans dependencies and the Go standard library for known
  advisories.

### Running locally

```bash
go build -o screws-box ./cmd/screwsbox
./screws-box                 # listens on :8080 (override with PORT)
```

On first launch you'll be prompted to create admin credentials. Configuration is
via environment variables — see [`.env.example`](./.env.example) and the
[README](./README.md#configuration) for the full list (`PORT`, `DB_PATH`,
`SESSION_TTL`, `REDIS_URL`, `OIDC_*`). Locked out of the admin account?
`./screws-box --disable-auth` clears the stored credentials.

Or run the container:

```bash
docker compose up -d         # add --profile redis for Redis-backed sessions
```

## Commits

This project uses [Conventional Commits 1.0](https://www.conventionalcommits.org/en/v1.0.0/).
[release-please](https://github.com/googleapis/release-please) reads the commit
history to prepare releases, so the type prefix matters:

- `feat:` → **minor** version bump (new capability)
- `fix:` → **patch** version bump (corrects a defect)
- `feat!:` or a `BREAKING CHANGE:` footer → **major** version bump
- `ci` / `chore` / `docs` / `refactor` / `test` / `build` / `style` / `perf` /
  `deps` → recorded in the changelog per
  [`release-please-config.json`](./release-please-config.json); `ci`, `chore`,
  `refactor`, `test`, `build`, and `style` never cut a release on their own.

Pick the type that honestly reflects what the change *is* — don't inflate or
downgrade it to manipulate release behavior.

Examples:

- `feat(server): add CSV export of the grid`
- `fix(store): prevent duplicate tags on import`
- `docs(readme): document REDIS_URL`
- `ci: pin actions to commit SHAs`

## Branch & PR Flow

1. Create a topic branch from `master` (e.g. `fix-import-dupe-tags`).
2. Push and open a Pull Request against `master`.
3. CI must be green — the required checks are `lint`, `test`, `coverage`, and
   `vulnerability` (the `build` job also runs) — and one approving review is
   required.
4. As `feat` / `fix` commits land on `master`, release-please opens or updates a
   **Release PR**; merging that Release PR cuts the tag, the GitHub Release, and
   the signed `ghcr.io/egeek-tech/screws-box` image with a build-provenance
   attestation.

## Code Style

- `gofmt`-clean (enforced by the `go-fmt` pre-commit hook).
- `golangci-lint run` clean — the enabled linters live in
  [`.golangci.yml`](./.golangci.yml).
- Tests use [`testify`](https://github.com/stretchr/testify) — `require` for
  preconditions, `assert` for verifications — with a `t.Run` subtest per case. No
  other assertion or test frameworks.
- New or changed exported symbols get doc comments.
- `govulncheck ./...` clean.

## Reporting Issues

- Bugs, features, and docs problems: use the
  [issue templates](https://github.com/egeek-tech/screws-box/issues/new/choose).
- Security vulnerabilities: **do not** open a public issue — follow
  [`SECURITY.md`](./SECURITY.md).
- Be respectful — this project follows the
  [Code of Conduct](./CODE_OF_CONDUCT.md).
