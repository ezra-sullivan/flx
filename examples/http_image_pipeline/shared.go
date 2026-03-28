package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	stdDraw "image/draw"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ezra-sullivan/flx"
)

const (
	defaultDownloadRetryTimes          = 3
	defaultDownloadRetryInterval       = 1000 * time.Millisecond
	defaultDownloadRetryAttemptTimeout = 8 * time.Second
)

// PipelineConfig collects the knobs used by the example pipeline. Zero or
// empty values fall back to small demo-friendly defaults so the sample can run
// without requiring every field to be populated.
type PipelineConfig struct {
	ListEndpoint           string
	ListPageSize           int
	ListMaxPages           int
	LocalOutputDir         string
	DownloadWidth          int
	DownloadHeight         int
	DownloadWorkers        int
	DownloadRetryTimes     int
	DownloadRetryInterval  time.Duration
	DownloadAttemptTimeout time.Duration
	WatermarkWorkers       int
	UploadWorkers          int
	ResizeWidth            int
	ResizeHeight           int
	WatermarkText          string
	UploadEndpoint         string
}

// PicsumPhoto models the minimal response shape this example needs from the
// Picsum list API. The pipeline only requires a stable image ID because it
// derives the actual download URL itself.
type PicsumPhoto struct {
	ID string `json:"id"`
}

// RemoteImage is the payload emitted by the listing stage and consumed by the
// download stage. Keeping the logical image ID next to the concrete source URL
// lets later stages log, persist, and correlate work items reliably.
type RemoteImage struct {
	ID        string
	SourceURL string
}

// ImageResult carries either a successful value or an item-scoped failure for a
// single image. Err does not mean the whole stream failed; it means this item
// can no longer progress and downstream stages should pass it through unchanged.
type ImageResult[T any] struct {
	ImageID   string
	SourceURL string
	Value     T
	Err       error
}

// ImageTransformer describes one pure image-processing step. Implementations
// should honor ctx cancellation, treat sourceURL as metadata only, and return a
// fresh byte slice rather than mutating shared state.
type ImageTransformer func(ctx context.Context, sourceURL string, imageBytes []byte) ([]byte, error)

// ListRemoteImages is the source stage for the example pipeline. It pages the
// Picsum listing API lazily and emits RemoteImage items as soon as they are
// discovered, which allows downstream work to start before the entire listing
// has been buffered in memory.
//
// Listing failures stop the stream immediately because there is no partially
// built business item that can carry the error forward.
func ListRemoteImages(
	ctx context.Context,
	sourceHTTPClient *http.Client,
	cfg PipelineConfig,
) flx.Stream[RemoteImage] {
	pageSize := cfg.ListPageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	downloadWidth := cfg.DownloadWidth
	if downloadWidth <= 0 {
		downloadWidth = 320
	}

	downloadHeight := cfg.DownloadHeight
	if downloadHeight <= 0 {
		downloadHeight = 240
	}

	return flx.From(func(out chan<- RemoteImage) {
		for page := 1; ; page++ {
			if cfg.ListMaxPages > 0 && page > cfg.ListMaxPages {
				return
			}

			photos, err := fetchPicsumPage(ctx, sourceHTTPClient, cfg.ListEndpoint, page, pageSize)
			if err != nil {
				panic(fmt.Errorf("list images page=%d: %w", page, err))
			}

			if len(photos) == 0 {
				return
			}

			for _, photo := range photos {
				img := RemoteImage{
					ID:        photo.ID,
					SourceURL: buildPicsumImageURL(photo.ID, downloadWidth, downloadHeight),
				}
				if !flx.SendContext(ctx, out, img) {
					return
				}
			}

			if len(photos) < pageSize {
				return
			}
		}
	})
}

// DownloadImages fetches the bytes for each listed image and converts network
// or retry exhaustion failures into item-scoped errors. That design keeps later
// stages running for unaffected items instead of failing the whole pipeline on
// the first bad download.
func DownloadImages(
	ctx context.Context,
	in flx.Stream[RemoteImage],
	sourceHTTPClient *http.Client,
	cfg PipelineConfig,
	opts ...flx.Option,
) flx.Stream[ImageResult[[]byte]] {
	return flx.MapContext(ctx, in, func(ctx context.Context, image RemoteImage) ImageResult[[]byte] {
		imageBytes, err := downloadImageWithRetry(ctx, sourceHTTPClient, image.SourceURL, cfg)
		if err != nil {
			return ImageResult[[]byte]{
				ImageID:   image.ID,
				SourceURL: image.SourceURL,
				Err:       fmt.Errorf("download %s: %w", image.SourceURL, err),
			}
		}

		return ImageResult[[]byte]{
			ImageID:   image.ID,
			SourceURL: image.SourceURL,
			Value:     imageBytes,
		}
	}, opts...)
}

