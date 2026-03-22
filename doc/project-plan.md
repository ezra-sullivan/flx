# flx 项目规划

## 1. 项目背景

`fx` 已经完成了两个重要目标：

- 从 go-zero `core/fx` 中拆出独立可用的流式处理库
- 补齐动态并发控制、错误策略、重试和超时等能力

但它仍然保留了不少“为了兼容 go-zero `fx` 习惯用法”而存在的 API 形状：

- 使用 `any` 作为主数据模型，类型安全较弱
- 对外暴露 `pkg/fx` 这样的包层级
- 保留 `Range`、`Merge`、`DoOption`、`WalkCtx + WithParentContext` 这类偏兼容性的接口
- 某些上下文能力依赖 option 隐式拼装，而不是显式参数

`flx` 的目标不是在 `fx` 上继续叠加兼容层，而是把“泛型化”作为一次新项目重建：

- 保留 `fx` 的主要功能和并发语义
- 去掉兼容 go-zero API 而保留的包装
- 形成一个以 `Stream[T]` 为核心、以显式上下文和明确错误边界为原则的新模块

## 2. 目标

### 2.1 必做目标

- 提供泛型 `Stream[T]`
- 保留 channel 异步管道执行模型
- 保留固定并发、无限并发、动态并发控制
- 保留强制缩容语义，但改成显式 context API
- 保留错误策略、`Parallel`、`DoWithRetry`、`DoWithTimeout`
- 用新的 API 命名收敛历史兼容接口
- 保留现有关键测试语义，尤其是：
  - 动态并发扩缩容
  - 强制缩容取消
  - 上游 drain 防泄漏
  - retry/timeout 的协作式边界

### 2.2 体验目标

- 默认导入路径直接是 `github.com/ezra-sullivan/flx`
- 类型变化操作尽可能类型安全
- 空结果不再依赖 `nil` 表示，而是返回 `ok` 布尔值
- 上下文能力改为显式参数，不再通过 option 偷渡

## 3. 非目标

- 不追求与 `fx` 源码级兼容
- 不保留所有 go-zero 风格命名
- 当前阶段不引入 `Close()` 生命周期控制
- 当前阶段不做 benchmark 之前的性能特化
- 当前阶段不做额外 DSL、builder、反射封装

## 4. API 收敛原则

### 4.1 保留的核心能力

- `Stream` 管道模型
- `Filter`、排序、截断、短路判断等同类型操作
- `Map` / `FlatMap` / `Reduce` 等类型变化操作
- 动态并发控制器与可调信号量
- worker 错误策略
- `Parallel`
- `DoWithRetry`
- `DoWithTimeout`

### 4.2 明确移除或替换的兼容接口

| `fx` 接口 | `flx` 方案 | 原因 |
|---|---|---|
| `pkg/fx` | 根包 `flx` | 去掉为目录结构服务的包装层 |
| `Just` | `Values` | 命名更直观，避免沿用 go-zero 习惯名 |
| `Range` | `FromChan` | 语义更直接 |
| `Merge` | `Collect` / `CollectErr` | 用终结操作替代“把切片再包回 Stream”的兼容设计 |
| `Walk` | `FlatMap` | 明确表达 0:N 转换语义 |
| `WalkCtx` + `WalkCtxStrict` + `WithParentContext` | `MapContext` / `FlatMapContext`，显式传入 `context.Context` | 去掉隐式 context 和 fallback 设计 |
| `WithDynamicWorkersCtx` | `WithForcedDynamicWorkers` | 用“强制模式”对齐并发模型术语 |
| `SendCtx` | `SendContext` | 去掉兼容式缩写 |
| `DoOption` | 删除，仅保留 `TimeoutOption` | 去掉兼容别名 |

## 5. 核心设计决策

### 5.1 `Stream[T]` 保留，但跨类型操作改成包级函数

由于当前本地工具链是 `go1.26.1`，并且 Go 仍不支持“带额外类型参数的方法”，以下写法无法成立：

```go
func (s Stream[T]) Map[U any](fn func(T) U) Stream[U]
```

因此 `flx` 采用混合 API：

- 同类型操作保留为方法
  - `Filter`
  - `Buffer`
  - `Sort`
  - `Head`
  - `Tail`
  - `Skip`
  - `First`
  - `Last`
