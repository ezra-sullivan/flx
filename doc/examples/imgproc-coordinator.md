# 图片处理 Coordinator 示例

这个示例只讲 `pipeline/coordinator`，并且直接复用图片处理业务场景，不再单独维护
一套与业务无关的控制面 workload。

相关文件：

- 入口：[examples/imgproc_coordinator/main.go](../../examples/imgproc_coordinator/main.go)
- coordinator 主流程：[examples/imgproc_coordinator/coordinator_pipeline.go](../../examples/imgproc_coordinator/coordinator_pipeline.go)
- 外部 loop：[examples/imgproc_coordinator/control_loop.go](../../examples/imgproc_coordinator/control_loop.go)
- resource observer：[examples/imgproc_coordinator/resource.go](../../examples/imgproc_coordinator/resource.go)
- snapshot / decision 日志格式：[examples/imgproc_coordinator/logging.go](../../examples/imgproc_coordinator/logging.go)
- 共享业务层：[examples/internal/imgproc](../../examples/internal/imgproc)

## 运行方式

```powershell
go run ./examples/imgproc_coordinator
```

## 这个例子讲什么

这条 example 用同一条 “Picsum 下载 + 本地处理” 流水线展示 coordinator 接法：

- 所有 stage 都挂 `WithCoordinator(...)` + `WithStageName(...)`
- 只有 resize stage 配 `WithStageBudget(...)`
- coordinator 通过外部 ticker loop 周期性调用 `Snapshot()` / `Tick()`
- 决策通过 `WithDecisionObserver(...)` 打日志
- 资源样本通过 `WithResourceObserver(...)` 进入 snapshot 和决策

你会看到两类控制面日志：

```text
kind=coordinator_snapshot snapshot=tick stages=[...] links=[...] resources=[heap{ok ...}]
kind=coordinator_decision decision=[resize{workers=1->2 reason=incoming_link_backlog backlog=4/3}]
```

## 为什么这条 example 和 observe example 分开

拆开以后更容易读：

- 看 observe，只关注 stage / link snapshot
- 看 coordinator，只关注 snapshot 聚合、budget、decision 和 `Tick()` 节奏

业务流水线本身仍然是同一条 Picsum 图片处理链路，所以两者可以直接横向对照。

## 数据来源说明

coordinator example 与主业务 example 共用同一条 source / upload 组合：

- source 侧通过正常 HTTP client 访问 Picsum
- optional upload 侧仍可走 example 内置 target transport
- 业务语义依然是“先列图，再下载，再处理”

如果你想先单独理解 observe 的挂法，再回来补 coordinator，先看：

- [coordinator 与 observe 使用说明](../coordinator-observe.md)
- [图片处理 Observe 示例](./imgproc-observe.md)
