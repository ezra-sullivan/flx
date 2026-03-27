# Tag Notes

这个目录用于记录需要补充说明、但不属于正式 release 的 Git tag。

适用场景：

- 内部里程碑 tag
- 验证性 tag
- 集成点 tag
- RC / 实验性 tag

约定：

- 不是每个 tag 都必须写 note
- 只有需要解释背景、范围或验证结果时才写
- 文件名默认使用 tag 名，例如 `v0.1.2.md`

正式 release 版本说明应写入 `doc/release-notes/`，并在根目录 `CHANGELOG.md` 中保留摘要。

当前仓库里已有的 `v0.1.0`、`v0.1.1`、`v0.1.2`、`v0.1.3` 都按 tag note 管理，而不是 formal release note。
