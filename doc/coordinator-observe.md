# coordinator 与 observe 使用说明

`pipeline/observe` 和 `pipeline/coordinator` 经常一起出现，但它们解决的不是同一件事：

- `observe` 解决的是“怎么看到 pipeline 现在长什么样”
- `coordinator` 解决的是“根据这些观测结果，要不要调 worker”

把两者拆开理解，会更容易判断当前缺的是“观测”还是“调度”。

## 1. 什么时候只用 `observe`

如果你只是想看清 pipeline 的运行状态，而不打算让库自动调 worker 数，通常只需要：

- `coordinator.WithStageName(...)`
- `observe.WithStageMetricsObserver(...)`
- `observe.WithLinkMetricsObserver(...)`

最常见的场景是：

- 看某个 stage 的吞吐、backlog、active / idle worker
- 看 stage 之间某条 link 的 backlog 和容量
- 把这些快照写日志、接监控、做压测对比
- 先确认瓶颈位置，再决定要不要引入 coordinator

一个最小 observe-only 接法大致是：

```go
out := flx.Stage(
	ctx,
	input,
	prepare,
	control.WithWorkers(3),
	coordinator.WithStageName("prepare"),
	observe.WithStageMetricsObserver(stageRecorder.ObserveStage),
	observe.WithLinkMetricsObserver(stageRecorder.ObserveLink),
).Through(
	ctx,
	analyze,
	control.WithForcedDynamicWorkers(analyzeController),
	coordinator.WithStageName("analyze"),
	observe.WithStageMetricsObserver(stageRecorder.ObserveStage),
	observe.WithLinkMetricsObserver(stageRecorder.ObserveLink),
)
```

这里要注意两点：

- `WithStageName(...)` 虽然定义在 `pipeline/coordinator`，但本质上是 stage 标识能力，observe-only 场景也应该配
- `observe` 只负责把快照暴露出来，不会自动做任何 worker 调整

## 2. `observe` 暴露了什么

`pipeline/observe` 主要提供两类东西。

### 2.1 挂在 operation 上的 observer

- `observe.WithStageMetricsObserver(...)`
- `observe.WithLinkMetricsObserver(...)`

它们都是 `control.Option`，直接挂在 `flx.Stage(...)`、`.Through(...)`、
`flx.MapContext(...)` 这类 operation 上。

### 2.2 coordinator 也会复用的观测模型

- `observe.StageMetrics`
- `observe.LinkMetrics`
- `observe.PipelineSnapshot`
- `observe.PipelineDecision`
- `observe.ResourceSnapshot`
- `observe.PressureSample`
- `observe.PressureLevel`
- `observe.ResourceObserver`

可以把这套类型理解成统一的数据模型：

- stage / link observer 用的是单点快照
- coordinator 用的是聚合后的 pipeline 快照和决策结果
- resource observer 用的是外部资源压力快照

## 3. 什么时候再引入 `coordinator`

当你不只是“看”，而是希望根据 backlog 或资源压力自动调节动态 worker 时，再引入
`pipeline/coordinator`。

典型信号包括：

- 下游 stage backlog 持续升高
- 某条 link backlog 持续堆积
- 内存、CPU、外部队列长度、限流状态需要参与扩缩容判断

这时需要补上的不是新的 observer，而是完整的 coordinator 接法：

1. 创建 `PipelineCoordinator`
2. 在要纳入聚合视图的 stage 上挂 `coordinator.WithCoordinator(...)`
3. 给这些 stage 标名字 `coordinator.WithStageName(...)`
4. 只给真正需要被调节的动态 stage 配 `coordinator.WithStageBudget(...)`
5. 在外部 loop 里显式调用 `Snapshot()` / `Tick()`

最小 coordinator 接法：

```go
analyzeController := control.NewConcurrencyController(1)

pipelineCoordinator := coordinator.NewPipelineCoordinator(
	coordinator.PipelineCoordinatorPolicy{
		ScaleUpStep:         1,
		ScaleDownStep:       1,
		ScaleUpCooldown:     200 * time.Millisecond,
		ScaleDownCooldown:   350 * time.Millisecond,
		ScaleUpHysteresis:   1,
		ScaleDownHysteresis: 2,
	},
	coordinator.WithResourceObserver(myResourceObserver),
)

out := flx.Stage(
	ctx,
	input,
	prepare,
	control.WithWorkers(3),
	coordinator.WithCoordinator(pipelineCoordinator),
	coordinator.WithStageName("prepare"),
).Through(
	ctx,
	analyze,
	control.WithForcedDynamicWorkers(analyzeController),
	coordinator.WithCoordinator(pipelineCoordinator),
	coordinator.WithStageName("analyze"),
	coordinator.WithStageBudget(coordinator.StageBudget{
		MinWorkers: 1,
		MaxWorkers: 4,
	}),
)
```

边界要分清：

- 固定 worker stage 可以被 observe，也可以出现在 coordinator 的 snapshot 里
- 但只有“动态 controller + `WithStageBudget(...)`”的 stage，才会真的被 coordinator 改 worker 数

