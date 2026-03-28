# HTTP 图片处理流水线示例

这个示例已经落到真实代码目录，运行时会从 **Lorem Picsum** 拉取一批图片，做两段处理，然后把原图和处理图都保存到本地，方便直接肉眼对比。

相关文件：
- 入口：[examples/http_image_pipeline/main.go](../../examples/http_image_pipeline/main.go)
- stage 函数组合版：[examples/http_image_pipeline/stage_pipeline.go](../../examples/http_image_pipeline/stage_pipeline.go)
- 原生 `flx` API 版：[examples/http_image_pipeline/native_pipeline.go](../../examples/http_image_pipeline/native_pipeline.go)
- 共享类型与 helper：[examples/http_image_pipeline/shared.go](../../examples/http_image_pipeline/shared.go)

## 运行方式

在仓库根目录执行：

```powershell
go run ./examples/http_image_pipeline
```

默认配置在 [examples/http_image_pipeline/main.go](../../examples/http_image_pipeline/main.go)：

- 图片列表来源：`https://picsum.photos/v2/list`
- `ListPageSize: 5`
- `ListMaxPages: 1`
- 下载大图尺寸：`1280x960`
- resize 后缩略图尺寸：`320x240`
- `DownloadWorkers: 4`
- 下载阶段会用 `flx.DoWithRetryCtx(...)` 自动重试：默认 `3` 次，间隔 `250ms`，单次尝试超时 `8s`
- resize 阶段使用动态 worker
- watermark 阶段固定 `4` 个 worker
- 上传默认关闭

## 本地输出

示例默认会把图片写到：`examples/http_image_pipeline/output/`

具体目录：
- 原图：`examples/http_image_pipeline/output/original/`
- 处理图：`examples/http_image_pipeline/output/processed/`

这样跑完以后，你可以直接打开同一个图片 ID 的两份文件对比：
- `original/<image-id>.jpg`：下载下来的大图
- `processed/<image-id>.jpg`：缩略后的处理图

当前示例里的处理逻辑不是空占位，而是会实际做两件事：
- 把大图缩成更小的缩略图
- 在底部叠一条明显的标记带，方便确认处理结果确实变化了

这些处理函数都在 [examples/http_image_pipeline/shared.go](../../examples/http_image_pipeline/shared.go)。

## 两种写法

### 1. stage 函数组合版

见 [examples/http_image_pipeline/stage_pipeline.go](../../examples/http_image_pipeline/stage_pipeline.go)。

这一版更接近业务流水线，代码顺序基本就是：

```go
listedImages := ListRemoteImages(ctx, sourceHTTPClient, cfg)

images := DownloadImages(ctx, listedImages, sourceHTTPClient, flx.WithWorkers(cfg.DownloadWorkers))
images = SaveLocalImages(ctx, images, cfg.LocalOutputDir, "original", flx.WithWorkers(2))
images = TransformImages(ctx, images, ResizeTo(cfg.ResizeWidth, cfg.ResizeHeight), flx.WithForcedDynamicWorkers(resizeController))
images = TransformImages(ctx, images, AddWatermarkText(cfg.WatermarkText), flx.WithWorkers(cfg.WatermarkWorkers))
images = SaveLocalImages(ctx, images, cfg.LocalOutputDir, "processed", flx.WithWorkers(2))
```

适合：
- 业务 stage 会经常增删调整
- 想把 `DownloadImages`、`TransformImages`、`UploadImages` 这些步骤复用到别的任务
- 想让每个 stage 的并发策略在调用点上清晰可见

推荐变量风格：
- `listedImages`
- `images`
- `results`

中间统一复用 `images`，后续要删掉一个 stage、插入一个 stage，通常只需要改调用点，不需要重命名一串中间变量。

### 2. 原生 `flx` API 版

见 [examples/http_image_pipeline/native_pipeline.go](../../examples/http_image_pipeline/native_pipeline.go)。

这一版直接把 `flx.From(...)`、`flx.MapContext(...)` 串起来，适合看最原始的流式拼装方式。优点是每一段行为都展开得很直接，排查问题时也更容易定位到底是哪一个 `MapContext(...)` 在做什么；代价是 stage 变多以后，可读性会比 stage 函数组合版差一些。

## 上传怎么打开

上传步骤在示例里默认注释掉了，因为公开可写的 HTTP 上传测试服务通常不稳定。要启用上传，只需要在 [examples/http_image_pipeline/main.go](../../examples/http_image_pipeline/main.go) 里填上你自己的接口：

```go
UploadEndpoint: "https://your-upload-service.example/upload",
```

填上以后：
- `RunStagePipeline(...)` 会在本地保存处理图之后继续调用 `UploadImages(...)`
- `RunNativePipeline(...)` 会继续执行最后一段上传 `MapContext(...)`

