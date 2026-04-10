# 图片处理重试示例

这个示例专门演示 `flx.DoWithRetryCtx(...)` 的下载重试行为。它是整个 `imgproc`
example family 里唯一保持完全本地化的示例，因为这里需要稳定控制失败次数和重试结果。

相关文件：

- 入口：[examples/imgproc_retry/main.go](../../examples/imgproc_retry/main.go)
- 主流程：[examples/imgproc_retry/retry_example.go](../../examples/imgproc_retry/retry_example.go)
- 本地 source / flaky transport：[examples/imgproc_retry/transport.go](../../examples/imgproc_retry/transport.go)
- 测试：[examples/imgproc_retry/retry_example_test.go](../../examples/imgproc_retry/retry_example_test.go)

## 运行方式

```powershell
go run ./examples/imgproc_retry
```

这个示例不依赖外网。它在本地生成图片字节，并通过可控 transport 稳定复现三种情况：

- 某个下载 3 次都失败
- 某个下载第 2 次成功
- 某个下载第 3 次成功

## 演示内容

当前固定 case 是：

- `always-fails`
  对应图片 `alpine-dawn`，所有尝试都失败，最终耗尽 `3` 次预算。
- `succeeds-on-second`
  对应图片 `city-lights`，第 `1` 次失败，第 `2` 次成功。
- `succeeds-on-third`
  对应图片 `forest-path`，前 `2` 次失败，第 `3` 次成功。

示例内部对每个 case 都使用：

```go
flx.DoWithRetryCtx(
	ctx,
	fn,
	flx.WithRetry(3),
	flx.WithInterval(150*time.Millisecond),
	flx.WithAttemptTimeout(2*time.Second),
	flx.WithOnRetry(...),
)
```

其中：

- `WithRetry(3)` 仍表示“最多 3 次总尝试”，包含第一次请求。
- `WithOnRetry(...)` 会在失败后、下次重试前打印 warning。
- 最终成功或失败结果会按 item 汇总输出。

## 典型输出

输出会类似下面这样：

```text
kind=download_retry_warning case=succeeds-on-third image=forest-path retry=1/3 err=unexpected status: 503 Service Unavailable
kind=download_retry_warning case=succeeds-on-third image=forest-path retry=2/3 err=unexpected status: 503 Service Unavailable
kind=download_retry_succeeded case=succeeds-on-third image=forest-path attempts=3 bytes=...
kind=download_retry_failed case=always-fails image=alpine-dawn attempts=3 err=unexpected status: 503 Service Unavailable x3
kind=download_retry_complete success=2 failed=1
```

重点是：

- 主 imgproc examples 走真实 Picsum 下载，这个示例单独保持本地可控。
- warning 只表示“这次失败后准备重试”。
- 最终 item 结果才表示“这个下载最终成功了还是耗尽预算失败了”。
- 你可以直接对照同一个 `case=` 和 `image=` 看完整重试路径。
