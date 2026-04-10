package imgproc

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ezra-sullivan/flx"
)

// ConsumeProcessedImages drains the locally processed stream and logs one line
// per item plus one summary line.
func ConsumeProcessedImages(images flx.Stream[ImageResult[[]byte]]) error {
	var successCount int
	var failedCount int

	err := images.ForEachErr(func(image ImageResult[[]byte]) {
		if image.Err != nil {
			failedCount++
			logExamplef(
				logKindImageProcessingFailed,
				"id=%s source=%s err=%s",
				image.ImageID,
				image.SourceURL,
				FormatItemError(image.Err),
			)
			return
		}

		successCount++
		logExamplef(
			logKindImageProcessed,
			"id=%s source=%s bytes=%d",
			image.ImageID,
			image.SourceURL,
			len(image.Value),
		)
	})
	if err != nil {
		return err
	}

	logExamplef(
		logKindImageProcessingComplete,
		"success=%d failed=%d",
		successCount,
		failedCount,
	)
	return nil
}

// ConsumeUploadedImages drains the upload result stream and logs one line per
// item plus one summary line.
func ConsumeUploadedImages(results flx.Stream[ImageResult[string]]) error {
	var successCount int
	var failedCount int

	err := results.ForEachErr(func(image ImageResult[string]) {
		if image.Err != nil {
			failedCount++
			logExamplef(
				logKindImageUploadFailed,
				"id=%s source=%s err=%s",
				image.ImageID,
				image.SourceURL,
				FormatItemError(image.Err),
			)
			return
		}

		successCount++
		logExamplef(
			logKindImageUploaded,
			"id=%s source=%s target=%s",
			image.ImageID,
			image.SourceURL,
			image.Value,
		)
	})
	if err != nil {
		return err
	}

	logExamplef(
		logKindImageUploadComplete,
		"success=%d failed=%d",
		successCount,
		failedCount,
	)
	return nil
}

// FormatItemError flattens wrapped and joined errors into one compact log
// line.
func FormatItemError(err error) string {
	leafErrs := collectLeafErrors(err)
	if len(leafErrs) == 0 {
		return compactErrorText(err.Error())
	}

	counts := make(map[string]int, len(leafErrs))
	order := make([]string, 0, len(leafErrs))
	for _, leafErr := range leafErrs {
		message := compactErrorText(leafErr.Error())
		if message == "" {
			continue
		}
		if counts[message] == 0 {
			order = append(order, message)
		}
		counts[message]++
	}

	if len(order) == 0 {
		return compactErrorText(err.Error())
	}

	parts := make([]string, 0, len(order))
	for _, message := range order {
		count := counts[message]
		if count == 1 {
			parts = append(parts, message)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s x%d", message, count))
	}

	return strings.Join(parts, "; ")
}

func collectLeafErrors(err error) []error {
	if err == nil {
		return nil
	}

	type multiUnwrapper interface {
		Unwrap() []error
	}

	if unwrapper, ok := err.(multiUnwrapper); ok {
		children := unwrapper.Unwrap()
		if len(children) == 0 {
			return []error{err}
		}

		leafErrs := make([]error, 0, len(children))
		for _, child := range children {
			leafErrs = append(leafErrs, collectLeafErrors(child)...)
		}
		return leafErrs
	}

	if child := errors.Unwrap(err); child != nil {
		return collectLeafErrors(child)
	}

	return []error{err}
}

func compactErrorText(message string) string {
	return strings.Join(strings.Fields(message), " ")
}
