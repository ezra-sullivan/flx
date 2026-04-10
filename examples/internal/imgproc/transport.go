package imgproc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ezra-sullivan/flx"
)

// DownloadImageWithRetry centralizes the retry policy shared by the examples.
func DownloadImageWithRetry(
	ctx context.Context,
	client *http.Client,
	image RemoteImage,
	cfg PipelineConfig,
) ([]byte, error) {
	var imageBytes []byte
	imageURL := image.SourceURL

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
		flx.WithOnRetry(func(event flx.RetryEvent) {
			logDownloadRetryWarning(imageURL, event)
		}),
	)
	if err != nil {
		return nil, err
	}

	return imageBytes, nil
}

// SaveImageBytes writes one successful image result into a stage-specific
// folder.
func SaveImageBytes(baseDir string, stage string, image ImageResult[[]byte]) error {
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

// PostImage uploads transformed bytes to the optional target endpoint.
func PostImage(
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
	defer func() {
		_ = resp.Body.Close()
	}()

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

// FormatDownloadRetryWarning keeps retry output aligned with the shared
// `kind=` logging contract.
func FormatDownloadRetryWarning(imageURL string, event flx.RetryEvent) string {
	return formatKinded(
		logKindDownloadRetryWarning,
		"retry=%d/%d url=%s err=%s",
		event.Retry,
		event.MaxAttempts,
		imageURL,
		FormatItemError(event.Err),
	)
}

func logDownloadRetryWarning(imageURL string, event flx.RetryEvent) {
	log.Print(FormatDownloadRetryWarning(imageURL, event))
}

func downloadImage(ctx context.Context, client *http.Client, imageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}
