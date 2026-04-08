# flx

`flx` 是一个以泛型 `Stream[T]` 为核心的流式处理与动态并发控制库。

它基于 go-zero  的 `fx`  并发语义重建，但不兼容  `fx` API 。整体目标很直接：

- 用 `Stream[T]` 提供类型安全的流式处理
- 保留固定并发、无限并发、动态并发和强制缩容能力
- 用显式 `context.Context` 替代隐式 context option
- 让 API 更贴近现代 Go

## 适用场景

- 对一批数据做过滤、映射、分组、聚合
- 需要在 pipeline 中控制并发度
- 需要在运行中动态调整 worker 数量
- 需要在缩容时取消多余 worker
- 需要在流式处理之外复用 `retry` / `timeout` / `parallel` 工具

## 版本要求

- Go `1.26.1+`

当前模块路径是 `github.com/ezra-sullivan/flx`，示例代码直接使用：

```go
import "github.com/ezra-sullivan/flx"
```

worker 并发控制相关 API 现在推荐从下面的公开子包导入：

```go
import "github.com/ezra-sullivan/flx/pipeline/control"
```

pipeline coordinator 与观测 API 推荐从下面两个公开子包导入：

```go
import "github.com/ezra-sullivan/flx/pipeline/coordinator"
import "github.com/ezra-sullivan/flx/pipeline/observe"
```

如果后续发布到新的模块路径，示例中的导入路径需要同步替换。

## 快速示例

```go
package main

import (
	"fmt"

	"github.com/ezra-sullivan/flx"
)

func main() {
	out := flx.Map(
		flx.Values(1, 2, 3, 4, 5).
			Filter(func(v int) bool { return v%2 == 0 }),
		func(v int) int { return v * 10 },
	)

	out.ForEach(func(v int) {
		fmt.Println(v)
	})
}
```

输出：

```text
20
40
```

## 实战示例

仓库里还带了一个可直接运行的 HTTP 图片处理 example：

```powershell
go run ./examples/http_image_pipeline
```

这个示例会：
- 从 `https://picsum.photos/v2/list` 分页拉取图片
- 下载大图并保存到 `examples/http_image_pipeline/.output/original/`（默认 5 张）
- 把大图缩成小图并保存到 `examples/http_image_pipeline/.output/processed/`
- 下载阶段会用 `flx.DoWithRetryCtx(...)` 自动重试，默认 3 次
- `coordinator` 模式会额外输出 stage/link/resource 三视图 snapshot，并周期性执行 `Tick()`
- 默认只做下载和处理，不做上传；如需上传，在 `examples/http_image_pipeline/main.go` 里填 `UploadEndpoint`

三种实现都在仓库里：
- coordinator 版：`examples/http_image_pipeline/coordinator_pipeline.go`
- `Stage` 组合版：`examples/http_image_pipeline/stage_pipeline.go`
- 原生 `flx` API 版：`examples/http_image_pipeline/native_pipeline.go`

可显式指定模式：

```powershell
go run ./examples/http_image_pipeline -mode coordinator
go run ./examples/http_image_pipeline -mode stage
go run ./examples/http_image_pipeline -mode native
```

完整说明见 [doc/examples/http-image-pipeline.md](./doc/examples/http-image-pipeline.md)。

## API 形状
`flx` 采用“同类型操作保留方法，跨类型操作使用包级泛型函数”的混合设计。

例如：

- 同类型操作：`Filter`、`Sort`、`Head`、`Skip`
- 跨类型操作：`flx.Map`、`flx.FlatMap`、`flx.MapContext`
- stage 语义封装：`flx.Stage`、`flx.StageErr`、`flx.FlatStage`、`flx.FlatStageErr`、`flx.Tap`

如果一段 pipeline 在若干 stage 之间保持同一个 `Stream[T]` 类型，可以继续用方法链把这段写得更像“流经多个 stage”，例如 `flx.Stage(...).Through(...).Through(...)`。

```go
s := flx.Values(1, 2, 3).Filter(func(v int) bool { return v > 1 })
out := flx.Map(s, func(v int) string { return fmt.Sprintf("v=%d", v) })
```

这样设计不是为了追求风格特别，而是因为 Go 1.26 仍然不支持“额外类型参数的方法”。

## 核心能力

### Stream 构造与中间操作

- `Values` / `From` / `FromChan` / `Concat`
- `Filter` / `Buffer` / `Sort` / `Reverse`
- `Head` / `Tail` / `Skip`
- `Map` / `MapErr`
- `FlatMap` / `FlatMapErr`
- `MapContext` / `FlatMapContext`
- `Stage` / `StageErr` / `FlatStage` / `FlatStageErr` / `Tap`
- `DistinctBy` / `GroupBy` / `Chunk`
- `DistinctByCount` / `GroupByCount`
- `DistinctByWindow` / `GroupByWindow`
- `Reduce`

