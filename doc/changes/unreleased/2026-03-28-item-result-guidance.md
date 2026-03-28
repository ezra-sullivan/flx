# 2026-03-28 Item-Result Guidance

- 文档明确 `flx` 的主错误语义是 stream state，而不是官方 `value + error` item 容器
- 从 `fx` 迁移时，如果业务确实需要“逐项结果 + 最后统一收集失败项”，继续使用用户自定义结构体，把它当普通数据
- `README.md`、`doc/guide.md`、`doc/architecture.md` 和 `doc/fx-to-flx-migration.md` 已同步补充这一边界
