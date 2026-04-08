# HTTP 图片处理流水线示例

这个示例已经落到真实代码目录，运行时会从 **Lorem Picsum** 拉取一批图片，做下载、缩放、水印和本地保存。默认会把原图和处理图都写到本地，方便直接肉眼对比。

相关文件：
- 入口：[examples/http_image_pipeline/main.go](../../examples/http_image_pipeline/main.go)
- coordinator 版：[examples/http_image_pipeline/coordinator_pipeline.go](../../examples/http_image_pipeline/coordinator_pipeline.go)
- `Stage` 组合版：[examples/http_image_pipeline/stage_pipeline.go](../../examples/http_image_pipeline/stage_pipeline.go)
- 原生 `flx` API 版：[examples/http_image_pipeline/native_pipeline.go](../../examples/http_image_pipeline/native_pipeline.go)
- 共享类型与 helper：[examples/http_image_pipeline/shared.go](../../examples/http_image_pipeline/shared.go)
- 水印绘制实现：[examples/http_image_pipeline/watermark.go](../../examples/http_image_pipeline/watermark.go)

## 运行方式

在仓库根目录执行：

```powershell
go run ./examples/http_image_pipeline
```

默认等价于：

```powershell
go run ./examples/http_image_pipeline -mode coordinator
```

也可以显式切换模式：

```powershell
go run ./examples/http_image_pipeline -mode coordinator
go run ./examples/http_image_pipeline -mode stage
go run ./examples/http_image_pipeline -mode native
```

## 默认配置

默认配置在 [examples/http_image_pipeline/main.go](../../examples/http_image_pipeline/main.go)：

- 图片列表来源：`https://picsum.photos/v2/list`
- `ListPageSize: 5`
- `ListMaxPages: 1`
- 下载大图尺寸：`1920x1080`
- resize 后缩略图尺寸：`960x540`
- `DownloadWorkers: 4`
- 下载阶段会用 `flx.DoWithRetryCtx(...)` 自动重试：默认 `3` 次，间隔 `250ms`，单次尝试超时 `8s`
- resize 阶段使用动态 worker
- watermark 阶段固定 `4` 个 worker
- 上传默认关闭

## 本地输出

示例默认会把图片写到：`examples/http_image_pipeline/.output/`

具体目录：
- 原图：`examples/http_image_pipeline/.output/original/`
- 处理图：`examples/http_image_pipeline/.output/processed/`

这样跑完以后，你可以直接打开同一个图片 ID 的两份文件对比：
- `original/<image-id>.jpg`：下载下来的大图
- `processed/<image-id>.jpg`：缩略并加水印后的处理图

当前示例里的处理逻辑不是空占位，而是会实际做两件事：
- 把大图缩成更小的缩略图
- 在底部叠半透明标记带和内置位图字体水印文字，方便确认处理结果确实变化了

这些处理函数分布在 [examples/http_image_pipeline/shared.go](../../examples/http_image_pipeline/shared.go) 和 [examples/http_image_pipeline/watermark.go](../../examples/http_image_pipeline/watermark.go)。

## 三种写法

### 1. coordinator 版

见 [examples/http_image_pipeline/coordinator_pipeline.go](../../examples/http_image_pipeline/coordinator_pipeline.go)。

这一版在 `Stage` DSL 之上挂了：
- `pipeline/coordinator.WithCoordinator(...)`
- `pipeline/coordinator.WithStageName(...)`
- `pipeline/coordinator.WithStageBudget(...)`
- `pipeline/coordinator.WithResourceObserver(...)`

它会周期性调用：
- `PipelineCoordinator.Snapshot()`：读取 stage/link/resource 三视图
- `PipelineCoordinator.Tick()`：按当前 backlog 和资源压力做一次 worker 调整决策

示例里的 resource observer 是一个最小实现：它读取 `runtime.MemStats`，把当前堆内存占用映射成 `observe.ResourceSnapshot`，然后让 snapshot 日志里直接带出 memory pressure。

示例里当前使用的 policy 是：

```go
pipelinecoordinator.PipelineCoordinatorPolicy{
	ScaleUpStep:         1,
	ScaleDownStep:       1,
	ScaleUpCooldown:     500 * time.Millisecond,
	ScaleDownCooldown:   750 * time.Millisecond,
	ScaleUpHysteresis:   1,
	ScaleDownHysteresis: 2,
}
```

