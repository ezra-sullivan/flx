# fx -> flx 迁移对照表

本文档按当前 `flx` 已实现代码整理，列出从 `fx` 迁移到 `flx` 时最常见的 API 对照关系。

`是否兼容` 列的含义：

- `是`：除导入路径、类型参数外，调用形状基本一致
- `部分`：可以机械迁移，但签名、返回值或约束有变化
- `否`：需要改名、改调用方式，或改上下文/控制流设计

## 包与构造

| 旧 API | 新 API | 是否兼容 | 示例 |
|---|---|---:|---|
| `import "fx/pkg/fx"` | `import "github.com/ezra-sullivan/flx"` | 否 | `import "github.com/ezra-sullivan/flx"` |
| `fx.Just(1, 2, 3)` | `flx.Values(1, 2, 3)` | 否 | `s := flx.Values(1, 2, 3)` |
| `fx.From(func(source chan<- any) { ... })` | `flx.From(func(source chan<- T) { ... })` | 部分 | `s := flx.From(func(out chan<- int) { out <- 1 })` |
| `fx.Range(ch)` | `flx.FromChan(ch)` | 否 | `s := flx.FromChan(ch)` |
| `fx.Concat(a, b, c)` | `flx.Concat(a, b, c)` | 部分 | `s := flx.Concat(s1, s2)` |
| `stream.Concat(b, c)` | `stream.Concat(b, c)` | 部分 | `s := s1.Concat(s2)` |

## 中间操作

### 同类型操作

| 旧 API | 新 API | 是否兼容 | 示例 |
|---|---|---:|---|
| `stream.Filter(fn, opts...)` | `stream.Filter(fn, opts...)` | 部分 | `s := s.Filter(func(v int) bool { return v%2 == 0 })` |
| `stream.Buffer(n)` | `stream.Buffer(n)` | 是 | `s := s.Buffer(128)` |
| `stream.Sort(less)` | `stream.Sort(less)` | 部分 | `s := s.Sort(func(a, b int) bool { return a < b })` |
| `stream.Reverse()` | `stream.Reverse()` | 部分 | `s := s.Reverse()` |
| `stream.Head(n)` | `stream.Head(n)` | 是 | `s := s.Head(10)` |
| `stream.Tail(n)` | `stream.Tail(n)` | 是 | `s := s.Tail(10)` |
| `stream.Skip(n)` | `stream.Skip(n)` | 是 | `s := s.Skip(5)` |

### 跨类型或重命名操作

| 旧 API | 新 API | 是否兼容 | 示例 |
|---|---|---:|---|
| `stream.Map(fn, opts...)` | `flx.Map(stream, fn, opts...)` | 否 | `out := flx.Map(in, func(v int) string { return strconv.Itoa(v) })` |
| `stream.MapErr(fn, opts...)` | `flx.MapErr(stream, fn, opts...)` | 否 | `out := flx.MapErr(in, parse)` |
| `stream.Walk(fn, opts...)` | `flx.FlatMap(stream, fn, opts...)` | 否 | `out := flx.FlatMap(in, func(v int, pipe chan<- int) { pipe <- v; pipe <- v * 10 })` |
| `stream.WalkErr(fn, opts...)` | `flx.FlatMapErr(stream, fn, opts...)` | 否 | `out := flx.FlatMapErr(in, worker)` |
| `stream.WalkCtx(fn, opts...)` | `flx.FlatMapContext(ctx, stream, fn, opts...)` | 否 | `out := flx.FlatMapContext(ctx, in, worker)` |
| `stream.WalkCtxErr(fn, opts...)` | `flx.FlatMapContextErr(ctx, stream, fn, opts...)` | 否 | `out := flx.FlatMapContextErr(ctx, in, worker)` |
| `stream.WalkCtxStrict(fn, opts...)` | `flx.FlatMapContext(ctx, stream, fn, flx.WithForcedDynamicWorkers(ctrl))` | 否 | `out := flx.FlatMapContext(ctx, in, worker, flx.WithForcedDynamicWorkers(ctrl))` |
| `stream.WalkCtxStrictErr(fn, opts...)` | `flx.FlatMapContextErr(ctx, stream, fn, flx.WithForcedDynamicWorkers(ctrl))` | 否 | `out := flx.FlatMapContextErr(ctx, in, worker, flx.WithForcedDynamicWorkers(ctrl))` |
| `stream.Distinct(fn)` | `flx.DistinctBy(stream, fn)` | 否 | `out := flx.DistinctBy(in, func(v User) string { return v.Email })` |
| `stream.Group(fn)` | `flx.GroupBy(stream, fn)` | 否 | `out := flx.GroupBy(in, func(v User) string { return v.Team })` |
| `stream.Split(n)` | `flx.Chunk(stream, n)` | 否 | `out := flx.Chunk(in, 100)` |
| `stream.Merge()` | `stream.Collect()` / `stream.CollectErr()` | 否 | `items := s.Collect()` |
| `stream.Reduce(fn)` | `flx.Reduce(stream, fn)` | 否 | `sum, err := flx.Reduce(s, reduce)` |

