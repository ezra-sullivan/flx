# Changelog

All notable changes to this project will be documented in this file.

This project follows Semantic Versioning. Release dates use the `YYYY-MM-DD` format.

## [Unreleased]

## [v0.1.1] - 2026-03-22

Documentation-only patch release.

### Added

- Root `CHANGELOG.md` documenting tagged releases.
- Changelog entry in `README.md` document navigation for easier discovery.

### Notes

- No API or runtime behavior changes in this release.
- `v0.1.0` remains the first feature release; `v0.1.1` only improves release documentation.

## [v0.1.0] - 2026-03-22

Initial public release of `github.com/ezra-sullivan/flx`.

### Added

- Generic `Stream[T]` as the core abstraction for stream processing.
- Stream constructors and same-type operations:
  `Values`, `From`, `FromChan`, `Concat`, `Filter`, `Buffer`, `Sort`, `Reverse`, `Head`, `Tail`, and `Skip`.
- Terminal and query operations:
  `Done`, `DoneErr`, `ForEach`, `ForEachErr`, `ForAll`, `ForAllErr`, `Parallel`, `ParallelErr`, `Count`, `CountErr`, `Collect`, `CollectErr`, `First`, `FirstErr`, `Last`, `LastErr`, `AllMatch`, `AnyMatch`, `NoneMatch`, `Max`, `Min`, and `Err`.
- Cross-type generic transforms:
  `Map`, `MapErr`, `FlatMap`, `FlatMapErr`, `MapContext`, `MapContextErr`, `FlatMapContext`, `FlatMapContextErr`, `DistinctBy`, `GroupBy`, `Chunk`, and `Reduce`.
- Dynamic concurrency primitives:
  `DynamicSemaphore`, `ConcurrencyController`, `WithWorkers`, `WithUnlimitedWorkers`, `WithDynamicWorkers`, and `WithForcedDynamicWorkers`.
- Compatibility alias `WithInterruptibleWorkers` for code migrating from earlier naming.
- Error handling controls:
  `ErrorStrategyFailFast`, `ErrorStrategyCollect`, `ErrorStrategyLogAndContinue`, and `WorkerError`.
- Standalone execution helpers:
  `Parallel`, `ParallelErr`, `ParallelWithErrorStrategy`, `DoWithRetry`, `DoWithRetryCtx`, `DoWithTimeout`, and `DoWithTimeoutCtx`.
- End-user documentation:
  `README.md`, quick start, usage guide, architecture notes, and migration guide.
- Release support files:
  `LICENSE`, `doc.go`, `.gitignore`, and GitHub Actions CI workflow.

### Changed

- The public API is exposed as a single package: `github.com/ezra-sullivan/flx`.
- Cross-type stream operations use package-level generic functions instead of methods, matching current Go generic method limits.
- Context is passed explicitly via `MapContext*` and `FlatMapContext*` instead of being injected through options.
- API names were simplified relative to `fx`, including:
  `Just -> Values`, `Range -> FromChan`, `Walk -> FlatMap`, and `SendCtx -> SendContext`.

### Notes

- This is the first tagged release and should be treated as an early stable baseline for further iteration.
- For migration details from `fx`, see [doc/fx-to-flx-migration.md](./doc/fx-to-flx-migration.md).
- The `v0.x` series may still refine API details before a `v1.0.0` stability commitment.