// TransformImages applies one transform to every successful item while passing
// failed items through unchanged. That contract lets multiple transform stages
// compose cleanly without each stage needing to repeat failure fan-out logic.
func TransformImages(
	ctx context.Context,
	in flx.Stream[ImageResult[[]byte]],
	transform ImageTransformer,
	opts ...flx.Option,
) flx.Stream[ImageResult[[]byte]] {
	return flx.MapContext(ctx, in, func(ctx context.Context, image ImageResult[[]byte]) ImageResult[[]byte] {
		if image.Err != nil {
			return image
		}

		nextBytes, err := transform(ctx, image.SourceURL, image.Value)
		if err != nil {
			return ImageResult[[]byte]{
				ImageID:   image.ImageID,
				SourceURL: image.SourceURL,
				Err:       err,
			}
		}

		return ImageResult[[]byte]{
			ImageID:   image.ImageID,
			SourceURL: image.SourceURL,
			Value:     nextBytes,
		}
	}, opts...)
}

// SaveLocalImages writes successful items into a stage-specific directory while
// returning the original payload so later stages can continue processing. Local
// file I/O therefore follows the same worker, cancellation, and item-level
// error model as every other stage in the example.
func SaveLocalImages(
	ctx context.Context,
	in flx.Stream[ImageResult[[]byte]],
	baseDir string,
	stage string,
	opts ...flx.Option,
) flx.Stream[ImageResult[[]byte]] {
	if strings.TrimSpace(baseDir) == "" {
		return in
	}

	return flx.MapContext(ctx, in, func(ctx context.Context, image ImageResult[[]byte]) ImageResult[[]byte] {
		if image.Err != nil {
			return image
		}

		if err := saveImageBytes(baseDir, stage, image); err != nil {
			return ImageResult[[]byte]{
				ImageID:   image.ImageID,
				SourceURL: image.SourceURL,
				Err:       fmt.Errorf("save %s image %s: %w", stage, image.SourceURL, err),
			}
		}

		return image
	}, opts...)
}

// UploadImages posts the final image bytes to an optional target endpoint and
// converts upload failures into item-scoped errors. Successful uploads keep the
// returned remote URL in Value so the sink can log where each image landed.
func UploadImages(
	ctx context.Context,
	in flx.Stream[ImageResult[[]byte]],
	targetHTTPClient *http.Client,
	uploadEndpoint string,
	opts ...flx.Option,
) flx.Stream[ImageResult[string]] {
	return flx.MapContext(ctx, in, func(ctx context.Context, image ImageResult[[]byte]) ImageResult[string] {
		if image.Err != nil {
			return ImageResult[string]{
				ImageID:   image.ImageID,
				SourceURL: image.SourceURL,
				Err:       image.Err,
			}
		}

		postedURL, err := postImage(ctx, targetHTTPClient, uploadEndpoint, image.SourceURL, image.Value)
		if err != nil {
			return ImageResult[string]{
				ImageID:   image.ImageID,
				SourceURL: image.SourceURL,
				Err:       fmt.Errorf("post image %s: %w", image.SourceURL, err),
			}
		}

		return ImageResult[string]{
			ImageID:   image.ImageID,
			SourceURL: image.SourceURL,
			Value:     postedURL,
		}
	}, opts...)
}

// consumeProcessedImages drains the locally processed stream, logs the outcome
// for each item, and emits one summary line at the end. Only stream-level
// iteration failures are returned; per-item failures are already encoded in Err.
func consumeProcessedImages(images flx.Stream[ImageResult[[]byte]]) error {
	var successCount int
	var failedCount int

	err := images.ForEachErr(func(image ImageResult[[]byte]) {
		if image.Err != nil {
			failedCount++
			log.Printf("image processing failed id=%s source=%s err=%s", image.ImageID, image.SourceURL, formatItemError(image.Err))
			return
		}

		successCount++
		log.Printf("image processed id=%s source=%s bytes=%d", image.ImageID, image.SourceURL, len(image.Value))
	})
	if err != nil {
		return err
	}

	log.Printf("image processing complete success=%d failed=%d", successCount, failedCount)
	return nil
}

// consumeUploadedImages drains the upload result stream with the same reporting
// contract as consumeProcessedImages, but includes the returned target URL for
// each successful upload.
func consumeUploadedImages(results flx.Stream[ImageResult[string]]) error {
	var successCount int
	var failedCount int

	err := results.ForEachErr(func(image ImageResult[string]) {
		if image.Err != nil {
			failedCount++
			log.Printf("image upload failed id=%s source=%s err=%s", image.ImageID, image.SourceURL, formatItemError(image.Err))
			return
		}

		successCount++
		log.Printf("image uploaded id=%s source=%s target=%s", image.ImageID, image.SourceURL, image.Value)
	})
	if err != nil {
		return err
	}

	log.Printf("image upload complete success=%d failed=%d", successCount, failedCount)
	return nil
}

