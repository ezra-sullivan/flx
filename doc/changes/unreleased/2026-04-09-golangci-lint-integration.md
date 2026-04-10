# 2026-04-09 golangci-lint integration

## Goals

- Add static analysis coverage beyond `go vet ./...` for `flx`'s public API and concurrency-heavy runtime code.
- Keep the initial rollout low-noise so CI failures stay actionable.
- Use one pinned `golangci-lint` version in CI and local development.

## Decisions

- Introduce a root `.golangci.yml` using configuration `version: "2"`.
- Pin `golangci-lint` with `.golangci-lint-version` so CI does not drift across runner updates.
- Run linting in a dedicated CI job instead of mixing it into the existing test commands.
- Start with a small high-signal ruleset:
  - `errcheck`
  - `govet`
  - `ineffassign`
  - `staticcheck`
  - `unused`

## Notes

- The module targets Go `1.26.1`, so the lint version must support Go 1.26 syntax and type checking.
- Pin `golangci-lint` to `v2.11.4`; Go 1.26 support starts in the v2.9 line, so older versions are not acceptable for this repository.
- `gosimple` is not configured as a standalone linter in `golangci-lint` v2; the simplification checks come through `staticcheck`.
- Formatter enforcement stays with the existing `gofmt` CI step for now.

## Validation

- CI runs `golangci-lint` with the pinned version and repository config.
- Local verification uses the same config and should be run with a Go 1.26-compatible `golangci-lint` binary.
