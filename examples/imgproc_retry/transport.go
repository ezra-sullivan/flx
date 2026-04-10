package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
)

type retryCaseTransport struct {
	mu       sync.Mutex
	attempts map[string]int
	cases    map[string]retryDownloadCase
}

// newRetryExampleClient keeps the retry example self-contained with a local
// deterministic source transport so each planned retry outcome stays
// reproducible.
func newRetryExampleClient(cases []retryDownloadCase) *http.Client {
	return &http.Client{
		Timeout: retryExampleAttemptTimeout,
		Transport: &retryCaseTransport{
			attempts: make(map[string]int, len(cases)),
			cases:    indexRetryCases(cases),
		},
	}
}

func indexRetryCases(cases []retryDownloadCase) map[string]retryDownloadCase {
	index := make(map[string]retryDownloadCase, len(cases))
	for _, item := range cases {
		index[item.Image.SourceURL] = item
	}

	return index
}

func (t *retryCaseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil || req == nil || req.URL == nil {
		return nil, fmt.Errorf("retry example transport: nil request")
	}

	imageURL := req.URL.String()
	item, ok := t.cases[imageURL]
	if !ok {
		// Requests outside the scripted cases still read from the same local retry
		// source so the transport stays usable for ad-hoc inputs.
		return serveRetryImage(req)
	}

	attempt := t.nextAttempt(imageURL)
	if shouldFailRetryDownload(item, attempt) {
		return statusResponse(
			req,
			http.StatusServiceUnavailable,
			fmt.Sprintf(
				"planned failure case=%s image=%s attempt=%d",
				item.Name,
				item.Image.ID,
				attempt,
			),
		), nil
	}

	return serveRetryImage(req)
}

func (t *retryCaseTransport) nextAttempt(imageURL string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.attempts[imageURL]++
	return t.attempts[imageURL]
}

func statusResponse(req *http.Request, statusCode int, body string) *http.Response {
	resp := &http.Response{
		StatusCode:    statusCode,
		Status:        fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader([]byte(body))),
		ContentLength: int64(len(body)),
		Request:       req,
	}
	resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
	return resp
}

const (
	retrySourceScheme = "https"
	retrySourceHost   = "imgproc-retry.local"
	retrySourcePrefix = "/v1/retry/images"
)

func buildRetryImageURL(imageID string, width int, height int) string {
	values := url.Values{}
	values.Set("w", strconv.Itoa(width))
	values.Set("h", strconv.Itoa(height))

	return (&url.URL{
		Scheme:   retrySourceScheme,
		Host:     retrySourceHost,
		Path:     path.Join(retrySourcePrefix, imageID+".jpg"),
		RawQuery: values.Encode(),
	}).String()
}

func serveRetryImage(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("retry source transport: nil request")
	}
	if req.URL.Host != retrySourceHost {
		return nil, fmt.Errorf("retry source transport: unsupported host %q", req.URL.Host)
	}

	imageID := retryImageIDFromPath(req.URL.Path)
	if imageID == "" {
		return statusResponse(req, http.StatusNotFound, "image not found"), nil
	}

	width := retryPositiveQueryInt(req.URL.Query(), "w", 640)
	height := retryPositiveQueryInt(req.URL.Query(), "h", 360)
	imageBytes, err := retryImageBytes(imageID, width, height)
	if err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(imageBytes)),
		ContentLength: int64(len(imageBytes)),
		Request:       req,
	}
	resp.Header.Set("Content-Type", "image/jpeg")
	return resp, nil
}

func retryImageIDFromPath(requestPath string) string {
	if !strings.HasPrefix(requestPath, retrySourcePrefix+"/") {
		return ""
	}

	filename := path.Base(requestPath)
	return strings.TrimSuffix(filename, path.Ext(filename))
}

func retryPositiveQueryInt(values url.Values, key string, fallback int) int {
	value, err := strconv.Atoi(values.Get(key))
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}

func retryImageBytes(imageID string, width int, height int) ([]byte, error) {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x*17 + len(imageID)*13) % 256),
				G: uint8((y*29 + len(imageID)*7) % 256),
				B: uint8((x*3 + y*5 + len(imageID)*11) % 256),
				A: 0xFF,
			})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
