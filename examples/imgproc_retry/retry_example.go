package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ezra-sullivan/flx"
	imgproc "github.com/ezra-sullivan/flx/examples/internal/imgproc"
	"github.com/ezra-sullivan/flx/pipeline/control"
)

const (
	// Keep the retry budget intentionally small so the example finishes quickly
	// while still showing meaningful retry behavior.
	retryExampleMaxAttempts    = 3
	retryExampleRetryInterval  = 150 * time.Millisecond
	retryExampleAttemptTimeout = 2 * time.Second
)

// retryDownloadCase describes one deterministic image-processing retry
// scenario.
type retryDownloadCase struct {
	Name                  string
	Image                 imgproc.RemoteImage
	FailuresBeforeSuccess int
}

// retryDownloadResult is the item emitted by the Stage pipeline after all retry
// attempts for one case have completed.
type retryDownloadResult struct {
	Name     string
	ImageID  string
	URL      string
	Attempts int
	Bytes    int
	Err      error
}

// runDownloadRetryExample assembles the deterministic retry demo from a local
// image transport plus one flaky wrapper layer that injects per-image failures.
func runDownloadRetryExample(ctx context.Context) error {
	cases := demoRetryDownloadCases()
	client := newRetryExampleClient(cases)

	// Use one worker per case so each scenario can progress independently and the
	// logs show overlapping retry timelines.
	results := flx.Stage(
		ctx,
		flx.Values(cases...),
		func(ctx context.Context, item retryDownloadCase) retryDownloadResult {
			return downloadCaseWithRetry(ctx, client, item)
		},
		control.WithWorkers(len(cases)),
	)

	var successCount int
	var failedCount int
	err := results.ForEachErr(func(result retryDownloadResult) {
		if result.Err != nil {
			failedCount++
			logRetryExamplef(
				logKindDownloadRetryFailed,
				"case=%s image=%s attempts=%d err=%s",
				result.Name,
				result.ImageID,
				result.Attempts,
				imgproc.FormatItemError(result.Err),
			)
			return
		}

		successCount++
		logRetryExamplef(
			logKindDownloadRetrySucceeded,
			"case=%s image=%s attempts=%d bytes=%d",
			result.Name,
			result.ImageID,
			result.Attempts,
			result.Bytes,
		)
	})
	if err != nil {
		return err
	}

	logRetryExamplef(
		logKindDownloadRetryComplete,
		"success=%d failed=%d",
		successCount,
		failedCount,
	)
	return nil
}

// demoRetryDownloadCases captures the three outcomes the example wants to
// demonstrate on the controlled local image dataset: permanent failure, recovery on
// the second attempt, and recovery on the third attempt.
func demoRetryDownloadCases() []retryDownloadCase {
	return []retryDownloadCase{
		{
			Name:                  "always-fails",
			Image:                 exampleRetryImage("alpine-dawn"),
			FailuresBeforeSuccess: -1,
		},
		{
			Name:                  "succeeds-on-second",
			Image:                 exampleRetryImage("city-lights"),
			FailuresBeforeSuccess: 1,
		},
		{
			Name:                  "succeeds-on-third",
			Image:                 exampleRetryImage("forest-path"),
			FailuresBeforeSuccess: 2,
		},
	}
}

// shouldFailRetryDownload answers whether the current attempt still belongs to
// the scenario's planned failure window.
func shouldFailRetryDownload(item retryDownloadCase, attempt int) bool {
	if item.FailuresBeforeSuccess < 0 {
		return true
	}

	return attempt <= item.FailuresBeforeSuccess
}

// downloadCaseWithRetry wraps one case in flx.DoWithRetryCtx so the example
// demonstrates retry budget, interval, attempt timeout, and retry observer
// behavior in one place.
func downloadCaseWithRetry(
	ctx context.Context,
	client *http.Client,
	item retryDownloadCase,
) retryDownloadResult {
	var body []byte
	var attempts int

	// Keep the current attempt number outside the callback so the final result
	// can report how far the retry loop progressed even on failure.
	err := flx.DoWithRetryCtx(
		ctx,
		func(ctx context.Context, attempt int) error {
			attempts = attempt + 1

			data, err := downloadRetryExampleURL(ctx, client, item.Image.SourceURL)
			if err != nil {
				return fmt.Errorf("attempt %d: %w", attempts, err)
			}

			body = data
			return nil
		},
		flx.WithRetry(retryExampleMaxAttempts),
		flx.WithInterval(retryExampleRetryInterval),
		flx.WithAttemptTimeout(retryExampleAttemptTimeout),
		flx.WithOnRetry(func(event flx.RetryEvent) {
			// Log retry warnings as they happen so readers can correlate transient
			// failures with the eventual success/failure summary line for this case.
			logRetryExamplef(
				logKindDownloadRetryWarning,
				"case=%s image=%s retry=%d/%d err=%s",
				item.Name,
				item.Image.ID,
				event.Retry,
				event.MaxAttempts,
				imgproc.FormatItemError(event.Err),
			)
		}),
	)

	return retryDownloadResult{
		Name:     item.Name,
		ImageID:  item.Image.ID,
		URL:      item.Image.SourceURL,
		Attempts: attempts,
		Bytes:    len(body),
		Err:      err,
	}
}

// downloadRetryExampleURL performs one plain HTTP GET attempt. Retry and
// backoff are intentionally handled by the caller so this helper stays focused
// on one transport interaction.
func downloadRetryExampleURL(
	ctx context.Context,
	client *http.Client,
	imageURL string,
) ([]byte, error) {
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

func exampleRetryImage(imageID string) imgproc.RemoteImage {
	const (
		retryImageWidth  = 640
		retryImageHeight = 360
	)

	return imgproc.RemoteImage{
		ID:        imageID,
		SourceURL: buildRetryImageURL(imageID, retryImageWidth, retryImageHeight),
	}
}
