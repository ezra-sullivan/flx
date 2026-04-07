# flx 架构设计

## 1. 总览

`flx` 是一个以泛型 `Stream[T]` 为核心的流式处理库，保留 `fx` 已验证过的并发语义：

- 基于 channel 的异步管道
- 每个操作独立应用并发选项
- 支持固定并发、无限并发、动态并发
- 支持 fail-fast / collect / log-and-continue 三种错误策略

与 `fx` 的主要区别是：

- `flx` 是新模块，不追求 go-zero 风格兼容
- 泛型化后，跨类型操作改为包级泛型函数
- context 改为显式参数，不再通过 option 隐式注入
- 根包直接暴露能力，不再拆成 `pkg/fx`、`pkg/threading`、`pkg/errorx` 等对外层级

## 2. 包结构

```text
flx/
├── .github/
│   └── workflows/
│       └── ci.yml
├── LICENSE
├── doc.go
├── go.mod
├── README.md
├── doc/
│   ├── architecture.md
│   ├── quickstart.md
│   └── guide.md
├── stream_api.go
├── transform_api.go
├── control_api.go
├── runtime_api.go
└── internal/
    ├── collections/
    ├── config/
    ├── control/
    ├── runtime/
    ├── state/
    └── streaming/
```

原则：

- 对外只暴露一个包：`flx`
- 根包优先保持 thin facade，运行时与状态实现下沉到 `internal/*`
- 不再为“复刻 go-zero 内部目录”保留公开子包
- 根包文件按“公共 API 入口 / 少量独立工具”组织，复杂实现归内部 owner 包

## 3. 核心类型

### 3.1 `Stream[T]`

```go
type Stream[T any] struct {
    inner streaming.Stream[T]
}
```

职责：

- 作为根包公开 facade，承载稳定的链式 API
- 保持公开类型语义，例如 `Stream` 可比较
- 将 source、错误状态、运行时行为委托给 `internal/streaming`

### 3.2 `internal/state.Handle`

```go
type Handle interface {
    Add(err error, panicOnTerminal bool)
    Err() error
    ShouldPanic() bool
}
```

职责：

- 抽象单流状态与组合流状态
- 由 `internal/streaming` 持有并在组合操作中传播
- 为 `Concat` 等组合操作保留对上游状态的实时观察

### 3.3 `internal/state.streamState`

```go
type streamState struct {
    errs            BatchError
    panicOnTerminal atomic.Bool
}
```

职责：

- 收集 worker panic/error
- 记录 fail-fast 是否需要在终结操作触发 panic
- 作为单条流的本地错误状态容器

### 3.4 `internal/state.mergedStreamState`

```go
type mergedStreamState struct {
    local   *streamState
    parents []Handle
}
```

职责：

- 组合多个上游 `Stream` 的错误状态
- 让 `Concat` 可以保留上游错误的实时可见性
- 同时容纳当前阶段新增的本地错误

### 3.5 `ConcurrencyController`

```go
type ConcurrencyController struct {
    sem *DynamicSemaphore

    mu      sync.Mutex
    nextID  int
    cancels map[int]context.CancelCauseFunc
}
```

职责：

- 作为动态并发控制入口
- 优雅模式下只调整 `DynamicSemaphore`
- 可打断模式下在缩容时取消多余 worker

### 3.6 `DynamicSemaphore`

```go
type DynamicSemaphore struct {
    mu      sync.Mutex
    max     int
    cur     int
    waiters list.List
}
```

职责：

- 提供运行时可变容量的并发 slot 控制
- 用单锁 waiter 队列保证 `AcquireCtx` 不丢唤醒、不泄漏 slot

## 3.7 文件职责

- `stream_api.go`：`Stream[T]` facade、构造函数、同类型操作、终结操作
- `transform_api.go`：公开的跨类型变换 API、stage 风格 API、链式 `Through` / `Tap`
- `control_api.go`：动态并发控制、`Option`、错误策略、兼容别名
- `runtime_api.go`：`Parallel`、`Retry`、`Timeout` 等独立工具 API
- `internal/streaming`：流运行时与变换执行
- `internal/state`：错误状态与批量错误容器
- `internal/runtime`：goroutine 安全执行、context 归一化、walker runtime

## 4. API 设计

## 4.1 构造

```go
func Values[T any](items ...T) Stream[T]
func From[T any](generate func(chan<- T)) Stream[T]
func FromChan[T any](source <-chan T) Stream[T]
func Concat[T any](s Stream[T], others ...Stream[T]) Stream[T]
```

说明：

- `Values` 替代 `Just`
- `FromChan` 替代 `Range`
- `Concat` 保留包级函数，同时 `Stream[T]` 也保留同名方法

## 4.2 同类型方法

这些操作不引入新的类型参数，因此保留为方法：

```go
func (s Stream[T]) Concat(others ...Stream[T]) Stream[T]
func (s Stream[T]) Filter(fn func(T) bool, opts ...Option) Stream[T]
func (s Stream[T]) Buffer(n int) Stream[T]
func (s Stream[T]) Sort(less func(T, T) bool) Stream[T]
func (s Stream[T]) Reverse() Stream[T]
func (s Stream[T]) Head(n int64) Stream[T]
func (s Stream[T]) Tail(n int64) Stream[T]
func (s Stream[T]) Skip(n int64) Stream[T]
```

