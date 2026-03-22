# flx

`flx` 是一个以泛型 `Stream[T]` 为核心的流式处理与动态并发控制库。

它基于 `fx` 的并发语义重建，但不再保留为了兼容 go-zero `fx` API 而存在的包装层。整体目标很直接：

- 用 `Stream[T]` 提供类型安全的流式处理
- 保留固定并发、无限并发、动态并发和强制缩容能力
- 用显式 `context.Context` 替代隐式 context option
- 收敛旧版兼容接口，让 API 更贴近现代 Go

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

## 错误处理建议

默认错误策略是 `ErrorStrategyFailFast`。如果 worker 返回错误或 panic：

- `fail-fast`：尽快取消当前操作，并在终结阶段暴露错误
- `collect`：继续执行，最后合并错误
- `log-and-continue`：记录日志并继续

业务代码里推荐优先使用 `*Err` 变体，这样错误边界最清楚：

```go
out := flx.MapErr(flx.Values("1", "x", "3"), strconv.Atoi)
items, err := out.CollectErr()
```

## 与 fx 的主要差异

- `fx.Just` -> `flx.Values`
- `fx.Range` -> `flx.FromChan`
- `stream.Map(...)` -> `flx.Map(stream, ...)`
- `stream.Walk(...)` -> `flx.FlatMap(stream, ...)`
- `stream.WalkCtx...` -> `flx.MapContext...` / `flx.FlatMapContext...`
- `stream.Merge()` -> `stream.Collect()` / `stream.CollectErr()`
- `fx.WithDynamicWorkersCtx` -> `flx.WithForcedDynamicWorkers`
- `fx.SendCtx` -> `flx.SendContext`

完整迁移表见 [doc/fx-to-flx-migration.md](./doc/fx-to-flx-migration.md)。

## 文档导航

- 快速上手：[doc/quickstart.md](./doc/quickstart.md)
- 详细用法：[doc/guide.md](./doc/guide.md)
- 架构设计：[doc/architecture.md](./doc/architecture.md)
- 项目规划：[doc/project-plan.md](./doc/project-plan.md)
- 迁移对照：[doc/fx-to-flx-migration.md](./doc/fx-to-flx-migration.md)
