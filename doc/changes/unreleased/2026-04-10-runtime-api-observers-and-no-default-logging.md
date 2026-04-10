# 2026-04-10 runtime api observers and no default logging

- Promoted `control.ErrorStrategyContinue` as the main public name for the non-recording error path, while keeping `ErrorStrategyLogAndContinue` as a deprecated compatibility alias.
- Removed built-in stdlib logging side effects from the public runtime APIs so `Parallel*`, retry, and timeout helpers no longer emit logs on behalf of callers.
- Added retry and timeout observer hooks through `flx.WithOnRetry(...)` / `flx.RetryEvent` and `flx.WithTimeoutLatePanicObserver(...)` / `flx.TimeoutLatePanicEvent`.
- Added tests that lock down the new no-default-logging contract and the observer callback behavior.