## 4.3 跨类型泛型函数

由于 Go 1.26 仍不支持额外类型参数的方法，这些操作必须是包级函数：

```go
func Map[T, U any](s Stream[T], fn func(T) U, opts ...Option) Stream[U]
func MapErr[T, U any](s Stream[T], fn func(T) (U, error), opts ...Option) Stream[U]

func FlatMap[T, U any](s Stream[T], fn func(T, chan<- U), opts ...Option) Stream[U]
func FlatMapErr[T, U any](s Stream[T], fn func(T, chan<- U) error, opts ...Option) Stream[U]

func MapContext[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T) U, opts ...Option) Stream[U]
func MapContextErr[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T) (U, error), opts ...Option) Stream[U]

func FlatMapContext[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T, chan<- U), opts ...Option) Stream[U]
func FlatMapContextErr[T, U any](ctx context.Context, s Stream[T], fn func(context.Context, T, chan<- U) error, opts ...Option) Stream[U]

func DistinctBy[T any, K comparable](s Stream[T], fn func(T) K) Stream[T]
func GroupBy[T any, K comparable](s Stream[T], fn func(T) K) Stream[[]T]
func Chunk[T any](s Stream[T], n int) Stream[[]T]

func Reduce[T, R any](s Stream[T], fn func(<-chan T) (R, error)) (R, error)
```

## 4.4 终结操作

```go
func (s Stream[T]) ForEach(fn func(T))
func (s Stream[T]) ForEachErr(fn func(T)) error
func (s Stream[T]) ForAll(fn func(<-chan T))
func (s Stream[T]) ForAllErr(fn func(<-chan T)) error
func (s Stream[T]) Parallel(fn func(T), opts ...Option)
func (s Stream[T]) ParallelErr(fn func(T) error, opts ...Option) error
func (s Stream[T]) Done()
func (s Stream[T]) DoneErr() error
func (s Stream[T]) Count() int
func (s Stream[T]) CountErr() (int, error)
func (s Stream[T]) Collect() []T
func (s Stream[T]) CollectErr() ([]T, error)
func (s Stream[T]) First() (T, bool)
func (s Stream[T]) FirstErr() (T, bool, error)
func (s Stream[T]) Last() (T, bool)
func (s Stream[T]) LastErr() (T, bool, error)
func (s Stream[T]) AllMatch(fn func(T) bool) bool
func (s Stream[T]) AllMatchErr(fn func(T) bool) (bool, error)
func (s Stream[T]) AnyMatch(fn func(T) bool) bool
func (s Stream[T]) AnyMatchErr(fn func(T) bool) (bool, error)
func (s Stream[T]) NoneMatch(fn func(T) bool) bool
func (s Stream[T]) NoneMatchErr(fn func(T) bool) (bool, error)
func (s Stream[T]) Max(less func(T, T) bool) (T, bool)
func (s Stream[T]) MaxErr(less func(T, T) bool) (T, bool, error)
func (s Stream[T]) Min(less func(T, T) bool) (T, bool)
func (s Stream[T]) MinErr(less func(T, T) bool) (T, bool, error)
func (s Stream[T]) Err() error
```

空流返回 `ok == false`，不再依赖 `nil`。

短路终结操作的语义约束：

- `First` / `AllMatch` / `AnyMatch` / `NoneMatch` 这类非 `*Err` 短路终结操作会同步 drain 上游，再决定是否以 fail-fast 方式暴露错误
- `FirstErr` / `AllMatchErr` / `AnyMatchErr` / `NoneMatchErr` 这类 `*Err` 短路终结操作优先保证低延迟返回：命中结果后立即返回当前错误快照，并在后台继续 drain 上游
- 如果调用方需要稳定的最终错误边界，应使用 `DoneErr` / `CollectErr` 等完整消费型终结操作，而不是依赖短路 `*Err` API
- `Head` 在拿到前 `n` 个元素后会先 drain 上游，再关闭自身输出，避免下游过早完成导致晚到的 worker error 不可见

## 5. 并发模型

## 5.1 执行模型

`flx` 仍然采用 `fx` 已验证过的执行模型：

- Stream 本质上是基于 channel 的异步管道
- `From`、`Concat` 和部分中间操作在调用时就会创建 channel 或 goroutine
- 终结操作负责消费并收口

这不是严格惰性求值模型。

## 5.2 Option 作用域

每个操作单独消费自己的 `Option`：

- `Map(..., WithWorkers(8))` 不会把 8 个 worker 传播给下游
- `WithDynamicWorkers(ctrl)` 只作用于当前那一段操作

## 5.3 并发模式

### 固定并发

- `WithWorkers(n)`
- 使用令牌池控制并发

### 无限制并发

- `WithUnlimitedWorkers()`
- 每个 item 一个 goroutine

### 动态并发，优雅模式

