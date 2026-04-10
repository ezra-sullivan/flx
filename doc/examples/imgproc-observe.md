# 图片处理 Observe 示例

这个示例只讲 `pipeline/observe`，业务场景与主业务 example 相同，都是
“Picsum 下载 + 本地处理” 这条流水线。

相关文件：

- 入口：[examples/imgproc_observe/main.go](../../examples/imgproc_observe/main.go)
- observe 主流程：[examples/imgproc_observe/observe_pipeline.go](../../examples/imgproc_observe/observe_pipeline.go)
- 本地 recorder：[examples/imgproc_observe/recorder.go](../../examples/imgproc_observe/recorder.go)
- snapshot 日志格式：[examples/imgproc_observe/logging.go](../../examples/imgproc_observe/logging.go)
- 共享业务层：[examples/internal/imgproc](../../examples/internal/imgproc)

## 运行方式

```powershell
go run ./examples/imgproc_observe
```

## 这个例子讲什么

它只做 observe，不做 coordinator：

- 给每个 stage 挂 `coordinator.WithStageName(...)`
- 给每个 stage 挂 `observe.WithStageMetricsObserver(...)`
- 给每条 link 挂 `observe.WithLinkMetricsObserver(...)`
- 用一个本地 recorder 聚合最新快照
- 用独立 loop 周期性打印 snapshot

你会看到类似日志：

```text
kind=observe_snapshot snapshot=tick stages=[download{...} | resize{...}] links=[download->save-original{...} | resize->watermark{...}]
kind=observe_snapshot snapshot=final stages=[...] links=[...]
```

## 为什么这里仍然保留动态 resize stage

这个 example 故意保留了动态 worker 的 resize stage，并用一个简单的手动
controller 变化来制造 worker 和 backlog 变化。这样可以更清楚地回答：

- observe 能看到什么
- stage / link backlog 怎么变化
- worker 数变化时，snapshot 会长什么样

但这里没有：

- `PipelineCoordinator`
- `WithCoordinator(...)`
- `WithStageBudget(...)`
- `Tick()` loop

## 数据来源说明

observe example 与主业务 example 共用同一条 Picsum source 链路：

- image catalog 来自 Picsum list API
- 图片下载字节来自真实 Picsum 图片 URL
- 默认输出仍写到 `examples/imgproc_observe/.output/`

如果你想看 coordinator、decision、resource observer 和 `Tick()` 节奏，去看：

- [图片处理 Coordinator 示例](./imgproc-coordinator.md)
