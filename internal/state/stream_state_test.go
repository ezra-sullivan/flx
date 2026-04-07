package state

import (
	"errors"
	"testing"
)

func TestMergedStateReflectsParentsImmediately(t *testing.T) {
	left := NewStream()
	right := NewStream()
	merged := NewMerged(left, right)

	leftErr := errors.New("left failed")
	rightErr := errors.New("right failed")

	left.Add(leftErr, true)
	right.Add(rightErr, false)

	if !errors.Is(merged.Err(), leftErr) {
		t.Fatalf("expected merged error to include left failure, got %v", merged.Err())
	}
	if !errors.Is(merged.Err(), rightErr) {
		t.Fatalf("expected merged error to include right failure, got %v", merged.Err())
	}
	if !merged.ShouldPanic() {
		t.Fatal("expected merged state to inherit panic-on-terminal from parents")
	}
}