- `WithDynamicWorkers(ctrl)`
- 缩容时不打断已运行 worker
- `AcquireCtx` 受控制器容量变化影响

### 动态并发，可打断模式

- `WithForcedDynamicWorkers(ctrl)`
- 仅对 `MapContext*` / `FlatMapContext*` 生效
- 缩容时取消多余 worker 的派生 context

约束：

- 如果 `WithForcedDynamicWorkers` 用在非 context 操作上，直接 panic
- `context.Cause(ctx)` 在缩容取消时为 `ErrWorkerLimitReduced`

## 6. context 设计

## 6.1 显式传入

context 不再通过 option 传入，而是直接成为 API 参数：

```go
ctx := context.Background()
out := flx.MapContext(ctx, in, func(ctx context.Context, v Task) Result {
    return work(ctx, v)
}, flx.WithForcedDynamicWorkers(ctrl))
```

收益：

- 父 context 来源明确
- 不再需要 `WithParentContext`
- 不再需要 `WalkCtxStrict`

## 6.2 发送边界

仍保留可取消发送工具：

```go
func SendContext[T any](ctx context.Context, pipe chan<- T, item T) bool
```

用途：

- 当 worker 可能阻塞在下游发送时，允许 `ctx.Done()` 打断发送等待

## 7. 错误模型

### 7.1 错误来源

- worker 显式返回 error
- worker panic
- context 取消/超时

### 7.2 错误策略

```go
type ErrorStrategy uint8

const (
    ErrorStrategyFailFast ErrorStrategy = iota
    ErrorStrategyCollect
    ErrorStrategyLogAndContinue
)
```

### 7.3 终结行为

- `FailFast`
  - 尽快停止新任务调度
  - `Done()` / `ForEach()` / `Count()` 等默认终结操作会 panic
  - `DoneErr()` / `CountErr()` 等显式错误终结操作返回 error
- `Collect`
  - 收集错误但继续处理
- `LogAndContinue`
  - 仅日志输出，不中断流水线

### 7.4 item 级业务结果

`flx` 的主错误语义是“错误进入 stream 状态”，而不是给每个 item 官方附带一个 `error` 字段。

- 库本身不提供官方 `Result[T]` / `ItemError[T]` 公共类型，避免和 `MapErr` / `CollectErr` / `DoneErr` 形成双轨错误模型
- 如果调用方确实需要“逐项成功/失败都作为数据继续流转”，应自行定义领域结构体，并把它当普通 `Stream[T]` 数据处理
- worker panic、context cancel、timeout 等运行时失败仍属于 stream 错误，而不是 item 数据

## 8. retry / timeout

保留 `fx` 的主语义，但 API 只保留收敛后的版本：

```go
func DoWithRetry(fn func() error, opts ...RetryOption) error
func DoWithRetryCtx(ctx context.Context, fn func(context.Context, int) error, opts ...RetryOption) error

func DoWithTimeout(fn func() error, timeout time.Duration, opts ...TimeoutOption) error
func DoWithTimeoutCtx(fn func(context.Context) error, timeout time.Duration, opts ...TimeoutOption) error
```

设计原则：

- `DoWithRetry` / `DoWithTimeout` 约束的是等待和预算，不强制停止已启动任务
- 协作式取消统一由 `ctx.Done()` 驱动
- 不再保留 `DoOption` 兼容别名

## 9. 与 `fx` 的迁移关系

如需从 `fx` 迁移，请结合 README、`quickstart.md` 和 `guide.md` 中的 API 对照与语义说明逐步调整。

| `fx` | `flx` |
|---|---|
| `fx.Just(1, 2, 3)` | `flx.Values(1, 2, 3)` |
| `fx.Range(ch)` | `flx.FromChan(ch)` |
| `stream.Map(...)` | `flx.Map(stream, ...)` |
| `stream.MapErr(...)` | `flx.MapErr(stream, ...)` |
| `stream.Walk(...)` | `flx.FlatMap(stream, ...)` |
| `stream.WalkErr(...)` | `flx.FlatMapErr(stream, ...)` |
| `stream.WalkCtx(..., fx.WithParentContext(ctx), fx.WithDynamicWorkersCtx(ctrl))` | `flx.FlatMapContext(ctx, stream, ..., flx.WithForcedDynamicWorkers(ctrl))` |

`WithInterruptibleWorkers` 仍保留为兼容别名，但文档和新代码建议优先使用 `WithForcedDynamicWorkers`。
| `stream.Distinct(...)` | `flx.DistinctBy(stream, ...)` |
| `stream.Group(...)` | `flx.GroupBy(stream, ...)` |
| `stream.Split(n)` | `flx.Chunk(stream, n)` |
| `stream.Merge().First()` | `stream.Collect()` |
| `fx.SendCtx(...)` | `flx.SendContext(...)` |
| `fx.DoOption` | 删除 |

## 10. 实现顺序

1. 先实现 `streamState`、基础构造和基础终结操作
2. 再实现 `Map` / `FlatMap` 及错误策略
3. 再接入 `DynamicSemaphore` / `ConcurrencyController`
4. 最后实现 `Parallel`、retry、timeout 及测试迁移
