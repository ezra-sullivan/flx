package imgproc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ezra-sullivan/flx"
)

func TestDownloadImageWithRetryRetriesUntilSuccess(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := requestCount.Add(1)
		if attempt == 1 {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}

		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	cfg := PipelineConfig{
		DownloadRetryTimes:     3,
		DownloadRetryInterval:  time.Millisecond,
		DownloadAttemptTimeout: time.Second,
	}

	data, err := DownloadImageWithRetry(
		context.Background(),
		server.Client(),
		RemoteImage{
			ID:        "demo",
			SourceURL: server.URL,
		},
		cfg,
	)
	if err != nil {
		t.Fatalf("DownloadImageWithRetry returned error: %v", err)
	}

	if got := string(data); got != "ok" {
		t.Fatalf("unexpected body: %q", got)
	}

	if got := requestCount.Load(); got != 2 {
		t.Fatalf("expected two HTTP requests after one retry, got %d", got)
	}
}

func TestFetchImageCatalogPageUsesLocalTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("page"); got != "1" {
			t.Fatalf("unexpected page query: %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "3" {
			t.Fatalf("unexpected limit query: %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"id":"10"},{"id":"11"},{"id":"12"}]`)
	}))
	defer server.Close()

	items, err := FetchImageCatalogPage(t.Context(), server.Client(), server.URL, 1, 3)
	if err != nil {
		t.Fatalf("FetchImageCatalogPage returned error: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected three catalog entries, got %d", len(items))
	}
	if items[0].ID != "10" || items[2].ID != "12" {
		t.Fatalf("unexpected catalog page: %#v", items)
	}
}

func TestDownloadImageWithRetryFailsAfterRetryBudget(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		http.Error(w, "still failing", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := PipelineConfig{
		DownloadRetryTimes:     3,
		DownloadRetryInterval:  time.Millisecond,
		DownloadAttemptTimeout: time.Second,
	}

	_, err := DownloadImageWithRetry(
		context.Background(),
		server.Client(),
		RemoteImage{
			ID:        "demo",
			SourceURL: server.URL,
		},
		cfg,
	)
	if err == nil {
		t.Fatal("expected DownloadImageWithRetry to fail after retry budget")
	}

	if !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := requestCount.Load(); got != 3 {
		t.Fatalf("expected three HTTP requests for a three-attempt budget, got %d", got)
	}
}

func TestPostImageUsesLocalTargetTransport(t *testing.T) {
	got, err := PostImage(
		t.Context(),
		NewTargetHTTPClient(),
		ExampleUploadEndpoint(),
		BuildImageDownloadURL("forest-path", 640, 360),
		[]byte("processed"),
	)
	if err != nil {
		t.Fatalf("PostImage returned error: %v", err)
	}

	want := "https://image-target.local/v1/stored/images/forest-path.jpg"
	if got != want {
		t.Fatalf("unexpected uploaded url:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestConsumeProcessedImagesLogsKinds(t *testing.T) {
	output := captureExampleLogs(t, func() {
		err := ConsumeProcessedImages(flx.Values(
			ImageResult[[]byte]{
				ImageID:   "ok",
				SourceURL: "https://example.com/ok.jpg",
				Value:     []byte("ok"),
			},
			ImageResult[[]byte]{
				ImageID:   "bad",
				SourceURL: "https://example.com/bad.jpg",
				Err:       context.DeadlineExceeded,
			},
		))
		if err != nil {
			t.Fatalf("ConsumeProcessedImages returned error: %v", err)
		}
	})

	for _, want := range []string{
		"kind=image_processed id=ok source=https://example.com/ok.jpg bytes=2",
		"kind=image_processing_failed id=bad source=https://example.com/bad.jpg err=context deadline exceeded",
		"kind=image_processing_complete success=1 failed=1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing log line %q in output:\n%s", want, output)
		}
	}
}

func TestConsumeUploadedImagesLogsKinds(t *testing.T) {
	output := captureExampleLogs(t, func() {
		err := ConsumeUploadedImages(flx.Values(
			ImageResult[string]{
				ImageID:   "ok",
				SourceURL: "https://example.com/ok.jpg",
				Value:     "https://target.example.com/ok.jpg",
			},
			ImageResult[string]{
				ImageID:   "bad",
				SourceURL: "https://example.com/bad.jpg",
				Err:       context.Canceled,
			},
		))
		if err != nil {
			t.Fatalf("ConsumeUploadedImages returned error: %v", err)
		}
	})

	for _, want := range []string{
		"kind=image_uploaded id=ok source=https://example.com/ok.jpg target=https://target.example.com/ok.jpg",
		"kind=image_upload_failed id=bad source=https://example.com/bad.jpg err=context canceled",
		"kind=image_upload_complete success=1 failed=1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing log line %q in output:\n%s", want, output)
		}
	}
}

func TestFormatDownloadRetryWarning(t *testing.T) {
	got := FormatDownloadRetryWarning("https://picsum.photos/id/42/1920/1080.jpg", flx.RetryEvent{
		Attempt:     2,
		MaxAttempts: 3,
		Retry:       2,
		MaxRetries:  2,
		NextAttempt: 3,
		Err:         errors.New("temporary network failure"),
	})
	want := "kind=download_retry_warning retry=2/3 url=https://picsum.photos/id/42/1920/1080.jpg err=temporary network failure"
	if got != want {
		t.Fatalf("unexpected retry warning format:\nwant: %s\ngot:  %s", want, got)
	}
}

func captureExampleLogs(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	originalPrefix := log.Prefix()

	log.SetOutput(&buf)
	log.SetFlags(0)
	log.SetPrefix("")
	defer func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
		log.SetPrefix(originalPrefix)
	}()

	fn()
	return buf.String()
}
