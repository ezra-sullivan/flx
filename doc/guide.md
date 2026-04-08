# flx Usage Guide

本文档按“概念 -> API 分类 -> 边界行为 -> 实战建议”的顺序整理 `flx` 的完整使用方式。

如果你只是第一次上手，建议先看 [quickstart.md](./quickstart.md)。

本文档中的完整导入路径默认使用 `github.com/ezra-sullivan/flx`，代码里的包名仍然是 `flx`。

## 1. 心智模型

`flx` 的核心是：

```go
type Stream[T any]
```

它代表一条基于 channel 的异步处理管道，而不是普通切片包装。

你可以把它理解为：

- 上游持续产出 `T`
- 中间操作持续消费上游并产出新的结果
- 终结操作负责收口

这带来两个直接影响：

- 某些中间操作在调用时就会创建 goroutine 或 channel
- 最好总是调用一个终结操作，把流显式消费完成

## 2. API 总体设计

`flx` 采用两类 API：

### 同类型操作保留为方法

这些操作不会引入新的类型参数：

- `Concat`
- `Filter`
- `Buffer`
- `Sort`
- `Reverse`
- `Head`
- `Tail`
- `Skip`
- 各类终结方法

### 跨类型操作使用包级泛型函数

这些操作会把 `Stream[T]` 变成 `Stream[U]`，因此写成包级函数：

- `Map`
- `MapErr`
- `FlatMap`
- `FlatMapErr`
- `MapContext`
- `MapContextErr`
- `FlatMapContext`
- `FlatMapContextErr`
- `Reduce`

这是为了适应 Go 1.26 仍然不支持额外类型参数方法的现实约束。

## 3. 构造 Stream

### `Values`

适合少量固定值：

```go
s := flx.Values(1, 2, 3)
```

### `From`

适合用代码主动生产元素：

```go
s := flx.From(func(out chan<- int) {
	for i := range 10 {
		out <- i
	}
})
```

约束：

- 你只负责写入，不需要关闭 channel
- `flx` 会在生成函数返回后关闭内部 channel
- 如果生产函数 panic，panic 会被记录到 stream 错误状态；`*Err` 终结操作会返回该错误，默认终结操作在 fail-fast 下会 panic

### `FromChan`

适合接管现有只读 channel：

```go
s := flx.FromChan(ch)
```

约束：

- 调用方必须保证上游最终关闭 channel

### `Concat`

把多个同类型 stream 合并到一个输出 stream：

```go
merged := flx.Concat(
	flx.Values(1, 2),
	flx.Values(3, 4),
)
```

也可以用方法形式：

```go
merged := flx.Values(1, 2).Concat(flx.Values(3, 4))
```

行为说明：

- 会合并上游错误状态，而不是丢掉它们
- 不同上游之间的输出相对顺序不保证
- 每个上游内部仍保持各自的输出顺序

## 4. 同类型中间操作

### `Filter`

只保留满足条件的元素：

```go
s := flx.Values(1, 2, 3, 4).Filter(func(v int) bool {
	return v%2 == 0
})
```

### `Buffer`

调整当前阶段输出 channel 的缓冲区大小：

```go
s := upstream.Buffer(128)
```

### `Sort`

把当前 stream 全量收集后排序，再重新输出：

```go
s := flx.Values(3, 1, 2).Sort(func(a, b int) bool {
	return a < b
})
```

注意：

- 这是一个全量操作，不是流式排序
- `less` 需要满足稳定且自洽的比较关系

### `Reverse`

全量收集后反转顺序：

```go
s := flx.Values(1, 2, 3).Reverse()
```

### `Head`

保留前 `n` 个元素：

```go
s := flx.Values(1, 2, 3, 4).Head(2)
```

约束：

- `n < 1` 会 panic

补充说明：

- `Head` 在拿到前 `n` 个元素后，仍会继续 drain 上游再关闭自己的输出
- 这样可以保证上游 goroutine 收敛，并让后续 `*Err` 终结操作看到晚到的 worker error

### `Tail`

保留最后 `n` 个元素：

```go
s := flx.Values(1, 2, 3, 4).Tail(2)
```

约束：

- `n < 1` 会 panic

### `Skip`

跳过前 `n` 个元素：

```go
s := flx.Values(1, 2, 3, 4).Skip(2)
```

约束：

- `n < 0` 会 panic

## 5. 跨类型变换

### `Map`

1:1 映射：

```go
out := flx.Map(flx.Values(1, 2, 3), func(v int) string {
	return strconv.Itoa(v)
})
```

### `MapErr`

