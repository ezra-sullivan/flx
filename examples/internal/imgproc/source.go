package imgproc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/ezra-sullivan/flx"
)

// ListExampleImages is the source stage shared by the image-processing examples.
// It pages the Picsum list endpoint lazily and emits work items as soon as they
// are discovered.
func ListExampleImages(
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

			photos, err := fetchImageCatalogPage(ctx, sourceHTTPClient, cfg.ListEndpoint, page, pageSize)
			if err != nil {
				panic(fmt.Errorf("list images page=%d: %w", page, err))
			}

			if len(photos) == 0 {
				return
			}

			for _, photo := range photos {
				img := RemoteImage{
					ID:        photo.ID,
					SourceURL: BuildImageDownloadURL(photo.ID, downloadWidth, downloadHeight),
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

// FetchImageCatalogPage requests one page of image metadata from the Picsum
// list endpoint.
func FetchImageCatalogPage(
	ctx context.Context,
	client *http.Client,
	listEndpoint string,
	page int,
	limit int,
) ([]ImageCatalogEntry, error) {
	return fetchImageCatalogPage(ctx, client, listEndpoint, page, limit)
}

// BuildImageDownloadURL derives one concrete image download URL from the image
// ID and target dimensions.
func BuildImageDownloadURL(photoID string, width int, height int) string {
	return fmt.Sprintf("https://picsum.photos/id/%s/%d/%d.jpg", photoID, width, height)
}

func fetchImageCatalogPage(
	ctx context.Context,
	client *http.Client,
	listEndpoint string,
	page int,
	limit int,
) ([]ImageCatalogEntry, error) {
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
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var photos []ImageCatalogEntry
	if err := json.NewDecoder(resp.Body).Decode(&photos); err != nil {
		return nil, err
	}

	return photos, nil
}
