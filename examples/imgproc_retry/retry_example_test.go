package main

import "testing"

func TestDownloadCaseWithRetrySucceedsOnThirdAttempt(t *testing.T) {
	ctx := t.Context()
	item := retryDownloadCase{
		Name:                  "succeeds-on-third",
		Image:                 exampleRetryImage("forest-path"),
		FailuresBeforeSuccess: 2,
	}
	client := newRetryExampleClient([]retryDownloadCase{item})

	result := downloadCaseWithRetry(ctx, client, item)
	if result.Err != nil {
		t.Fatalf("downloadCaseWithRetry returned error: %v", result.Err)
	}

	if result.Attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", result.Attempts)
	}

	if result.ImageID != "forest-path" {
		t.Fatalf("unexpected image id: %q", result.ImageID)
	}

	if result.Bytes <= 0 {
		t.Fatalf("expected successful download bytes, got %d", result.Bytes)
	}
}

func TestDownloadCaseWithRetryFailsAfterThreeAttempts(t *testing.T) {
	ctx := t.Context()
	item := retryDownloadCase{
		Name:                  "always-fails",
		Image:                 exampleRetryImage("alpine-dawn"),
		FailuresBeforeSuccess: -1,
	}
	client := newRetryExampleClient([]retryDownloadCase{item})

	result := downloadCaseWithRetry(ctx, client, item)
	if result.Err == nil {
		t.Fatal("expected downloadCaseWithRetry to fail")
	}

	if result.Attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", result.Attempts)
	}

	if result.Bytes != 0 {
		t.Fatalf("expected no successful download bytes, got %d", result.Bytes)
	}
}