### 终结与查询

- `Done` / `DoneErr`
- `ForEach` / `ForEachErr`
- `Parallel` / `ParallelErr`
- `Count` / `Collect`
- `First` / `Last`
- `AllMatch` / `AnyMatch` / `NoneMatch`
- `Max` / `Min`
- `Err`

### 并发与错误控制

- `control.WithWorkers`
- `control.WithUnlimitedWorkers`
- `control.WithDynamicWorkers`
- `control.WithForcedDynamicWorkers`
- `control.WithInterruptibleWorkers`（兼容别名）
- `control.WithErrorStrategy`
- `control.ErrorStrategyFailFast`
- `control.ErrorStrategyCollect`
- `control.ErrorStrategyLogAndContinue`

### 独立工具函数

- `Parallel` / `ParallelErr` / `ParallelWithErrorStrategy`
- `DoWithRetry` / `DoWithRetryCtx`
- `DoWithTimeout` / `DoWithTimeoutCtx`

## 内存边界

- `DistinctBy` 会维护当前流的全局已见 key 集合；如果唯一 key 持续增长，内存也会持续增长。
- `GroupBy` 会先缓存整条输入流，再按 key 输出分组；它不适合超大数据集或无界流。
- 这两个 API 更适合有明确边界的批处理数据。
- `DistinctByCount` / `GroupByCount` 采用按输入数量切分的 tumbling window，适合需要更硬内存边界的去重/分组场景。
- `DistinctByWindow` / `GroupByWindow` 采用 processing-time tumbling window，并显式接受 `context.Context`；它们更适合做按时归档或周期性输出。

## 动态并发

### 优雅缩容

缩容时不打断已运行 worker，只影响后续竞争 slot 的 worker：

需要额外导入：`github.com/ezra-sullivan/flx/pipeline/control`

```go
ctrl := control.NewConcurrencyController(4)

out := flx.Map(
	flx.Values(1, 2, 3, 4, 5),
	func(v int) int { return v * 10 },
	control.WithDynamicWorkers(ctrl),
)

ctrl.SetWorkers(8)
ctrl.SetWorkers(2)

out.Done()
```

### 强制缩容

缩容时取消多余 worker，要求使用 `MapContext*` 或 `FlatMapContext*`：

需要额外导入：`github.com/ezra-sullivan/flx/pipeline/control`

```go
ctx := context.Background()
ctrl := control.NewConcurrencyController(4)

out := flx.FlatMapContext(
	ctx,
	flx.Values("a", "b", "c"),
	func(ctx context.Context, v string, pipe chan<- string) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		flx.SendContext(ctx, pipe, v+"!")
	},
	control.WithForcedDynamicWorkers(ctrl),
)

ctrl.SetWorkers(1)
out.Done()
```

`control.WithInterruptibleWorkers` 仍然可用，但新代码建议统一写成 `control.WithForcedDynamicWorkers`。

`MapContext` / `MapContextErr` 的单结果发送现在也会响应 `ctx.Done()`；如果你在 `FlatMapContext*` 里自己向下游发送值，仍然建议显式使用 `SendContext`。

## Pipeline Coordinator

`pipeline/coordinator` 用来把动态 worker 从“手动 `SetWorkers(...)`”提升到“基于 stage/link/resource 信号的显式 `Tick()` 决策”。

最小用法通常是：

- 给动态 stage 挂 `coordinator.WithCoordinator(...)`
- 给 stage 命名 `coordinator.WithStageName(...)`
- 给可调 stage 配预算 `coordinator.WithStageBudget(...)`
- 在外部控制循环里周期性调用 `PipelineCoordinator.Snapshot()` 和 `PipelineCoordinator.Tick()`

`PipelineCoordinatorPolicy` 当前几个核心字段：

- `ScaleUpStep`
  每次扩容最多增加多少 worker。
- `ScaleDownStep`
  每次缩容最多减少多少 worker。
- `ScaleUpCooldown`
  同一 stage 两次扩容之间的最小时间间隔。
- `ScaleDownCooldown`
  同一 stage 两次缩容之间的最小时间间隔。
- `ScaleUpHysteresis`
  扩容信号需要连续满足多少个 `Tick()` 才会真正扩容。
- `ScaleDownHysteresis`
  缩容信号需要连续满足多少个 `Tick()` 才会真正缩容。

默认语义是保守兼容的：