## 终结操作

### 基本保持一致

| 旧 API | 新 API | 是否兼容 | 示例 |
|---|---|---:|---|
| `stream.Done()` | `stream.Done()` | 是 | `s.Done()` |
| `stream.DoneErr()` | `stream.DoneErr()` | 是 | `err := s.DoneErr()` |
| `stream.ForEach(fn)` | `stream.ForEach(fn)` | 部分 | `s.ForEach(func(v int) { fmt.Println(v) })` |
| `stream.ForEachErr(fn)` | `stream.ForEachErr(fn)` | 部分 | `err := s.ForEachErr(func(v int) { fmt.Println(v) })` |
| `stream.ForAll(fn)` | `stream.ForAll(fn)` | 部分 | `s.ForAll(func(ch <-chan int) { ... })` |
| `stream.ForAllErr(fn)` | `stream.ForAllErr(fn)` | 部分 | `err := s.ForAllErr(func(ch <-chan int) { ... })` |
| `stream.Count()` | `stream.Count()` | 是 | `n := s.Count()` |
| `stream.CountErr()` | `stream.CountErr()` | 是 | `n, err := s.CountErr()` |
| `stream.AllMatch(fn)` | `stream.AllMatch(fn)` | 部分 | `ok := s.AllMatch(func(v int) bool { return v > 0 })` |
| `stream.AllMatchErr(fn)` | `stream.AllMatchErr(fn)` | 部分 | `ok, err := s.AllMatchErr(check)` |
| `stream.AnyMatch(fn)` | `stream.AnyMatch(fn)` | 部分 | `ok := s.AnyMatch(func(v int) bool { return v == 0 })` |
| `stream.AnyMatchErr(fn)` | `stream.AnyMatchErr(fn)` | 部分 | `ok, err := s.AnyMatchErr(check)` |
| `stream.NoneMatch(fn)` | `stream.NoneMatch(fn)` | 部分 | `ok := s.NoneMatch(func(v int) bool { return v < 0 })` |
| `stream.NoneMatchErr(fn)` | `stream.NoneMatchErr(fn)` | 部分 | `ok, err := s.NoneMatchErr(check)` |
| `stream.Parallel(fn, opts...)` | `stream.Parallel(fn, opts...)` | 部分 | `s.Parallel(func(v Job) { run(v) }, flx.WithWorkers(8))` |
| `stream.ParallelErr(fn, opts...)` | `stream.ParallelErr(fn, opts...)` | 部分 | `err := s.ParallelErr(process)` |
| `stream.Err()` | `stream.Err()` | 是 | `err := s.Err()` |

### 返回值已变化

| 旧 API | 新 API | 是否兼容 | 示例 |
|---|---|---:|---|
| `stream.First()` | `stream.First() (T, bool)` | 否 | `v, ok := s.First()` |
| `stream.FirstErr()` | `stream.FirstErr() (T, bool, error)` | 否 | `v, ok, err := s.FirstErr()` |
| `stream.Last()` | `stream.Last() (T, bool)` | 否 | `v, ok := s.Last()` |
| `stream.LastErr()` | `stream.LastErr() (T, bool, error)` | 否 | `v, ok, err := s.LastErr()` |
| `stream.Max(less)` | `stream.Max(less) (T, bool)` | 否 | `v, ok := s.Max(func(a, b int) bool { return a < b })` |
| `stream.MaxErr(less)` | `stream.MaxErr(less) (T, bool, error)` | 否 | `v, ok, err := s.MaxErr(less)` |
| `stream.Min(less)` | `stream.Min(less) (T, bool)` | 否 | `v, ok := s.Min(func(a, b int) bool { return a < b })` |
| `stream.MinErr(less)` | `stream.MinErr(less) (T, bool, error)` | 否 | `v, ok, err := s.MinErr(less)` |

