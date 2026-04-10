# 2026-04-10 coordinator observe example restructure

- Added two focused control-plane examples: `examples/imgproc_observe` and `examples/imgproc_coordinator`.
- Aligned the business example naming as `examples/imgproc_pipeline` so the full example family shares one naming line.
- Extracted the shared business flow into `examples/internal/imgproc`, including the Picsum list/download integration, the local upload transport, stage logic, recorder, control loop, resource observer, and logging helpers.
