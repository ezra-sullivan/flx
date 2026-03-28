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
- 下载大图并保存到 `examples/http_image_pipeline/output/original/`（默认 5 张）
- 把大图缩成小图并保存到 `examples/http_image_pipeline/output/processed/`
- 下载阶段会用 `flx.DoWithRetryCtx(...)` 自动重试，默认 3 次
- 默认只做下载和处理，不做上传；如需上传，在 `examples/http_image_pipeline/main.go` 里填 `UploadEndpoint`

两种实现都在仓库里：
- stage 函数组合版：`examples/http_image_pipeline/stage_pipeline.go`
- 原生 `flx` API 版：`examples/http_image_pipeline/native_pipeline.go`

完整说明见 [doc/examples/http-image-pipeline.md](./doc/examples/http-image-pipeline.md)。

## API 形状
`flx` 采用“同类型操作保留方法，跨类型操作使用包级泛型函数”的混合设计。

例如：

- 同类型操作：`Filter`、`Sort`、`Head`、`Skip`
- 跨类型操作：`flx.Map`、`flx.FlatMap`、`flx.MapContext`

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

- `WithWorkers`
- `WithUnlimitedWorkers`
- `WithDynamicWorkers`
- `WithForcedDynamicWorkers`
- `WithInterruptibleWorkers`（兼容别名）
- `WithErrorStrategy`
- `ErrorStrategyFailFast`
- `ErrorStrategyCollect`
- `ErrorStrategyLogAndContinue`

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

```go
ctrl := flx.NewConcurrencyController(4)

out := flx.Map(
	flx.Values(1, 2, 3, 4, 5),
	func(v int) int { return v * 10 },
	flx.WithDynamicWorkers(ctrl),
)

ctrl.SetWorkers(8)
ctrl.SetWorkers(2)

out.Done()
```

### 强制缩容

缩容时取消多余 worker，要求使用 `MapContext*` 或 `FlatMapContext*`：

```go
ctx := context.Background()
ctrl := flx.NewConcurrencyController(4)

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
	flx.WithForcedDynamicWorkers(ctrl),
)

ctrl.SetWorkers(1)
out.Done()
```

`WithInterruptibleWorkers` 仍然可用，但新代码建议统一写成 `WithForcedDynamicWorkers`。

`MapContext` / `MapContextErr` 的单结果发送现在也会响应 `ctx.Done()`；如果你在 `FlatMapContext*` 里自己向下游发送值，仍然建议显式使用 `SendContext`。

## 错误处理建议

默认错误策略是 `ErrorStrategyFailFast`。如果 worker 返回错误或 panic：

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

## 与 fx 的主要差异

- `fx.Just` -> `flx.Values`
- `fx.Range` -> `flx.FromChan`
- `stream.Map(...)` -> `flx.Map(stream, ...)`
- `stream.Walk(...)` -> `flx.FlatMap(stream, ...)`
- `stream.WalkCtx...` -> `flx.MapContext...` / `flx.FlatMapContext...`
- `stream.Merge()` -> `stream.Collect()` / `stream.CollectErr()`
- `fx.WithDynamicWorkersCtx` -> `flx.WithForcedDynamicWorkers`
- `fx.SendCtx` -> `flx.SendContext`
- `flx` 不提供官方 `Result[T]` / `ItemError[T]`；需要逐项结果时请自定义业务结构体

完整迁移表见 [doc/fx-to-flx-migration.md](./doc/fx-to-flx-migration.md)。

## 文档导航

- 变更记录：[CHANGELOG.md](./CHANGELOG.md)
- 未发布变更碎片：[doc/changes/unreleased/README.md](./doc/changes/unreleased/README.md)
- 正式版本说明：[doc/release-notes/README.md](./doc/release-notes/README.md)
- 非 release 标签说明：[doc/tag-notes/README.md](./doc/tag-notes/README.md)
- 快速上手：[doc/quickstart.md](./doc/quickstart.md)
- 详细用法：[doc/guide.md](./doc/guide.md)
- 实战示例：[doc/examples/http-image-pipeline.md](./doc/examples/http-image-pipeline.md)
- 架构设计：[doc/architecture.md](./doc/architecture.md)
- 项目规划：[doc/project-plan.md](./doc/project-plan.md)
- 迁移对照：[doc/fx-to-flx-migration.md](./doc/fx-to-flx-migration.md)