行为补充：

- `First` / `AllMatch` / `AnyMatch` / `NoneMatch` 这些非 `*Err` 短路终结操作会同步 drain 上游，优先保证 fail-fast 错误可见性
- `FirstErr` / `AllMatchErr` / `AnyMatchErr` / `NoneMatchErr` 现在是真正短路：命中结果后立即返回，并在后台继续 drain 上游
- 这四个短路 `*Err` API 返回的是当前错误快照；返回后才记录的晚到 fail-fast error 不保证包含在返回值中
- 如果迁移后代码依赖“最终错误边界”，优先改用 `DoneErr` / `CollectErr`，或者用 `Head(1).CollectErr()` 显式保留首个元素

## 并发选项与上下文

| 旧 API | 新 API | 是否兼容 | 示例 |
|---|---|---:|---|
| `fx.WithWorkers(n)` | `flx.WithWorkers(n)` | 是 | `flx.WithWorkers(16)` |
| `fx.UnlimitedWorkers()` | `flx.WithUnlimitedWorkers()` | 否 | `flx.WithUnlimitedWorkers()` |
| `fx.WithDynamicWorkers(ctrl)` | `flx.WithDynamicWorkers(ctrl)` | 是 | `flx.WithDynamicWorkers(ctrl)` |
| `fx.WithDynamicWorkersCtx(ctrl)` | `flx.WithForcedDynamicWorkers(ctrl)` | 否 | `flx.WithForcedDynamicWorkers(ctrl)` |
| `fx.WithParentContext(ctx)` | 删除，改为显式 `ctx` 参数 | 否 | `flx.FlatMapContext(ctx, s, worker, ...)` |
| `fx.WithErrorStrategy(strategy)` | `flx.WithErrorStrategy(strategy)` | 是 | `flx.WithErrorStrategy(flx.ErrorStrategyCollect)` |

## 控制器与信号量

| 旧 API | 新 API | 是否兼容 | 示例 |
|---|---|---:|---|
| `fx.NewConcurrencyController(n)` | `flx.NewConcurrencyController(n)` | 是 | `ctrl := flx.NewConcurrencyController(8)` |
| `ctrl.SetWorkers(n)` | `ctrl.SetWorkers(n)` | 是 | `ctrl.SetWorkers(4)` |
| `ctrl.Workers()` | `ctrl.Workers()` | 是 | `n := ctrl.Workers()` |
| `ctrl.ActiveWorkers()` | `ctrl.ActiveWorkers()` | 是 | `n := ctrl.ActiveWorkers()` |
| `fx.NewDynamicSemaphore(n)` | `flx.NewDynamicSemaphore(n)` | 是 | `sem := flx.NewDynamicSemaphore(4)` |
| `sem.Acquire()` | `sem.Acquire()` | 是 | `sem.Acquire()` |
| `sem.AcquireCtx(ctx)` | `sem.AcquireCtx(ctx)` | 是 | `err := sem.AcquireCtx(ctx)` |
| `sem.Release()` | `sem.Release()` | 是 | `sem.Release()` |
| `sem.Resize(n)` | `sem.Resize(n)` | 是 | `sem.Resize(8)` |
| `sem.Cap()` | `sem.Cap()` | 是 | `cap := sem.Cap()` |
| `sem.Current()` | `sem.Current()` | 是 | `cur := sem.Current()` |
| `fx.ErrWorkerLimitReduced` | `flx.ErrWorkerLimitReduced` | 是 | `errors.Is(context.Cause(ctx), flx.ErrWorkerLimitReduced)` |

## 包级工具函数

