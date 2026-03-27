package flx

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
)

func TestShortCircuitTerminalsWaitForLateFailFastError(t *testing.T) {
	errBoom := errors.New("boom")

	tests := []struct {
		name string
		run  func(Stream[int])
	}{
		{
			name: "First",
			run: func(s Stream[int]) {
				_, _ = s.First()
			},
		},
		{
			name: "AllMatch",
			run: func(s Stream[int]) {
				_ = s.AllMatch(func(item int) bool { return item > 1 })
			},
		},
		{
			name: "AnyMatch",
			run: func(s Stream[int]) {
				_ = s.AnyMatch(func(item int) bool { return item == 1 })
			},
		},
		{
			name: "NoneMatch",
			run: func(s Stream[int]) {
				_ = s.NoneMatch(func(item int) bool { return item == 1 })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, release := newLateFailFastStream(errBoom)
			panicCh := make(chan any, 1)
			done := make(chan struct{})

			go func() {
				defer close(done)
				defer func() {
					panicCh <- recover()
				}()

				tt.run(stream)
			}()

			select {
			case <-done:
				t.Fatalf("%s returned before the delayed fail-fast error was released", tt.name)
			case <-time.After(50 * time.Millisecond):
			}

			close(release)

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatalf("%s did not finish after releasing the delayed error", tt.name)
			}

			panicValue := <-panicCh
			if panicValue == nil {
				t.Fatalf("%s should panic on the delayed fail-fast error", tt.name)
			}

			err, ok := panicValue.(error)
			if !ok {
				t.Fatalf("%s panic should be an error, got %T", tt.name, panicValue)
			}
			if !errors.Is(err, errBoom) {
				t.Fatalf("%s panic should wrap boom, got %v", tt.name, err)
			}
		})
	}
}

func TestHeadCollectErrWaitsForLateFailFastError(t *testing.T) {
	errBoom := errors.New("boom")
	stream, release := newLateFailFastStream(errBoom)

	type result struct {
		items []int
		err   error
	}

	resultCh := make(chan result, 1)
	go func() {
		items, err := stream.Head(1).CollectErr()
		resultCh <- result{items: items, err: err}
	}()

	select {
	case <-resultCh:
		t.Fatal("Head(1).CollectErr returned before the delayed fail-fast error was released")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	select {
	case got := <-resultCh:
		if !slices.Equal(got.items, []int{1}) {
			t.Fatalf("unexpected Head(1) items: %v", got.items)
		}
		if !errors.Is(got.err, errBoom) {
			t.Fatalf("expected Head(1).CollectErr to include boom, got %v", got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("Head(1).CollectErr did not finish after releasing the delayed error")
	}
}

func TestShortCircuitErrTerminalsReturnBeforeLateFailFastErrorSettles(t *testing.T) {
	errBoom := errors.New("boom")

	tests := []struct {
		name string
		run  func(Stream[int]) error
	}{
		{
			name: "FirstErr",
			run: func(s Stream[int]) error {
				got, ok, err := s.FirstErr()
				if !ok || got != 1 {
					return fmt.Errorf("expected FirstErr to return item 1, got %v (ok=%v)", got, ok)
				}
				return err
			},
		},
		{
			name: "AllMatchErr",
			run: func(s Stream[int]) error {
				got, err := s.AllMatchErr(func(item int) bool { return item > 1 })
				if got {
					return errors.New("expected AllMatchErr to short-circuit with false")
				}
				return err
			},
		},
		{
			name: "AnyMatchErr",
			run: func(s Stream[int]) error {
				got, err := s.AnyMatchErr(func(item int) bool { return item == 1 })
				if !got {
					return errors.New("expected AnyMatchErr to short-circuit with true")
				}
				return err
			},
		},
		{
			name: "NoneMatchErr",
			run: func(s Stream[int]) error {
				got, err := s.NoneMatchErr(func(item int) bool { return item == 1 })
				if got {
					return errors.New("expected NoneMatchErr to short-circuit with false")
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, release := newLateFailFastStream(errBoom)
			resultCh := make(chan error, 1)

			go func() {
				resultCh <- tt.run(stream)
			}()

			select {
			case err := <-resultCh:
				if err != nil {
					t.Fatalf("%s should return before the delayed fail-fast error settles, got %v", tt.name, err)
				}
			case <-time.After(50 * time.Millisecond):
				t.Fatalf("%s did not short-circuit before the delayed fail-fast error was released", tt.name)
			}

			if err := stream.Err(); err != nil {
				t.Fatalf("%s should not see the delayed fail-fast error before release, got %v", tt.name, err)
			}

			close(release)
			waitStreamErr(t, stream, errBoom, tt.name)
		})
	}
}

func TestRingWrapKeepsIndexBoundedAndOrderStable(t *testing.T) {
	ring := newRing[int](3)
	for i := range 10 {
		ring.Add(i)
	}

	if ring.index < 0 || ring.index >= len(ring.elements) {
		t.Fatalf("expected ring index to stay within bounds, index=%d size=%d", ring.index, len(ring.elements))
	}

	got := ring.Take()
	want := []int{7, 8, 9}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected ring order: got %v want %v", got, want)
	}
}

func newLateFailFastStream(err error) (Stream[int], chan struct{}) {
	release := make(chan struct{})
	firstDone := make(chan struct{})

	stream := MapErr(Values(1, 2), func(item int) (int, error) {
		if item == 1 {
			close(firstDone)
			return item, nil
		}

		<-firstDone
		<-release
		return 0, err
	}, WithWorkers(2))

	return stream, release
}

func waitStreamErr(t *testing.T, stream Stream[int], want error, name string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if err := stream.Err(); errors.Is(err, want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("%s should eventually record %v in stream state", name, want)
}
