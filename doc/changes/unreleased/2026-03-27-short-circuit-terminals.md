# 2026-03-27 Short-Circuit Terminals

- `FirstErr` / `AllMatchErr` / `AnyMatchErr` / `NoneMatchErr` 改为真正短路：命中结果后立即返回，并在后台继续 drain 上游
- 这四个 `*Err` 短路 API 返回的是当前错误快照，不保证包含返回后才记录的晚到 fail-fast error
- `First` / `AllMatch` / `AnyMatch` / `NoneMatch` 继续优先保证 fail-fast 错误可见性，因此会同步 drain 上游
- `Head` 继续在关闭自身输出前先 drain 上游，避免下游过早完成时丢失晚到的 worker error
- 公开文档已同步更新 `README.md`、`doc/guide.md` 和 `doc/architecture.md`