| 旧 API | 新 API | 是否兼容 | 示例 |
|---|---|---:|---|
| `fx.Parallel(fns...)` | `flx.Parallel(fns...)` | 是 | `flx.Parallel(task1, task2)` |
| `fx.ParallelWithErrorStrategy(strategy, fns...)` | `flx.ParallelWithErrorStrategy(strategy, fns...)` | 是 | `err := flx.ParallelWithErrorStrategy(flx.ErrorStrategyCollect, task1, task2)` |
| `fx.ParallelErr(fns...)` | `flx.ParallelErr(fns...)` | 是 | `err := flx.ParallelErr(task1, task2)` |
| `fx.SendCtx(ctx, pipe, item)` | `flx.SendContext(ctx, pipe, item)` | 否 | `ok := flx.SendContext(ctx, pipe, v)` |

## retry / timeout

| 旧 API | 新 API | 是否兼容 | 示例 |
|---|---|---:|---|
| `fx.DoWithRetry(fn, opts...)` | `flx.DoWithRetry(fn, opts...)` | 是 | `err := flx.DoWithRetry(call, flx.WithRetry(3))` |
| `fx.DoWithRetryCtx(ctx, fn, opts...)` | `flx.DoWithRetryCtx(ctx, fn, opts...)` | 是 | `err := flx.DoWithRetryCtx(ctx, call, flx.WithAttemptTimeout(time.Second))` |
| `fx.WithRetry(n)` | `flx.WithRetry(n)` | 是 | `flx.WithRetry(5)` |
| `fx.WithInterval(d)` | `flx.WithInterval(d)` | 是 | `flx.WithInterval(time.Second)` |
| `fx.WithTimeout(d)` | `flx.WithTimeout(d)` | 是 | `flx.WithTimeout(30 * time.Second)` |
| `fx.WithAttemptTimeout(d)` | `flx.WithAttemptTimeout(d)` | 是 | `flx.WithAttemptTimeout(2 * time.Second)` |
| `fx.WithIgnoreErrors(errs)` | `flx.WithIgnoreErrors(errs)` | 是 | `flx.WithIgnoreErrors([]error{ErrNotFound})` |
| `fx.DoWithTimeout(fn, timeout, opts...)` | `flx.DoWithTimeout(fn, timeout, opts...)` | 是 | `err := flx.DoWithTimeout(call, 5*time.Second)` |
| `fx.DoWithTimeoutCtx(fn, timeout, opts...)` | `flx.DoWithTimeoutCtx(fn, timeout, opts...)` | 是 | `err := flx.DoWithTimeoutCtx(call, 5*time.Second)` |
| `fx.WithContext(ctx)` | `flx.WithContext(ctx)` | 是 | `flx.WithContext(ctx)` |
| `fx.DoOption` | 删除 | 否 | 直接使用 `flx.TimeoutOption` |

## 迁移建议

### 1. 先改导入和构造函数

- `fx/pkg/fx` -> `flx`
- `Just` -> `Values`
- `Range` -> `FromChan`

### 2. 再改所有跨类型链式操作

这一类需要从“方法调用”改成“包级函数包住原 stream”：

```go
// fx
out := fx.Just(1, 2, 3).Map(func(item any) any {
    return item.(int) * 10
})

// flx
out := flx.Map(flx.Values(1, 2, 3), func(v int) int {
    return v * 10
})
```

### 3. 最后处理 context 和空值返回

- `WalkCtx*` 全部迁移到 `MapContext*` / `FlatMapContext*`
- `First` / `Last` / `Max` / `Min` 这类调用点都要补 `ok` 判断

如果你已经开始使用 `WithInterruptibleWorkers`，可以直接保留；但新代码建议统一改成 `WithForcedDynamicWorkers`。

```go
v, ok := s.First()
if !ok {
    return
}
```

### 4. 如果你以前用 `value + error` 结构体在流里传递结果

`flx` 不提供官方 `Result[T]` 之类的公共类型，因为它默认把错误建模为 stream 状态。迁移时先区分两类需求：

- 如果你要的是“操作失败就进入统一错误边界”，优先改成 `MapErr` / `FlatMapErr`，并搭配 `CollectErr` / `DoneErr`
- 如果你要的是“每条记录都保留自己的成功/失败结果，最后统一收集”，继续使用自定义结构体，把错误当普通数据

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

这种模式适合批处理统计、脏数据归档、部分成功场景；它不替代 `flx` 自身的 stream 错误模型。