这几个字段的含义：

- `ScaleUpStep` / `ScaleDownStep`
  单次 `Tick()` 最多增减多少 worker。
- `ScaleUpCooldown` / `ScaleDownCooldown`
  同一 stage 同方向两次调整之间至少要隔多久。
- `ScaleUpHysteresis` / `ScaleDownHysteresis`
  对应方向的信号需要连续命中多少个 `Tick()` 才真正触发调整。

这组默认示例策略的意图是：

- backlog 一出现可以尽快扩容
- 缩容比扩容更保守
- shrink 需要更稳定的空闲信号，避免乒乓

适合：
- 想先看清楚 coordinator API 怎么挂到真实 pipeline 上
- 想验证 `Snapshot()` 的输出长什么样
- 想看 `Tick()` 怎么和动态 worker stage 协作

### 2. `Stage` 组合版

见 [examples/http_image_pipeline/stage_pipeline.go](../../examples/http_image_pipeline/stage_pipeline.go)。

这一版直接把 `flx.Stage(...)` 摆在跨类型边界上，再用 `Stream[T].Through(...)` 串起中间保持同类型的 stage。代码顺序基本就是业务流水线本身。

这类调用点现在推荐同时导入：
- `github.com/ezra-sullivan/flx`
- `github.com/ezra-sullivan/flx/pipeline/control`

```go
listedImages := ListRemoteImages(ctx, sourceHTTPClient, cfg)

images := flx.Stage(ctx, listedImages, downloadImageStage(sourceHTTPClient, cfg), control.WithWorkers(cfg.DownloadWorkers)).
	Through(ctx, saveLocalImageStage(cfg.LocalOutputDir, "original"), control.WithWorkers(2)).
	Through(ctx, transformImageStage(ResizeTo(cfg.ResizeWidth, cfg.ResizeHeight)), control.WithForcedDynamicWorkers(resizeController)).
	Through(ctx, transformImageStage(AddWatermarkText(cfg.WatermarkText)), control.WithWorkers(cfg.WatermarkWorkers)).
	Through(ctx, saveLocalImageStage(cfg.LocalOutputDir, "processed"), control.WithWorkers(2))
```

适合：
- 业务 stage 会经常增删调整
- 想让 pipeline 调用点直接体现“这是一个 stage”，而不是一段底层变换拼装
- 想把每个 stage 的业务逻辑收敛成小 mapper helper，再把并发策略留在调用点

### 3. 原生 `flx` API 版

见 [examples/http_image_pipeline/native_pipeline.go](../../examples/http_image_pipeline/native_pipeline.go)。

这一版直接把 `flx.From(...)`、`flx.MapContext(...)` 串起来，适合看最原始的流式拼装方式。优点是每一段行为都展开得很直接，排查问题时更容易定位到底是哪一个变换在做什么；代价是 stage 变多以后，可读性会比 `Stage` 组合版差一些。

## coordinator 模式会看到什么

coordinator 模式会额外打印两类日志：

- snapshot 日志：包含 stage、link、resource 三个视图
- decision 日志：包含某个 stage 从多少 worker 调整到多少 worker，以及原因

resource 视图当前会输出类似：

```text
memory(ok 24.3MiB/256.0MiB)
```

这只是示例实现。业务代码里你可以用同样的 `observe.ResourceObserver` 接口换成网络、内存、CPU、外部队列长度等任意资源观测源。

## 上传怎么打开

上传步骤在示例里默认注释掉了，因为公开可写的 HTTP 上传测试服务通常不稳定。要启用上传，只需要在 [examples/http_image_pipeline/main.go](../../examples/http_image_pipeline/main.go) 里填上你自己的接口：

```go
UploadEndpoint: "https://your-upload-service.example/upload",
```

填上以后：
- `RunCoordinatorPipeline(...)` 会在本地保存处理图之后继续执行最后一段上传 stage，并继续输出 snapshot/decision 日志
- `RunStagePipeline(...)` 会在本地保存处理图之后继续执行最后一段 `flx.Stage(...)` 上传
- `RunNativePipeline(...)` 会继续执行最后一段上传 `MapContext(...)`
