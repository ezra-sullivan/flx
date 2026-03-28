package flx

import (
	"context"
	"errors"
	"testing"
)

// These tests cover context-aware transform behavior around canceled sends.
func TestSendContextDoesNotSendWhenContextAlreadyCanceled(t *testing.T) {
	errStopped := errors.New("stopped")

	for range 512 {
		ctx, cancel := context.WithCancelCause(t.Context())
		cancel(errStopped)

		pipe := make(chan int, 1)
		if SendContext(ctx, pipe, 1) {
			t.Fatal("SendContext reported success after context cancellation")
		}

		select {
		case got := <-pipe:
			t.Fatalf("SendContext delivered %d after context cancellation", got)
		default:
		}
	}
}

func TestMapContextCanceledSendDoesNotEmitValue(t *testing.T) {
	errStopped := errors.New("stopped")

	for range 128 {
		ctx, cancel := context.WithCancelCause(t.Context())
		started := make(chan struct{})
		release := make(chan struct{})

		out := MapContext(ctx, Values(1), func(ctx context.Context, item int) int {
			close(started)
			<-release
			return item
		}, WithWorkers(1))

		<-started
		cancel(errStopped)
		close(release)

		items, err := out.CollectErr()
		if err != nil {
			t.Fatalf("expected nil stream error, got %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("expected canceled send to emit no items, got %v", items)
		}
	}
}
