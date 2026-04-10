package main

import (
	"fmt"
	"log"
)

const (
	// Keep the retry example on the same plain-text `kind=` scheme as the rest
	// of the local imgproc example family so readers only learn one logging
	// convention.
	logKindDownloadRetryWarning   = "download_retry_warning"
	logKindDownloadRetrySucceeded = "download_retry_succeeded"
	logKindDownloadRetryFailed    = "download_retry_failed"
	logKindDownloadRetryComplete  = "download_retry_complete"
)

// formatRetryKinded centralizes the log prefix contract for this example.
func formatRetryKinded(kind string, format string, args ...any) string {
	params := make([]any, 0, len(args)+1)
	params = append(params, kind)
	params = append(params, args...)
	return fmt.Sprintf("kind=%s "+format, params...)
}

// logRetryExamplef is the single sink used by the retry demo's ordinary log
// lines.
func logRetryExamplef(kind string, format string, args ...any) {
	log.Print(formatRetryKinded(kind, format, args...))
}
