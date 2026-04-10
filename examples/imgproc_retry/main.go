package main

import (
	"context"
	"log"
)

// main runs a deterministic local retry demo that shows both successful
// recovery and retry-budget exhaustion without relying on flaky external
// services.
func main() {
	if err := runDownloadRetryExample(context.Background()); err != nil {
		log.Fatal(err)
	}
}
