package imgproc

import (
	"fmt"
	"log"
)

const (
	logKindDownloadRetryWarning    = "download_retry_warning"
	logKindImageProcessed          = "image_processed"
	logKindImageProcessingFailed   = "image_processing_failed"
	logKindImageProcessingComplete = "image_processing_complete"
	logKindImageUploaded           = "image_uploaded"
	logKindImageUploadFailed       = "image_upload_failed"
	logKindImageUploadComplete     = "image_upload_complete"
)

func formatKinded(kind string, format string, args ...any) string {
	params := make([]any, 0, len(args)+1)
	params = append(params, kind)
	params = append(params, args...)
	return fmt.Sprintf("kind=%s "+format, params...)
}

func logExamplef(kind string, format string, args ...any) {
	log.Print(formatKinded(kind, format, args...))
}
