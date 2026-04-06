# Git Workflow

本文档定义仓库采用的 `main-release-feature` 工作流。目标不是引入完整 GitFlow，而是在保持 `main` 可集成、可发布的前提下，为正式 release 冻结和已发布版本维护留出明确边界。

## 1. 分支角色

### `main`

- 默认集成分支。
- 日常开发最终都要回到这里。
- 目标状态是“尽量随时可以切一个正式版本”。
- 非正式集成 tag 也优先打在这里，便于标记某个可验证的里程碑。

### `feature/<topic>`

- 所有日常开发统一从 `main` 拉出。
- 功能、修复、重构、文档、测试都走这类短期分支。
- 分支名只表达主题，不再用 `fix/`、`docs/`、`chore/` 作为一级分支分类；这些信息保留在 commit message 里。

推荐命名：

- `feature/stage-family`
- `feature/retry-timeout-docs`
- `feature/windowed-group-fix`

约束：

- 一个 `feature/*` 分支只做一类逻辑改动。
- 生命周期尽量短；长时间不合并的工作要频繁同步 `main`。

### `release/<major.minor>`

- 只有在需要冻结正式版本或维护已发布版本线时才创建。
- 命名建议使用“版本线”而不是完整 patch 号，例如：
  - `release/v0.2`
  - `release/v1.4`

这个分支承担两类职责：

- 为新的正式版本做冻结、回归和收口。
- 为已经发布的旧版本线做补丁维护。

进入 `release/*` 后，默认不再接收新功能，只允许：

- release 阻塞级别的 bug fix
- 版本号和发布元数据调整
- release notes / changelog 收口
- 验证、打包、发布脚本修正

## 2. 日常开发流程

1. 从 `main` 拉出 `feature/<topic>`。
2. 在 `feature/*` 上完成开发和自测。
3. 如有用户可见变化，按仓库约定补 `doc/changes/unreleased/` 碎片。
4. 合并前同步最新 `main`，解决冲突后再跑最小必要验证。
5. 通过 `rebase + fast-forward`、`squash merge` 或 `rebase merge` 回到 `main`。

推荐验证基线：

- Go 代码默认至少 `go test ./...`
- 涉及 CGO 或本机环境不稳定的测试，使用远端测试主机补验证

## 3. 正式 Release 流程

当 `main` 上的功能集准备冻结为正式版本时：

1. 从 `main` 切出 `release/<major.minor>`。
2. 冻结新功能，只接受 release 相关修复和收口改动。
3. 补齐发布文档：
   - 汇总 `doc/changes/unreleased/`
   - 写入 `doc/release-notes/`
   - 更新根目录 `CHANGELOG.md` 摘要
4. 完成验证。
5. 将 `release/<major.minor>` 的最终发布提交回合到 `main`。
6. 对“当前发布线”从 `main` 打正式 tag，例如 `v0.2.0`。
7. 如果这条版本线后续还需要维护，保留 `release/<major.minor>`；否则删除。

这样做的原因很简单：

- `release/*` 给冻结期留出缓冲区，不阻塞 `main` 上后续开发准备。
- 正式发布结果最终仍然回到 `main`，避免主线和发布线长期漂移。

## 4. 已发布版本补丁流程

如果线上已经有 `v0.1.x`，但 `main` 已经进入 `v0.2` 开发：

1. 在 `release/v0.1` 上修补线上问题。
2. 在该 release 分支上完成验证。
3. 从 `release/v0.1` 打补丁 tag，例如 `v0.1.5`。
4. 把修复回合到 `main`，避免同一个问题在下一条版本线复发。

这里不要求补丁 tag 必须先落到 `main`，否则会把 `main` 上尚未准备发布的新功能混进旧版本补丁里。

## 5. Tag 规则

### 非正式 Tag

用于：

- 集成点冻结
- RC / 试运行
- 内部验证点

规则：

- 可以打在 `main` 或当前维护中的 `release/*` 分支
- 说明写入 `doc/tag-notes/`
- 不进入正式 release notes

### 正式 Release Tag

用于：

- 对外发布的稳定版本

规则：

- 当前发布线通常从 `main` 打 tag
- 历史维护线补丁允许从对应 `release/<major.minor>` 打 tag
- 版本说明写入 GitHub Release，必要时同步到 `doc/release-notes/`
- 根目录 `CHANGELOG.md` 只保留正式 release 摘要

## 6. 提交与合并约定

- commit message 继续使用：
  - `feat:`
  - `fix:`
  - `docs:`
  - `chore:`
  - `refactor:`
  - `test:`
- 合并 `main` 时优先保持线性历史。
- 不要把多个互不相关的主题塞进同一个 `feature/*`。
- 不要在 `release/*` 引入新的产品能力。

## 7. 什么时候创建 `release/*`

只有下面两种场景值得切：

- 你准备做一次正式版本冻结，需要让 `main` 和发布收口并行前进。
- 你已经有一个公开版本需要独立打补丁，而 `main` 已经在做下一条版本线。

如果只是想先把当前状态“冻一下”，但还不是正式 release，优先直接在 `main` 打非正式 tag 并补 `doc/tag-notes/`，不要为了一个集成点创建 `release/*`。
