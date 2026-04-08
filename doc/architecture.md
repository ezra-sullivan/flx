# flx 架构设计

## 1. 总览

`flx` 是一个以泛型 `Stream[T]` 为核心的流式处理库，保留 `fx` 已验证过的并发语义：

- 基于 channel 的异步管道
- 每个操作独立应用并发选项
- 支持固定并发、无限并发、动态并发
- 支持 fail-fast / collect / log-and-continue 三种错误策略

与 `fx` 的主要区别是：

- `flx` 是新模块，不追求 go-zero 风格兼容
- 跨类型操作改为包级泛型函数
- context 改为显式参数，不再通过 option 隐式注入
- 公开面按语义分层，而不是按实现文件平铺

## 2. 当前包结构

```text
flx/
├── doc.go
├── stream_api.go
├── transform_api.go
├── runtime_api.go
├── pipeline/
│   ├── control/
│   ├── coordinator/
│   └── observe/
└── internal/
    ├── collections/
    ├── config/
    ├── control/
    ├── coordinator/
    ├── link/
    ├── metrics/
    ├── resource/
    ├── runtime/
    ├── state/
    └── streaming/
```

原则：

- 根包 `flx` 负责主 DSL
- `flx/pipeline/control` 负责 worker/control plane 公开能力
- `flx/pipeline/coordinator` 负责 pipeline coordinator 公开能力
- `flx/pipeline/observe` 负责公开观测面
- `internal/...` 负责运行时、状态机和内部实现 owner
- 不再保留根包兼容桥接层

## 3. Ownership

### 3.1 根包 `flx`

根包长期保留：

- `Stream[T]`
- `Values`、`From`、`FromChan`、`Concat`
- `Map*`、`FlatMap*`
- `Stage*`、`Tap`、`Through*`
- `DistinctBy*`、`GroupBy*`、`Chunk`、`Reduce`
- `Parallel*`
- `DoWithRetry*`、`DoWithTimeout*`
- `SendContext`

### 3.2 `flx/pipeline/control`

`flx/pipeline/control` 负责：

- `Option`
- `WithWorkers`
- `WithUnlimitedWorkers`
- `WithDynamicWorkers`
- `WithForcedDynamicWorkers`
- `WithInterruptibleWorkers`
- `WithErrorStrategy`
- `ErrorStrategy`
- `ConcurrencyController`
- `DynamicSemaphore`
- `ErrWorkerLimitReduced` 等控制面错误

### 3.3 `flx/pipeline/coordinator`

`flx/pipeline/coordinator` 负责：

- `PipelineCoordinator`
- `PipelineCoordinatorPolicy`
- `CoordinatorOption`
- `StageBudget`
- `NewPipelineCoordinator`
- `WithCoordinator`
- `WithStageName`
- `WithStageBudget`
- `WithDecisionObserver`
- `WithResourceObserver`

### 3.4 `flx/pipeline/observe`

`flx/pipeline/observe` 负责：

- `StageMetrics`
- `LinkMetrics`
- `PipelineSnapshot`
- `PipelineSnapshot.Resources`
- `StageDecision`
- `PipelineDecision`
- `ResourceSnapshot`
- `PressureSample`
- `PressureLevel`
- `ResourceObserver`
- `WithStageMetricsObserver`
- `WithLinkMetricsObserver`

### 3.5 `internal/...`

默认进入 `internal/...` 的能力包括：

- stream runtime
- 错误状态聚合
- panic/cancel 归一化
- worker runtime
- 内部小型数据结构
- metrics、link、coordinator、resource 的真实实现

## 4. 后续扩展规则

后续如果开始做 pipeline coordinator，不应重新把实现堆回根目录。

推荐落点：

- `flx/pipeline/coordinator`
  - `PipelineCoordinator`
  - `PipelineCoordinatorPolicy`
  - `StageBudget`
  - `WithCoordinator`
  - `WithStageName`
  - `WithStageBudget`
- `flx/pipeline/observe`
  - `StageMetrics`、`LinkMetrics`、带 `Resources` 视图的 `PipelineSnapshot`、`PipelineDecision`、`ResourceSnapshot`
  - `WithStageMetricsObserver`、`WithLinkMetricsObserver`
- `internal/metrics`
- `internal/link`
- `internal/coordinator`
- `internal/resource`

而不是：

- `flx/coordinator`
- `flx/observe`