1:1 映射，允许返回错误：

```go
out := flx.MapErr(flx.Values("1", "2", "x"), strconv.Atoi)
items, err := out.CollectErr()
```

### `FlatMap`

1:N 映射，由 worker 主动向下游写入：

```go
out := flx.FlatMap(flx.Values(1, 2, 3), func(v int, pipe chan<- int) {
	pipe <- v
	pipe <- v * 10
})
```

### `FlatMapErr`

和 `FlatMap` 一样，但允许返回错误：

```go
out := flx.FlatMapErr(in, func(v Item, pipe chan<- Result) error {
	if err := validate(v); err != nil {
		return err
	}

	pipe <- convert(v)
	return nil
})
```

### `MapContext` / `FlatMapContext`

在 worker 中显式使用 `context.Context`：

```go
ctx := context.Background()

out := flx.MapContext(ctx, flx.Values(1, 2, 3), func(ctx context.Context, v int) int {
	select {
	case <-ctx.Done():
		return 0
	default:
		return v * 10
	}
})
```

适用场景：

- worker 需要响应取消或超时
- 需要配合强制动态并发
- 向下游发送时需要 `SendContext`

### `MapContextErr` / `FlatMapContextErr`

在显式 context 基础上同时返回错误：

```go
out := flx.MapContextErr(ctx, in, func(ctx context.Context, v string) (int, error) {
	return strconv.Atoi(v)
})
```

### `DistinctBy`

按 key 去重，保留第一次出现的元素：

```go
out := flx.DistinctBy(users, func(u User) string {
	return u.Email
})
```

边界说明：

- `DistinctBy` 会维护当前流中所有已见 key
- 它适合有明确边界的批处理流
- 如果唯一 key 数持续增长，内存会随之持续增长
- 对无界流或超高基数流，不建议直接使用全局 `DistinctBy`

### `GroupBy`

按 key 分组，输出 `Stream[[]T]`：

```go
out := flx.GroupBy(users, func(u User) string {
	return u.Team
})
```

行为说明：

- 每组内部保持原始输入顺序
- 组与组之间按 key 第一次出现的顺序输出
- `GroupBy` 会在输出前消费并缓存整条输入流
- 它不适合超大数据集或无界流
- 如果你需要边界化内存的分组，优先考虑窗口化分组或自定义聚合逻辑

### `DistinctByCount` / `GroupByCount`

按输入数量做 tumbling window：

```go
out := flx.DistinctByCount(users, 1000, func(u User) string {
	return u.Email
})

groups := flx.GroupByCount(users, 1000, func(u User) string {
	return u.Team
})
```

行为说明：
- 每 `n` 个输入元素构成一个窗口
- `DistinctByCount` 只在窗口内去重；跨窗口相同 key 会重新出现
- `GroupByCount` 只在窗口内分组；输出类型是 `Stream[flx.Group[K, T]]`
- 按量窗口比全局 `DistinctBy` / `GroupBy` 更接近硬内存边界

### `DistinctByWindow` / `GroupByWindow`

按 processing-time 做 tumbling window：

```go
ctx := context.Background()

out := flx.DistinctByWindow(ctx, users, 5*time.Second, func(u User) string {
	return u.Email
})

groups := flx.GroupByWindow(ctx, users, 5*time.Second, func(u User) string {
	return u.Team
})
```

行为说明：
- 窗口在收到第一个元素时启动，空窗口不输出
- `DistinctByWindow` 只在当前时间窗口内去重
- `GroupByWindow` 会在窗口结束或上游关闭时输出当前窗口内的所有 group
- `ctx.Done()` 后会停止处理，并尽快回收上游
- 按时窗口提供的是软边界；如果你需要更稳定的内存上界，优先使用按量窗口

### `Chunk`

按固定大小分块：

```go
out := flx.Chunk(flx.Values(1, 2, 3, 4, 5), 2)
```

输出顺序等价于：

- `[1, 2]`
- `[3, 4]`
- `[5]`

约束：

- `n < 1` 会 panic

### `Reduce`

把整个 source channel 交给自定义归约函数：

```go
sum, err := flx.Reduce(flx.Values(1, 2, 3), func(ch <-chan int) (int, error) {
	var sum int
	for v := range ch {
		sum += v
	}
	return sum, nil
})
```

适合：

- 做自定义聚合
- 需要控制消费逻辑

注意：

- `Reduce` 返回的错误会和 stream 内部错误合并
- 如果 reducer 提前返回，`flx` 仍会自动 drain 剩余输入，保证上游 goroutine 能正常收敛

## 6. 终结与查询

