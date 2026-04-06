# 2026-04-06 stage family wrappers

- Added `Stage`, `StageErr`, `FlatStage`, `FlatStageErr`, and `Tap` as thin semantic wrappers over the existing `*Context` transforms.
- Added same-type `Stream[T]` chain sugar with `Through`, `ThroughErr`, and `Tap`.
- Updated the HTTP image pipeline example so the stage-oriented version now uses `flx.Stage(...)` at cross-type boundaries and `Through(...)` across same-type segments.