- 跨类型操作使用包级泛型函数
  - `Map`
  - `MapErr`
  - `FlatMap`
  - `FlatMapErr`
  - `MapContext`
  - `FlatMapContext`
  - `Reduce`
  - `GroupBy`
  - `DistinctBy`
  - `Chunk`

这是 Go 泛型能力约束下最清晰、最稳定的 API 形状。

### 5.2 显式 context，移除 fallback 语义

`fx` 中 `WalkCtx` 的复杂度主要来自这几个事实同时成立：

- context 不是参数，而是藏在 option 里
- `WalkCtx` 既想兼容普通模式，又想兼容强制动态模式
- 缺配置时既可能 fallback，也可能需要严格失败

`flx` 直接改成显式 API：

- `MapContext(ctx, stream, fn, opts...)`
- `FlatMapContext(ctx, stream, fn, opts...)`

这样：

- 父 context 来源明确
- 不再需要 `WithParentContext`
- 不再需要 `WalkCtxStrict`
- 强制缩容是否生效，只由是否配置 `WithForcedDynamicWorkers(ctrl)` 决定

### 5.3 终结操作改进空值语义

`fx` 中 `First()` / `Last()` 在空流时返回 `nil`，这是 `any` API 的遗留形状。

`flx` 改为：

- `First() (T, bool)`
- `Last() (T, bool)`
- `FirstErr() (T, bool, error)`
- `LastErr() (T, bool, error)`

`Max` / `Min` 也沿用这一模式。

## 6. 里程碑

### M1. 文档与模块骨架

- 创建 `flx/go.mod`
- 输出规划文档和架构文档
- 确认根包 API 命名和迁移边界

### M2. 泛型 Stream 核心

- `Stream[T]`
- `Values` / `From` / `FromChan`
- `Filter` / `Concat` / `Buffer` / `Sort` / `Reverse`
- `Head` / `Tail` / `Skip`
- `Count` / `First` / `Last` / `Collect`
- `AllMatch` / `AnyMatch` / `NoneMatch`
- `Max` / `Min`

### M3. 泛型变换与错误策略

- `Map` / `MapErr`
- `FlatMap` / `FlatMapErr`
- `MapContext` / `FlatMapContext`
- `DistinctBy` / `GroupBy` / `Chunk`
- `streamState` / `ErrorStrategy` / `WorkerError`

### M4. 动态并发与工具函数

- `DynamicSemaphore`
- `ConcurrencyController`
- `WithDynamicWorkers`
- `WithForcedDynamicWorkers`
- `SendContext`
- 包级 `Parallel` / `ParallelErr`

### M5. retry / timeout / 测试闭环

- `DoWithRetry`
- `DoWithRetryCtx`
- `DoWithTimeout`
- `DoWithTimeoutCtx`
- 移植并改写关键测试

## 7. 测试策略

### 7.1 直接迁移的测试主题

- `DynamicSemaphore` 获取、取消、扩缩容竞态
- `ConcurrencyController` 强制缩容取消策略
- 短路操作会 drain 上游，避免 goroutine 泄漏
- retry/timeout 的非法输入、ctx 传播、协作式超时

### 7.2 需要重写的测试主题

- `Map` / `FlatMap` / `GroupBy` 由 `any` 改成泛型后，断言要换成强类型
- `First` / `Last` / `Max` / `Min` 的空流判断从 `nil` 改为 `ok`
- context 相关测试要从 `WithParentContext` 迁移到显式 `ctx` 参数

## 8. 风险与应对

### 风险 1：泛型方法不能直接复刻旧链式 API

应对：

- 接受“同类型方法 + 跨类型函数”的混合设计
- 在文档中明确这是 Go 语言限制，不是实现偷懒

### 风险 2：context 语义改造时引入行为偏差

应对：

- 强制复用 `fx` 现有并发/取消测试思路
- 保留 `ErrWorkerLimitReduced` 语义和 `SendContext` 边界

### 风险 3：API 改名过多导致迁移成本上升

应对：

- 在架构文档中给出清晰的 `fx -> flx` 对照表
- 优先确保一一可映射，而不是追求完全一致命名

## 9. 交付标准

满足以下条件后，`flx` 首版视为可用：

- `flx` 可以作为根包直接导入
- 核心流式、动态并发、错误策略、retry/timeout 能编译并通过测试
- 文档明确说明执行模型、上下文边界和不兼容迁移点
- `flx` 不再依赖 `pkg/fx` 风格的兼容包装