### `Done` / `DoneErr`

仅消费完流，不关心结果：

```go
out.Done()
err := out.DoneErr()
```

### `ForEach` / `ForEachErr`

逐个消费：

```go
out.ForEach(func(v int) {
	fmt.Println(v)
})
```

### `ForAll` / `ForAllErr`

把底层 channel 暴露给自定义消费逻辑：

```go
out.ForAll(func(ch <-chan int) {
	for v := range ch {
		fmt.Println(v)
	}
})
```

### `Parallel` / `ParallelErr`

把当前 stream 当作任务源，并发执行副作用：

```go
err := flx.Values("a", "b", "c").ParallelErr(func(v string) error {
	return process(v)
}, control.WithWorkers(4))
```

### `Count` / `CountErr`

统计元素数量：

```go
n := out.Count()
n, err := out.CountErr()
```

### `Collect` / `CollectErr`

收集成切片：

```go
items := out.Collect()
items, err := out.CollectErr()
```

### `First` / `FirstErr`

获取第一个元素：

```go
v, ok := out.First()
v, ok, err := out.FirstErr()
```

空流时：

- `ok == false`

补充说明：

- `First` 仍然属于非 `*Err` 终结操作
- 如果上游已经记录了 fail-fast 错误，`First` 会像 `Done` / `Collect` 一样 panic
- `First` 在拿到首个元素后，仍会等待上游 drain 完成，再决定是否暴露晚到的 fail-fast 错误
- `FirstErr` 现在是真正短路：拿到首个元素后会立即返回，并在后台继续 drain 上游
- `FirstErr` 返回的是当前时刻的错误快照；返回后才发生的 fail-fast error 不保证包含在 `err` 中
- 如果你需要稳定的最终错误边界，优先使用 `CollectErr`，或者用 `Head(1).CollectErr()` 显式保留首个元素

### `Last` / `LastErr`

获取最后一个元素：

```go
v, ok := out.Last()
v, ok, err := out.LastErr()
```

### `AllMatch` / `AnyMatch` / `NoneMatch`

做布尔短路判断：

```go
all := out.AllMatch(func(v int) bool { return v > 0 })
any := out.AnyMatch(func(v int) bool { return v == 0 })
none := out.NoneMatch(func(v int) bool { return v < 0 })
```

如果你需要显式错误处理，优先使用：

- `AllMatchErr`
- `AnyMatchErr`
- `NoneMatchErr`

补充说明：

- 这些非 `*Err` 短路终结操作同样遵循 fail-fast 语义
- 如果上游已经记录 fail-fast 错误，它们不会静默吞错，而是会 panic
- 命中短路条件后，它们仍会等待上游 drain 完成，避免晚到的 fail-fast 错误丢失
- 对应的 `*Err` 版本现在是真正短路：命中条件后立即返回，并在后台 drain 上游
- 这些 `*Err` 版本返回的是当前时刻的错误快照；返回后才发生的 fail-fast error 不保证包含在 `err` 中
- 如果你需要稳定的最终错误边界，优先使用 `DoneErr` / `CollectErr` 等完整消费型终结操作

### `Max` / `Min`

按 `less` 规则选出最大/最小值：

```go
best, ok := out.Max(func(a, b int) bool { return a < b })
least, ok := out.Min(func(a, b int) bool { return a < b })
```

如果需要错误返回，使用：

- `MaxErr`
- `MinErr`

### `Err`

读取当前 stream 已记录的错误状态：

```go
err := out.Err()
```

更推荐的做法仍然是直接在终结点使用 `*Err`。

## 7. 并发选项

并发 `Option` 只作用于当前操作，不会自动向下游传播。

### `WithWorkers`

固定并发数：

```go
out := flx.Map(in, worker, control.WithWorkers(8))
```

说明：

- `workers <= 0` 会被内部提升到最小值 `1`

### `WithUnlimitedWorkers`

每个元素一个 worker：

```go
out := flx.Map(in, worker, control.WithUnlimitedWorkers())
```

适合：

- 任务量小
- 或单个任务极慢、且你明确接受高 goroutine 数

### `WithDynamicWorkers`

优雅动态并发，缩容时不打断已运行 worker：

```go
ctrl := control.NewConcurrencyController(4)
out := flx.Map(in, worker, control.WithDynamicWorkers(ctrl))
ctrl.SetWorkers(8)
ctrl.SetWorkers(2)
```

### `WithForcedDynamicWorkers`

强制动态并发，缩容时取消多余 worker：

```go
ctrl := control.NewConcurrencyController(4)
out := flx.MapContext(ctx, in, worker, control.WithForcedDynamicWorkers(ctrl))
```

