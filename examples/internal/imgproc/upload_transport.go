package imgproc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

type localUploadTransport struct{}

// newLocalUploadTransport keeps the optional upload stage self-contained while
// the source-side examples continue to use real outbound downloads.
func newLocalUploadTransport() http.RoundTripper {
	return localUploadTransport{}
}

func (localUploadTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("local upload transport: nil request")
	}
	if req.URL.Host != exampleTargetHost {
		return nil, fmt.Errorf("local upload transport: unsupported host %q", req.URL.Host)
	}
	if req.Method != http.MethodPost || req.URL.Path != exampleUploadPath {
		return textResponse(req, http.StatusNotFound, "not found"), nil
	}

	if _, err := io.ReadAll(req.Body); err != nil {
		return nil, fmt.Errorf("read upload body: %w", err)
	}

	imageID := imageIDFromPicsumSourceURL(req.Header.Get("X-Source-URL"))
	if imageID == "" {
		imageID = "uploaded-" + strconv.Itoa(len(req.Header.Get("X-Source-URL")))
	}

	return jsonResponse(req, http.StatusOK, struct {
		URL string `json:"url"`
	}{
		URL: buildStoredImageURL(imageID),
	})
}

func jsonResponse(req *http.Request, statusCode int, payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return bytesResponse(req, statusCode, "application/json", body), nil
}

func bytesResponse(
	req *http.Request,
	statusCode int,
	contentType string,
	body []byte,
) *http.Response {
	resp := &http.Response{
		StatusCode:    statusCode,
		Status:        fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
	if contentType != "" {
		resp.Header.Set("Content-Type", contentType)
	}

	return resp
}

func textResponse(req *http.Request, statusCode int, body string) *http.Response {
	return bytesResponse(req, statusCode, "text/plain; charset=utf-8", []byte(body))
}
