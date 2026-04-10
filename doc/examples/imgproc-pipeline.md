# 图片处理流水线示例

这个示例是 `imgproc` example family 的业务基线，只讲 pipeline 组合本身，不混入
`observe` 或 `coordinator` 的控制逻辑。

相关文件：

- 入口：[examples/imgproc_pipeline/main.go](../../examples/imgproc_pipeline/main.go)
- `Stage` 组合版：[examples/imgproc_pipeline/stage_pipeline.go](../../examples/imgproc_pipeline/stage_pipeline.go)
- 原生 `flx` API 版：[examples/imgproc_pipeline/native_pipeline.go](../../examples/imgproc_pipeline/native_pipeline.go)
- 共享业务层：[examples/internal/imgproc](../../examples/internal/imgproc)

## 运行方式

默认运行：

```powershell
go run ./examples/imgproc_pipeline
```

等价于：

```powershell
go run ./examples/imgproc_pipeline -mode stage
```

切到原生 `flx` API 对照版：

```powershell
go run ./examples/imgproc_pipeline -mode native
```

## 这个例子讲什么

这条业务线的 source 来自真实 `picsum.photos`：

- 列表来自 `https://picsum.photos/v2/list`
- 下载图片 URL 来自 `https://picsum.photos/id/...`
- 下载完成后的 resize、水印、本地保存仍然在本地执行

业务流程保持不变：

1. 分页列出示例图片
2. 下载图片字节
3. 保存原图
4. resize
5. 加水印
6. 保存处理图
7. 可选上传

默认配置来自
[examples/internal/imgproc/model.go](../../examples/internal/imgproc/model.go)
的 `DefaultPipelineConfig(...)`。这个 example 自己只负责：

- 组织业务 stage 顺序
- 演示 `Stage` DSL
- 提供一份 native `flx` 对照写法

如果你想打开上传阶段，可在
[examples/imgproc_pipeline/main.go](../../examples/imgproc_pipeline/main.go)
里启用 `cfg.UploadEndpoint = imgproc.ExampleUploadEndpoint()`。

## 本地输出

输出目录：

- 原图：`examples/imgproc_pipeline/.output/original/`
- 处理图：`examples/imgproc_pipeline/.output/processed/`

## 和其他 example 的关系

如果你想看同一条 Picsum 图片处理业务线上的控制面接法，去看：

- [图片处理 Observe 示例](./imgproc-observe.md)
- [图片处理 Coordinator 示例](./imgproc-coordinator.md)

如果你想单独稳定观察下载阶段的 retry 行为，去看：

- [图片处理重试示例](./imgproc-retry.md)
