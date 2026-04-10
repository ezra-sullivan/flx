package imgproc

import (
	"net/url"
	"path"
	"strings"
)

const (
	exampleTargetScheme = "https"
	exampleTargetHost   = "image-target.local"
	exampleUploadPath   = "/v1/upload/images"
	exampleStoredPrefix = "/v1/stored/images"

	exampleUploadEndpoint = exampleTargetScheme + "://" + exampleTargetHost + exampleUploadPath
)

// buildStoredImageURL gives the optional upload stage a stable local target URL
// to log when a processed image is "uploaded".
func buildStoredImageURL(imageID string) string {
	return (&url.URL{
		Scheme: exampleTargetScheme,
		Host:   exampleTargetHost,
		Path:   path.Join(exampleStoredPrefix, imageID+".jpg"),
	}).String()
}

func imageIDFromPicsumSourceURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if parsedURL.Host != "picsum.photos" {
		return ""
	}

	segments := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(segments) < 3 || segments[0] != "id" {
		return ""
	}

	return segments[1]
}
