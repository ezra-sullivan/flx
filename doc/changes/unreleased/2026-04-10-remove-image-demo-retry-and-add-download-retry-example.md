# 2026-04-10 remove image demo retry and add download retry example

- Removed demo-injected download failures from the main imgproc example family so the normal pipeline / observe / coordinator paths no longer simulate retry warnings.
- Added `examples/imgproc_retry` as a deterministic local retry example that demonstrates both success-after-retry and fail-after-3-attempts cases on top of a dedicated local retry source.
- Updated README and example docs to point retry-focused readers to the dedicated retry example.