// formatItemError flattens wrapped and joined errors into one compact log line.
// This is especially useful when retry helpers return one leaf error per
// attempt, because otherwise a single item failure can become very noisy.
func formatItemError(err error) string {
	leafErrs := collectLeafErrors(err)
	if len(leafErrs) == 0 {
		return compactErrorText(err.Error())
	}

	counts := make(map[string]int, len(leafErrs))
	order := make([]string, 0, len(leafErrs))
	for _, leafErr := range leafErrs {
		message := compactErrorText(leafErr.Error())
		if message == "" {
			continue
		}
		if counts[message] == 0 {
			order = append(order, message)
		}
		counts[message]++
	}

	if len(order) == 0 {
		return compactErrorText(err.Error())
	}

	parts := make([]string, 0, len(order))
	for _, message := range order {
		count := counts[message]
		if count == 1 {
			parts = append(parts, message)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s x%d", message, count))
	}

	return strings.Join(parts, "; ")
}

// collectLeafErrors walks wrapped and joined errors until it reaches the leaf
// failures that best describe what actually happened. Joined errors expand to
// multiple leaves so retry output can be summarized accurately.
func collectLeafErrors(err error) []error {
	if err == nil {
		return nil
	}

	type multiUnwrapper interface {
		Unwrap() []error
	}

	if unwrapper, ok := err.(multiUnwrapper); ok {
		children := unwrapper.Unwrap()
		if len(children) == 0 {
			return []error{err}
		}

		leafErrs := make([]error, 0, len(children))
		for _, child := range children {
			leafErrs = append(leafErrs, collectLeafErrors(child)...)
		}
		return leafErrs
	}

	if child := errors.Unwrap(err); child != nil {
		return collectLeafErrors(child)
	}

	return []error{err}
}

// compactErrorText normalizes whitespace so multi-line errors can be rendered as
// single-line log messages without losing their core content.
func compactErrorText(message string) string {
	return strings.Join(strings.Fields(message), " ")
}

// fetchPicsumPage requests one page of image metadata from the list endpoint and
// decodes the subset of JSON this example cares about. Any non-2xx response is
// treated as a hard source-stage failure.
func fetchPicsumPage(
	ctx context.Context,
	client *http.Client,
	listEndpoint string,
	page int,
	limit int,
) ([]PicsumPhoto, error) {
	parsedURL, err := url.Parse(listEndpoint)
	if err != nil {
		return nil, err
	}

	query := parsedURL.Query()
	query.Set("page", strconv.Itoa(page))
	query.Set("limit", strconv.Itoa(limit))
	parsedURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var photos []PicsumPhoto
	if err := json.NewDecoder(resp.Body).Decode(&photos); err != nil {
		return nil, err
	}

	return photos, nil
}

// buildPicsumImageURL derives the concrete image download URL for one listed
// photo ID. Width and height are embedded directly into the path so every item
// is downloaded at a predictable size.
func buildPicsumImageURL(photoID string, width int, height int) string {
	return fmt.Sprintf("https://picsum.photos/id/%s/%d/%d.jpg", photoID, width, height)
}

// saveImageBytes writes one successful image result into a stage-specific
// folder. The image ID becomes the filename so original and processed artifacts
// line up by item and are easy to compare after the example finishes.
func saveImageBytes(baseDir string, stage string, image ImageResult[[]byte]) error {
	filename := image.ImageID
	if filename == "" {
		filename = "unknown"
	}

	dir := filepath.Join(baseDir, stage)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	path := filepath.Join(dir, filename+".jpg")
	return os.WriteFile(path, image.Value, 0o644)
}

// downloadImageWithRetry centralizes retry policy so both pipeline variants use
// the same timeout, backoff, and retry count. The returned bytes always come
// from the final successful attempt.
func downloadImageWithRetry(ctx context.Context, client *http.Client, imageURL string, cfg PipelineConfig) ([]byte, error) {
	var imageBytes []byte

	retryTimes := cfg.DownloadRetryTimes
	if retryTimes <= 0 {
		retryTimes = defaultDownloadRetryTimes
	}

	retryInterval := cfg.DownloadRetryInterval
	if retryInterval <= 0 {
		retryInterval = defaultDownloadRetryInterval
	}

	attemptTimeout := cfg.DownloadAttemptTimeout
	if attemptTimeout <= 0 {
		attemptTimeout = defaultDownloadRetryAttemptTimeout
	}

	err := flx.DoWithRetryCtx(
		ctx,
		func(ctx context.Context, attempt int) error {
			data, err := downloadImage(ctx, client, imageURL)
			if err != nil {
				return fmt.Errorf("attempt %d: %w", attempt+1, err)
			}

			imageBytes = data
			return nil
		},
		flx.WithRetry(retryTimes),
		flx.WithInterval(retryInterval),
		flx.WithAttemptTimeout(attemptTimeout),
	)
	if err != nil {
		return nil, err
	}

	return imageBytes, nil
}

// downloadImage performs a single HTTP GET for image bytes. Retry and backoff
// are handled by the caller so this helper stays focused on one transport
// attempt and one response validation step.
func downloadImage(ctx context.Context, client *http.Client, imageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// postImage uploads transformed bytes to the optional target endpoint and
// returns the remote URL reported by that service. The original source URL is
// forwarded as metadata so receivers can trace the uploaded artifact back to
// the source item.
func postImage(
	ctx context.Context,
	client *http.Client,
	uploadEndpoint string,
	sourceURL string,
	imageBytes []byte,
) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadEndpoint, bytes.NewReader(imageBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Source-URL", sourceURL)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.URL, nil
}

// ResizeTo returns a transformer that resizes images with a nearest-neighbor
// strategy. The artificial delay makes dynamic worker resizing visible while
// the example is running, which is useful for demonstration but not meant as a
// production pattern.
func ResizeTo(width int, height int) ImageTransformer {
	return func(ctx context.Context, sourceURL string, imageBytes []byte) ([]byte, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}

		_ = sourceURL

		srcImage, err := decodeImage(imageBytes)
		if err != nil {
			return nil, fmt.Errorf("decode for resize: %w", err)
		}

		resized := resizeNearest(srcImage, width, height)
		return encodeJPEG(resized)
	}
}

// AddWatermarkText returns a lightweight transformer that overlays a visible
// footer marker. The implementation is intentionally simple: the example only
// needs to show that a second transform stage ran, not implement a full text
// rendering pipeline.
func AddWatermarkText(text string) ImageTransformer {
	return func(ctx context.Context, sourceURL string, imageBytes []byte) ([]byte, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}

		srcImage, err := decodeImage(imageBytes)
		if err != nil {
			return nil, fmt.Errorf("decode for watermark: %w", err)
		}

		watermarked := addWatermarkMarker(srcImage, text, sourceURL)
		return encodeJPEG(watermarked)
	}
}

