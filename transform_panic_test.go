package flx

import (
	"context"
	"strings"
	"testing"
	"time"
)

// These tests verify that asynchronous transform panics are surfaced and still drain upstream producers.
func TestAsyncTransformPanicsAreReturnedByErrTerminals(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "DistinctBy",
			run: func() error {
				return DistinctBy(Values(1, 2, 3), func(v int) int {
					panic("distinct boom")
				}).DoneErr()
			},
		},
		{
			name: "DistinctByCount",
			run: func() error {
				return DistinctByCount(Values(1, 2, 3), 2, func(v int) int {
					panic("distinct-count boom")
				}).DoneErr()
			},
		},
		{
			name: "GroupByCount",
			run: func() error {
				return GroupByCount(Values(1, 2, 3), 2, func(v int) int {
					panic("group-count boom")
				}).DoneErr()
			},
		},
		{
			name: "DistinctByWindow",
			run: func() error {
				return DistinctByWindow(context.Background(), Values(1, 2, 3), time.Second, func(v int) int {
					panic("distinct-window boom")
				}).DoneErr()
			},
		},
		{
			name: "GroupByWindow",
			run: func() error {
				return GroupByWindow(context.Background(), Values(1, 2, 3), time.Second, func(v int) int {
					panic("group-window boom")
				}).DoneErr()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatal("expected panic to be returned as error")
			}
			if !strings.Contains(err.Error(), "boom") {
				t.Fatalf("expected error to mention panic, got %v", err)
			}
		})
	}
}

func TestAsyncTransformPanicsTriggerNonErrTerminalPanic(t *testing.T) {
	tests := []struct {
		name string
		run  func()
	}{
		{
			name: "DistinctBy",
			run: func() {
				DistinctBy(Values(1, 2, 3), func(v int) int {
					panic("distinct boom")
				}).Done()
			},
		},
		{
			name: "DistinctByWindow",
			run: func() {
				DistinctByWindow(context.Background(), Values(1, 2, 3), time.Second, func(v int) int {
					panic("distinct-window boom")
				}).Done()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var recovered any

			func() {
				defer func() {
					recovered = recover()
				}()
				tt.run()
			}()

			if recovered == nil {
				t.Fatal("expected Done to panic")
			}

			err, ok := recovered.(error)
			if !ok {
				t.Fatalf("expected panic value to be error, got %T", recovered)
			}
			if !strings.Contains(err.Error(), "boom") {
				t.Fatalf("expected panic to mention original panic, got %v", err)
			}
		})
	}
}

func TestAsyncTransformPanicStillDrainsUpstreamProducer(t *testing.T) {
	tests := []struct {
		name string
		run  func(Stream[int]) error
	}{
		{
			name: "DistinctBy",
			run: func(s Stream[int]) error {
				return DistinctBy(s, func(v int) int {
					panic("distinct boom")
				}).DoneErr()
			},
		},
		{
			name: "DistinctByCount",
			run: func(s Stream[int]) error {
				return DistinctByCount(s, 2, func(v int) int {
					panic("distinct-count boom")
				}).DoneErr()
			},
		},
		{
			name: "GroupByCount",
			run: func(s Stream[int]) error {
				return GroupByCount(s, 2, func(v int) int {
					panic("group-count boom")
				}).DoneErr()
			},
		},
		{
			name: "DistinctByWindow",
			run: func(s Stream[int]) error {
				return DistinctByWindow(context.Background(), s, time.Second, func(v int) int {
					panic("distinct-window boom")
				}).DoneErr()
			},
		},
		{
			name: "GroupByWindow",
			run: func(s Stream[int]) error {
				return GroupByWindow(context.Background(), s, time.Second, func(v int) int {
					panic("group-window boom")
				}).DoneErr()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, producerDone := testProducerStream(32)

			err := tt.run(stream)
			if err == nil {
				t.Fatal("expected panic to be returned as error")
			}

			waitProducerDone(t, producerDone, tt.name)
		})
	}
}
