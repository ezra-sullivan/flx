package flx

import (
	"testing"
	"time"
)

func TestShortCircuitOperatorsDrainUpstreamProducer(t *testing.T) {
	tests := []struct {
		name string
		run  func(Stream[int])
	}{
		{
			name: "First",
			run: func(s Stream[int]) {
				got, ok := s.First()
				if !ok || got != 0 {
					t.Fatalf("expected first item 0, got %v (ok=%v)", got, ok)
				}
			},
		},
		{
			name: "AnyMatch",
			run: func(s Stream[int]) {
				if !s.AnyMatch(func(item int) bool { return item == 0 }) {
					t.Fatal("expected AnyMatch to short-circuit with true")
				}
			},
		},
		{
			name: "AllMatch",
			run: func(s Stream[int]) {
				if s.AllMatch(func(item int) bool { return item > 0 }) {
					t.Fatal("expected AllMatch to short-circuit with false")
				}
			},
		},
		{
			name: "NoneMatch",
			run: func(s Stream[int]) {
				if s.NoneMatch(func(item int) bool { return item == 0 }) {
					t.Fatal("expected NoneMatch to short-circuit with false")
				}
			},
		},
		{
			name: "FirstErr",
			run: func(s Stream[int]) {
				got, ok, err := s.FirstErr()
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				if !ok || got != 0 {
					t.Fatalf("expected first item 0, got %v (ok=%v)", got, ok)
				}
			},
		},
		{
			name: "AnyMatchErr",
			run: func(s Stream[int]) {
				got, err := s.AnyMatchErr(func(item int) bool { return item == 0 })
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				if !got {
					t.Fatal("expected AnyMatchErr to short-circuit with true")
				}
			},
		},
		{
			name: "AllMatchErr",
			run: func(s Stream[int]) {
				got, err := s.AllMatchErr(func(item int) bool { return item > 0 })
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				if got {
					t.Fatal("expected AllMatchErr to short-circuit with false")
				}
			},
		},
		{
			name: "NoneMatchErr",
			run: func(s Stream[int]) {
				got, err := s.NoneMatchErr(func(item int) bool { return item == 0 })
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				if got {
					t.Fatal("expected NoneMatchErr to short-circuit with false")
				}
			},
		},
		{
			name: "Head",
			run: func(s Stream[int]) {
				if count := s.Head(1).Count(); count != 1 {
					t.Fatalf("expected Head(1) to produce 1 item, got %d", count)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, producerDone := testProducerStream(32)
			tt.run(stream)
			waitProducerDone(t, producerDone, tt.name)
		})
	}
}

func TestForAllPartialConsumerStillDrainsUpstreamProducer(t *testing.T) {
	stream, producerDone := testProducerStream(32)

	var consumed []int
	stream.ForAll(func(source <-chan int) {
		for range 3 {
			item, ok := <-source
			if !ok {
				t.Fatal("expected source to still have items during partial consumption")
			}
			consumed = append(consumed, item)
		}
	})

	if len(consumed) != 3 {
		t.Fatalf("expected 3 consumed items, got %d", len(consumed))
	}
	waitProducerDone(t, producerDone, "ForAll")
}

func TestReduceEarlyReturnStillDrainsUpstreamProducer(t *testing.T) {
	stream, producerDone := testProducerStream(32)

	got, err := Reduce(stream, func(source <-chan int) (int, error) {
		item, ok := <-source
		if !ok {
			t.Fatal("expected Reduce source to have an item")
		}

		return item, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != 0 {
		t.Fatalf("expected first item 0, got %d", got)
	}

	waitProducerDone(t, producerDone, "Reduce")
}

func testProducerStream(total int) (Stream[int], <-chan struct{}) {
	done := make(chan struct{})
	stream := From(func(source chan<- int) {
		defer close(done)
		for i := range total {
			source <- i
		}
	})
	return stream, done
}

func waitProducerDone(t *testing.T, done <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("%s should drain upstream and let the producer exit", name)
	}
}
