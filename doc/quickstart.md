# flx Quick Start

本文档面向第一次接触 `flx` 的使用者，目标是用最短路径把一条 pipeline 跑起来，并理解最关键的几个概念。

如果你需要完整 API 说明、并发策略细节和边界行为，请继续看 [guide.md](./guide.md)。

## 1. 前置条件

- Go `1.26.1+`
- 当前示例默认模块导入路径为 `github.com/ezra-sullivan/flx`

## 2. 第一个例子

下面的例子完成三件事：

- 构造一个 `Stream[int]`
- 过滤偶数
- 把值乘以 `10`

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

## 3. 先记住这三个概念

### `Stream[T]` 是异步管道，不是惰性切片

`flx` 基于 channel 工作。`From`、`Concat` 和部分中间操作会在调用时启动 goroutine 或创建 channel。

这意味着：

- 不要把它当成“纯函数式惰性列表”
- 尽量总是用一个终结操作把流消费完

常见终结操作包括：

- `Done`
- `ForEach`
- `Collect`
- `Count`
- `First`

### 同类型操作是方法，跨类型操作是包级函数

```go
s := flx.Values(1, 2, 3).Filter(func(v int) bool { return v > 1 })
out := flx.Map(s, func(v int) string { return fmt.Sprintf("v=%d", v) })
```

这是 `flx` 最重要的 API 约定之一。

### 优先使用 `*Err` 变体处理业务错误

如果变换函数会失败，优先选择 `MapErr`、`FlatMapErr`、`CollectErr`、`DoneErr` 这些显式返回错误的 API。

```go
out := flx.MapErr(flx.Values("1", "2", "x"), strconv.Atoi)
items, err := out.CollectErr()
```

## 4. 常见起步写法

### 从固定值开始

```go
s := flx.Values("a", "b", "c")
```

### 从自定义生产函数开始

```go
s := flx.From(func(out chan<- int) {
	for i := range 5 {
		out <- i
	}
})
```

如果生产函数 panic，错误会进入 stream 状态；使用 `CollectErr` / `DoneErr` 可以显式拿到，使用默认终结操作则会按 fail-fast 语义 panic。

### 从已有 channel 开始

```go
ch := make(chan int)
go func() {
	defer close(ch)
	ch <- 1
	ch <- 2
	ch <- 3
}()

s := flx.FromChan(ch)
```

## 5. 最常见的终结方式

### 逐个消费

```go
flx.Map(flx.Values(1, 2, 3), func(v int) int {
	return v * 2
}).ForEach(func(v int) {
	fmt.Println(v)
})
```

### 收集成切片

```go
items := flx.Map(flx.Values(1, 2, 3), func(v int) int {
	return v * 2
}).Collect()
```

### 取第一个结果

```go
v, ok := flx.Map(flx.Values(1, 2, 3), func(v int) int {
	return v * 2
}).First()

if ok {
	fmt.Println(v)
}
```

注意：`First` / `Last` / `Max` / `Min` 这类 API 在空流时返回 `ok == false`。

## 6. 并发执行

`Map`、`FlatMap`、`Filter`、`Parallel` 这类操作都可以单独配置并发选项。

### 固定并发

```go
out := flx.Map(
	flx.Values(1, 2, 3, 4, 5),
	func(v int) int { return v * 10 },
	flx.WithWorkers(4),
)
```

### 无限制并发

```go
out := flx.Map(
	flx.Values(1, 2, 3),
	func(v int) int { return v * 10 },
	flx.WithUnlimitedWorkers(),
)
```

### 动态并发

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

## 7. 带 context 的变换

当你的 worker 需要响应取消、超时，或者要配合强制缩容时，用 `MapContext` / `FlatMapContext`。

```go
ctx := context.Background()

out := flx.MapContext(
	ctx,
	flx.Values(1, 2, 3),
	func(ctx context.Context, v int) int {
		select {
		case <-ctx.Done():
			return 0
		default:
			return v * 10
		}
	},
)

items := out.Collect()
_ = items
```

如果 worker 需要向下游发送结果，优先使用 `SendContext`：

```go
ctx := context.Background()

out := flx.FlatMapContext(
	ctx,
	flx.Values("a", "b"),
	func(ctx context.Context, v string, pipe chan<- string) {
		flx.SendContext(ctx, pipe, v+"!")
	},
)
```

`MapContext` / `MapContextErr` 的单结果发送已经内建取消感知；你自己在 `FlatMapContext*` worker 里发送多个值时，仍然优先使用 `SendContext`。

## 8. 强制缩容

如果你希望在并发数缩小时直接取消多余 worker：

- 使用 `MapContext` / `MapContextErr`
- 或使用 `FlatMapContext` / `FlatMapContextErr`
- 再配合 `WithForcedDynamicWorkers`

```go
ctx := context.Background()
ctrl := flx.NewConcurrencyController(4)

out := flx.MapContext(
	ctx,
	flx.Values(1, 2, 3, 4, 5),
	func(ctx context.Context, v int) int {
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(200 * time.Millisecond):
			return v * 10
		}
	},
	flx.WithForcedDynamicWorkers(ctrl),
)

ctrl.SetWorkers(1)
_ = out.Collect()
```

如果 worker 被缩容取消，可以通过 `context.Cause(ctx)` 判断是否为 `flx.ErrWorkerLimitReduced`。

## 9. 重试与超时

`flx` 不只有 stream，也提供独立的执行工具。

### 重试

```go
err := flx.DoWithRetry(func() error {
	return callRemote()
}, flx.WithRetry(3), flx.WithInterval(200*time.Millisecond))
```

### 协作式单次尝试超时

```go
err := flx.DoWithRetryCtx(
	context.Background(),
	func(ctx context.Context, attempt int) error {
		return callRemoteWithContext(ctx)
	},
	flx.WithRetry(3),
	flx.WithAttemptTimeout(time.Second),
)
```

### 总超时

```go
err := flx.DoWithTimeout(func() error {
	return callRemote()
}, 5*time.Second)
```

## 10. 下一步读什么

- 需要完整 API 说明和边界行为：看 [guide.md](./guide.md)
- 需要理解内部并发模型：看 [architecture.md](./architecture.md)
- 需要从 `fx` 迁移：看 [fx-to-flx-migration.md](./fx-to-flx-migration.md)