重要约束：

- 只能用于 `MapContext*` / `FlatMapContext*`
- 如果用于非 context 变换，会 panic

### `WithInterruptibleWorkers`

兼容别名：

```go
out := flx.MapContext(ctx, in, worker, control.WithInterruptibleWorkers(ctrl))
```

新代码建议统一改成 `WithForcedDynamicWorkers`。

## 8. 错误模型

### 默认策略：`ErrorStrategyFailFast`

worker 返回错误或 panic 时：

- 记录错误
- 取消当前操作
- 在终结阶段暴露错误

### `ErrorStrategyCollect`

继续执行所有 worker，最后把错误聚合起来：

```go
out := flx.MapErr(
	in,
	worker,
	control.WithErrorStrategy(control.ErrorStrategyCollect),
)
```

### `ErrorStrategyLogAndContinue`

记录日志并继续，不把错误写入 stream 状态：

```go
out := flx.MapErr(
	in,
	worker,
	control.WithErrorStrategy(control.ErrorStrategyLogAndContinue),
)
```

### `WorkerError`

worker 显式返回错误或 panic 都会统一包装为 `WorkerError`：

```go
var workerErr *control.WorkerError

if errors.As(err, &workerErr) {
	...
}
```

### 实际建议

- 业务代码里，如果你需要稳定的最终错误边界，优先使用完整消费型 `*Err` API，例如 `DoneErr` / `CollectErr`
- `FirstErr` / `AllMatchErr` / `AnyMatchErr` / `NoneMatchErr` 现在是低延迟短路查询，返回的是当前错误快照而不是最终错误集合
- 只有你明确接受“出错即 panic”时，再使用非 `*Err` 终结操作
- 做批处理统计时，`ErrorStrategyCollect` 往往比 `fail-fast` 更合适
- `flx` 默认把错误建模为 stream 状态；如果你确实要把“每条数据自己的失败结果”继续往下游传，请自己定义业务结构体，并把它当普通数据处理

### item 级结果模式

如果你从 `fx` 迁移过来，之前习惯把 `error` 塞回每个 item，可以继续这样写，但要把它视为业务数据，而不是 `flx` 的官方错误模型：

```go
type ItemResult[T any] struct {
	Value T
	Err   error
}

out := flx.Map(flx.Values("1", "x", "3"), func(v string) ItemResult[int] {
	n, err := strconv.Atoi(v)
	return ItemResult[int]{Value: n, Err: err}
})

results := out.Collect()
```

适用场景：

- 你要保留每条记录的成功/失败结果，最后统一统计、落库或归档
- 你接受单条失败不终止流水线

不适用场景：

- 你希望 worker error、panic、timeout、context cancel 走统一的 stream 错误边界
- 你需要 `DoneErr` / `CollectErr` 暴露最终错误

## 9. context 与取消

### 显式传入 context

`flx` 不再通过 option 注入父 context，而是直接写成 API 参数：

```go
out := flx.MapContext(ctx, in, worker)
```

这样做的好处是：

- context 来源明确
- API 语义更稳定
- 不需要 `WithParentContext`

### `SendContext`

当 worker 向下游写入时，如果你希望在取消后尽快停止发送，使用：

```go
ok := flx.SendContext(ctx, pipe, item)
```

返回值：

- `true`：发送成功
- `false`：发送前已被取消

补充说明：

- `MapContext` / `MapContextErr` 的单结果发送已经内建取消感知
- 你自己在 `FlatMapContext*` 或其它自定义 worker 中向下游发送值时，仍然应该优先使用 `SendContext`

### 缩容取消原因

强制动态并发下，如果 worker 因缩容被取消，可以判断：

```go
if errors.Is(context.Cause(ctx), control.ErrWorkerLimitReduced) {
	...
}
```

## 10. 独立并发工具

除了 stream API，`flx` 还提供了包级并发执行工具。

### `Parallel`

并发执行多个无返回错误的函数，失败时 panic：

```go
flx.Parallel(task1, task2, task3)
```

### `ParallelWithErrorStrategy`

为并发执行设置错误策略：

```go
err := flx.ParallelWithErrorStrategy(
	control.ErrorStrategyCollect,
	task1,
	task2,
)
```

### `ParallelErr`

并发执行多个返回错误的函数，并聚合错误：

```go
err := flx.ParallelErr(task1, task2, task3)
```

## 11. retry

### `DoWithRetry`

对普通函数做重试：