## 4. `Snapshot()` 和 `Tick()` 是显式控制模型

`PipelineCoordinator` 不是后台自动运行的；当前是显式控制模型：

- `Snapshot()` 返回当前 pipeline 视图
- `Tick()` 做一次策略判断并尝试应用 worker 调整

最常见的写法是外部 ticker loop：

```go
ticker := time.NewTicker(250 * time.Millisecond)
defer ticker.Stop()

for {
	select {
	case <-ctx.Done():
		return
	case <-ticker.C:
		snapshot := pipelineCoordinator.Snapshot()
		logger.Print(snapshot)

		decision := pipelineCoordinator.Tick()
		if len(decision.Stages) > 0 {
			logger.Print(decision)
		}
	}
}
```

推荐把这个 loop 放在 pipeline 外部，而不是塞进业务 stage 里。职责会更清楚：

- stage 负责处理数据
- observe / coordinator 负责看和调

## 5. `PipelineSnapshot` 里有什么

当前 `Snapshot()` 返回三部分：

- `Stages`
- `Links`
- `Resources`

也就是说，一次读取里你可以同时拿到：

- 每个 stage 的输入、输出、backlog、worker 状态
- 每条 link 的发送、接收、backlog、容量
- 外部资源压力样本

直观理解可以是：

- `Stages` 更像“stage 自己忙不忙”
- `Links` 更像“stage 之间有没有堵住”
- `Resources` 更像“现在适不适合继续扩”

## 6. `PipelineCoordinatorPolicy` 怎么看

`PipelineCoordinatorPolicy` 当前主要控制三类东西：

- 每次最多增减多少 worker
- 同方向两次调整之间要不要冷却
- 某个方向的信号要连续命中多少次才生效

字段含义：

- `ScaleUpStep`
  单次 `Tick()` 最多扩多少 worker。
- `ScaleDownStep`
  单次 `Tick()` 最多缩多少 worker。
- `ScaleUpCooldown`
  同一 stage 两次扩容之间的最短间隔。
- `ScaleDownCooldown`
  同一 stage 两次缩容之间的最短间隔。
- `ScaleUpHysteresis`
  扩容信号需要连续命中多少个 `Tick()` 才真正扩容。
- `ScaleDownHysteresis`
  缩容信号需要连续命中多少个 `Tick()` 才真正缩容。

如果你刚开始接入，建议先用一组保守但容易观察的值：

- step 从 `1` 开始
- 缩容通常比扩容更保守
- 先让 backlog 跑起来，再回头调 cooldown / hysteresis

## 7. Resource Observer 怎么用

`coordinator.WithResourceObserver(...)` 接的是 `observe.ResourceObserver`。

最简单的写法：

```go
observer := observe.ResourceObserverFunc(func() observe.ResourceSnapshot {
	return observe.ResourceSnapshot{
		Samples: []observe.PressureSample{{
			Name:  "memory",
			Level: observe.PressureLevelWarning,
			Value: 80,
			Limit: 100,
		}},
	}
})
```

常见资源源包括：

- 内存
- CPU
- 网络
- 外部队列长度
- 下游服务限流状态

实现时建议注意：

- observer 尽量快速返回，因为它运行在调用 `Snapshot()` / `Tick()` 的 goroutine 上
- 同一个 coordinator 实例会串行化自己的 resource poll
- 但 `Snapshot()` 和 `Tick()` 仍然是两次独立采样
- 如果同一个 observer 被多个 coordinator 复用，observer 自己仍要保证并发安全

## 8. 当前边界

当前这套模型更适合：

- 线性 pipeline
- stage 边界清晰
- 只有少数动态 stage 需要自动调节

当前明确的边界是：

- 一个 `PipelineCoordinator` 实例按一次 pipeline run 使用
- 当前 link 视图按 `fromStage` 聚合
- fan-out / DAG 还没有展开成多条独立 outbound link 视图

## 9. 推荐接入顺序

如果你第一次把它接进业务代码，建议按这个顺序：

1. 先只挂 `WithStageName(...)` + `observe.With*Observer(...)`
2. 看清楚 stage / link backlog 长什么样
3. 再把真正需要被调的 stage 换成动态 controller
4. 最后再上 `WithCoordinator(...)` + `WithStageBudget(...)` + `Tick()` loop

这样最容易区分“问题是观测不够”还是“调度策略不对”。

## 10. 可运行示例

完整示例见：

- [图片处理 Observe 示例](./examples/imgproc-observe.md)
- [图片处理 Coordinator 示例](./examples/imgproc-coordinator.md)
- [图片处理流水线示例](./examples/imgproc-pipeline.md)

其中：

- `imgproc_observe` 只讲 observe
- `imgproc_coordinator` 只讲 coordinator
- `imgproc_pipeline` 继续聚焦业务 stage 组合和 native API 对照

这三条示例共用同一条图片处理业务线：列表和下载走真实 `picsum.photos`，
可选上传仍由 example 内部 transport 提供；只有 `imgproc_retry`
单独保留本地可控下载源，用来稳定演示重试。
