# 2026-04-10 example default stage and coordinator extraction

- Changed `go run ./examples/imgproc_pipeline` to default to the plain `Stage` example path, with `native` as the only alternate mode in that business example.
- Moved the advanced control-plane teaching path into two dedicated examples: `examples/imgproc_observe` and `examples/imgproc_coordinator`.
- Updated README and example docs so `imgproc_pipeline`, `imgproc_observe`, and `imgproc_coordinator` each teach one focused path.