// decodeImage converts encoded bytes into an image.Image using the standard
// decoders linked into this binary.
func decodeImage(imageBytes []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, err
	}

	return img, nil
}

// encodeJPEG serializes the transformed image with a fixed quality setting so
// the example produces predictable JPEG output.
func encodeJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// resizeNearest performs a simple nearest-neighbor resize into a fresh RGBA
// buffer. The implementation favors readability and zero external dependencies
// over image quality because this is example code, not a general-purpose image
// processing library.
func resizeNearest(src image.Image, width int, height int) *image.RGBA {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		srcY := srcBounds.Min.Y + y*srcHeight/height
		for x := 0; x < width; x++ {
			srcX := srcBounds.Min.X + x*srcWidth/width
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}

	return dst
}

// addWatermarkMarker draws a translucent footer bar plus accent blocks so the
// processed image is visibly distinct from the original. The accent color is
// derived from the watermark inputs, which keeps the marker deterministic for a
// given item without storing extra state.
func addWatermarkMarker(src image.Image, text string, sourceURL string) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	stdDraw.Draw(dst, dst.Bounds(), src, bounds.Min, stdDraw.Src)

	barHeight := max(bounds.Dy()/6, 24)
	barRect := image.Rect(0, bounds.Dy()-barHeight, bounds.Dx(), bounds.Dy())
	barColor := color.NRGBA{R: 24, G: 24, B: 24, A: 150}
	stdDraw.Draw(dst, barRect, image.NewUniform(barColor), image.Point{}, stdDraw.Over)

	accent := watermarkAccent(text, sourceURL)
	blockWidth := max(bounds.Dx()/18, 12)
	blockGap := max(blockWidth/3, 4)
	startX := 12
	topY := bounds.Dy() - barHeight + 8
	blockHeight := max(barHeight-16, 8)

	for i := 0; i < 6 && startX < bounds.Dx()-blockWidth; i++ {
		rect := image.Rect(startX, topY, startX+blockWidth, topY+blockHeight)
		stdDraw.Draw(dst, rect, image.NewUniform(accent), image.Point{}, stdDraw.Over)
		startX += blockWidth + blockGap
	}

	return dst
}

// watermarkAccent derives a repeatable accent color from the watermark text and
// source URL so the visual marker is deterministic for one item while still
// varying across different inputs.
func watermarkAccent(text string, sourceURL string) color.NRGBA {
	sum := 0
	for _, b := range []byte(text + sourceURL) {
		sum += int(b)
	}

	return color.NRGBA{
		R: uint8(80 + sum%120),
		G: uint8(40 + (sum/3)%120),
		B: uint8(120 + (sum/7)%100),
		A: 190,
	}
}
