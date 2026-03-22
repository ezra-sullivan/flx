package flx

import (
	"errors"
	"testing"
)

func assertPanicIs(t *testing.T, target error, fn func()) {
	t.Helper()

	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		fn()
	}()

	if recovered == nil {
		t.Fatal("expected panic")
	}

	err, ok := recovered.(error)
	if !ok {
		t.Fatalf("expected panic value to be error, got %T", recovered)
	}
	if !errors.Is(err, target) {
		t.Fatalf("expected panic to match %v, got %v", target, err)
	}
}