```go
err := flx.DoWithRetry(
	call,
	flx.WithRetry(3),
	flx.WithInterval(200*time.Millisecond),
)
```

### `DoWithRetryCtx`

对显式接收 context 的函数做重试：

```go
err := flx.DoWithRetryCtx(
	context.Background(),
	func(ctx context.Context, attempt int) error {
		return callWithContext(ctx, attempt)
	},
	flx.WithRetry(5),
)
```

### Retry 选项

- `WithRetry(n)`：重试次数
- `WithInterval(d)`：每次重试间隔
- `WithTimeout(d)`：整个重试过程的总超时
- `WithAttemptTimeout(d)`：单次尝试超时，仅 `DoWithRetryCtx` 可用
- `WithIgnoreErrors(errs)`：命中这些错误时直接视为成功

注意：

- `WithAttemptTimeout` 用在 `DoWithRetry` 上会 panic
- 各种负时间或非法次数会 panic

## 12. timeout

### `DoWithTimeout`

给普通函数包一层超时：

```go
err := flx.DoWithTimeout(call, 5*time.Second)
```

### `DoWithTimeoutCtx`

给接收 context 的函数包一层超时：

```go
err := flx.DoWithTimeoutCtx(func(ctx context.Context) error {
	return callWithContext(ctx)
}, 5*time.Second)
```

### `WithContext`

给 timeout 指定父 context：

```go
err := flx.DoWithTimeoutCtx(
	func(ctx context.Context) error { return callWithContext(ctx) },
	5*time.Second,
	flx.WithContext(parent),
)
```

### 常见错误

- `ErrCanceled`
- `ErrTimeout`
- `ErrNilContext`
- `ErrNegativeTimeout`

## 13. 最佳实践

### 1. 默认优先稳定错误边界的终结操作

如果你需要最终错误结果，优先使用会完整消费 source 的终结操作，例如 `DoneErr` / `CollectErr`。

`FirstErr` / `AllMatchErr` / `AnyMatchErr` / `NoneMatchErr` 更适合低延迟短路查询，而不是拿最终错误集合。

### 2. worker 向下游写入时优先 `SendContext`

尤其是在 `FlatMapContext*` 和强制动态并发下。

`MapContext*` 的单结果发送已经内建取消感知，但自定义多次发送场景仍然应显式使用 `SendContext`。

### 3. `WithUnlimitedWorkers` 要谨慎

它很方便，但也最容易放大 goroutine 数量和内存压力。

### 4. 全量或累计内存操作要注意成本

以下操作会把当前流整体或显著累计到内存中：

- `Sort`
- `Reverse`
- `Collect`
- `DistinctBy`
- `GroupBy`

补充说明：

- `Tail(n)` 只保留最后 `n` 个元素
- `Chunk(n)` 只保留当前 chunk，不会一次性缓存整个流
- `DistinctBy` 的成本随唯一 key 数增长
- `GroupBy` 的成本随整条输入流增长
- `DistinctByCount` / `GroupByCount` 提供按量 tumbling window，更接近硬边界
- `DistinctByWindow` / `GroupByWindow` 提供按时 tumbling window，属于软边界

### 5. 需要稳定错误行为时，用 `DoneErr` / `CollectErr`

这样比依赖 panic 更适合线上服务和单元测试。

## 14. 常见迁移提醒

从 `fx` 迁移到 `flx` 时，最容易踩的几个点是：

- `Just` 改名为 `Values`
- `Range` 改名为 `FromChan`
- `stream.Map(...)` 改为 `flx.Map(stream, ...)`
- `stream.Walk(...)` 改为 `flx.FlatMap(stream, ...)`
- `WalkCtx*` 改为显式 `ctx` 的 `MapContext*` / `FlatMapContext*`
- `First` / `Last` / `Max` / `Min` 现在都返回 `ok`
- 如果你以前在流里传 `struct{ Value T; Err error }` 这类结果对象，`flx` 里继续自定义即可，但不要把它和 `MapErr` / `CollectErr` 视为同一层错误语义

如需从 `fx` 迁移，请结合 README 和 `quickstart.md` 中的 API 对照说明逐步替换。

## 15. 实战示例

如果你想看一条包含多个 stage、且每个 stage 使用不同 worker 策略的 HTTP pipeline，可以参考这个完整示例：

- [HTTP 图片处理流水线示例](./examples/http-image-pipeline.md)

这个示例展示了：

- HTTP 分页列出图片
- 固定 worker 下载
- 动态 worker 做缩放
- 固定 worker 加水印并上传
- 推荐的 stage 函数组合写法
- 对照的原生 `flx` API 逐段拼接写法
