# 2026-04-10 imgproc example log kind field

- Added stable `kind=` fields to the imgproc example family so coordinator snapshot / decision lines are easier to distinguish from ordinary item, retry, and summary logs.
- Split observe snapshot logs and coordinator snapshot / decision logs into separate example-local formatters.
- Updated formatter tests and example docs to reflect the new log layout.
