# Change Fragments

这个目录用于存放尚未发布的变更碎片。

约定：

- 日常开发时，把单个改动写成独立的 Markdown 文件，放到 `doc/changes/unreleased/`
- 正式 release 前，汇总这些碎片
- 汇总完成后，把完整说明写入 `doc/release-notes/`
- 只有正式 release 才进入根目录 `CHANGELOG.md`

推荐命名：

- `YYYY-MM-DD-short-topic.md`
- 例如：`2026-03-26-windowed-distinct-group.md`