- `ScaleUpStep` / `ScaleDownStep` 零值按 `1`
- `ScaleUpCooldown` / `ScaleDownCooldown` 零值表示不额外等待
- `ScaleUpHysteresis` / `ScaleDownHysteresis` 零值按 `1`

当前 `Tick()` 的策略重点：

- 下游 `incoming link backlog` 优先触发下游扩容
- stage 自身 backlog 次之
- `warning` 级资源压力会 brake scale-up
- `critical` 级资源压力会对空闲 stage 施加 shrink bias
- `budget_min` 属于硬纠偏，不受 cooldown / hysteresis 限制

示例：

```go
pipelineCoordinator := coordinator.NewPipelineCoordinator(
	coordinator.PipelineCoordinatorPolicy{
		ScaleUpStep:         1,
		ScaleDownStep:       1,
		ScaleUpCooldown:     500 * time.Millisecond,
		ScaleDownCooldown:   750 * time.Millisecond,
		ScaleUpHysteresis:   1,
		ScaleDownHysteresis: 2,
	},
	coordinator.WithResourceObserver(myResourceObserver),
)
```

完整接法可直接看 [doc/examples/http-image-pipeline.md](./doc/examples/http-image-pipeline.md) 里的 coordinator 版示例。

## 错误处理建议

默认错误策略是 `control.ErrorStrategyFailFast`。如果 worker 返回错误或 panic：

- `fail-fast`：尽快取消当前操作，并在终结阶段暴露错误
- `collect`：继续执行，最后合并错误
- `log-and-continue`：记录日志并继续

业务代码里，如果你需要稳定的最终错误边界，优先使用会完整消费 source 的 `*Err` 终结操作，例如 `DoneErr` / `CollectErr`：

```go
out := flx.MapErr(flx.Values("1", "x", "3"), strconv.Atoi)
items, err := out.CollectErr()
```

补充说明：

- `From` 的生产函数如果 panic，会进入 stream 错误状态
- `DoneErr` / `CollectErr` 等完整消费型 `*Err` 终结操作可以显式拿到这类错误
- `FirstErr` / `AllMatchErr` / `AnyMatchErr` / `NoneMatchErr` 现在是真正短路：命中结果后立即返回，并在后台 drain 上游
- 这四个短路 `*Err` API 返回的是当前错误快照；返回后才发生的 fail-fast error 不保证包含在返回值里
- `First` / `AllMatch` / `AnyMatch` / `NoneMatch` 这类短路终结操作也遵循 fail-fast 语义；如果上游已经记录 fail-fast 错误，它们会像其他非 `*Err` 终结操作一样 panic

`flx` 默认把错误建模为 stream 状态，而不是官方提供一个 `value + error` 的 item 容器。
如果你从 `fx` 迁移过来，之前会在流里传 `struct{ Value T; Err error }` 这类结果对象，这种写法在 `flx` 里仍然可以保留，但它应该被视为业务数据建模：适合“部分成功、最后统一收集失败项”的场景，而不是替代 `MapErr` / `CollectErr` / `DoneErr` 这条主错误通道。

## 与 go-zero fx 的主要差异

- `fx.Just` -> `flx.Values`
- `fx.Range` -> `flx.FromChan`
- `stream.Map(...)` -> `flx.Map(stream, ...)`
- `stream.Walk(...)` -> `flx.FlatMap(stream, ...)`
- `stream.WalkCtx...` -> `flx.MapContext...` / `flx.FlatMapContext...`
- `stream.Merge()` -> `stream.Collect()` / `stream.CollectErr()`
- `fx.WithDynamicWorkersCtx` -> `control.WithForcedDynamicWorkers`
- `fx.SendCtx` -> `flx.SendContext`
- `flx` 不提供官方 `Result[T]` / `ItemError[T]`；需要逐项结果时请自定义业务结构体

如果你从 `fx` 迁移，建议结合本 README、`doc/quickstart.md` 和 `doc/guide.md` 中的 API 对照与语义说明逐步调整。

## 文档导航

- 变更记录：[CHANGELOG.md](./CHANGELOG.md)
- 未发布变更碎片：[doc/changes/unreleased/README.md](./doc/changes/unreleased/README.md)
- 正式版本说明：[doc/release-notes/README.md](./doc/release-notes/README.md)
- 非 release 标签说明：[doc/tag-notes/README.md](./doc/tag-notes/README.md)
- 快速上手：[doc/quickstart.md](./doc/quickstart.md)
- 详细用法：[doc/guide.md](./doc/guide.md)
- 实战示例：[doc/examples/http-image-pipeline.md](./doc/examples/http-image-pipeline.md)
- 架构设计：[doc/architecture.md](./doc/architecture.md)
- 仓库工作流：[doc/git-workflow.md](./doc/git-workflow.md)



